package kustomize

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/robdavid/img-pin/pkgs/digester"
	"github.com/robdavid/img-pin/pkgs/images"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func readDoc(t *testing.T, filename string) *yaml.Node {
	t.Helper()
	docs, err := yu.ReadDocs(filename)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	return docs[0]
}

func readKustomization(t *testing.T, dir string) *yaml.Node {
	t.Helper()
	return readDoc(t, filepath.Join(dir, "kustomization.yaml"))
}

type noSkipOptions []images.ImageOption

func (noSkipOptions) SkipOnPolicy(*images.Image) bool      { return false }
func (noSkipOptions) SkipV1Schema(*images.Image) bool      { return false }
func (noSkipOptions) SkipNoDigest(*images.Image) bool      { return false }
func (noSkipOptions) SkipNotFound(*images.Image) bool      { return false }
func (o noSkipOptions) ImageOptions() []images.ImageOption { return o }

func TestKustomizeDeploymentLoad(t *testing.T) {
	t.Run("basic kustomization", func(t *testing.T) {
		dir := filepath.Join("tests", "basic")
		doc := readKustomization(t, dir)
		deployment := &KustomizeDeployment{}
		require.NoError(t, deployment.Load(doc, dir))
		assert.Equal(t, dir, deployment.baseDirectory)
		assert.Empty(t, deployment.Options().Namespace)
		_ , err := deployment.Save()
		require.NoError(t, err)
	})

	t.Run("kustomization with helm charts", func(t *testing.T) {
		dir := filepath.Join("tests", "helm")
		doc := readKustomization(t, dir)
		deployment := &KustomizeDeployment{}
		require.NoError(t, deployment.Load(doc, dir))
		assert.Equal(t, dir, deployment.baseDirectory)
		require.NotNil(t, deployment.Options())
		assert.Empty(t, deployment.Options().Namespace)
	})
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func TestKustomizeDeploymentSaveValues(t *testing.T) {
	const header = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
`

	t.Run("imageOverrides are persisted", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "kustomization.yaml", header+`helmCharts:
  - name: demo
`)

		deployment := &KustomizeDeployment{}
		require.NoError(t, deployment.Load(readKustomization(t, dir), dir))
		deployment.AddImageOverride(
			yu.PathValue[string]{
				Path: yu.PathOf("image", "registry"), 
				Value: "testRegistry",
				},
			yu.PathValue[string]{
				Path: yu.PathOf("image", "repository"), 
				Value: "anImage",
				},
			yu.PathValue[string]{
				Path: yu.PathOf("image", "tag"), 
				Value: "newTag@sha2561234567890",
				})
		deployment.AddImageOverride(
			yu.PathValue[string]{
				Path: yu.PathOf("image", "registry"), 
				Value: "testRegistry",
				},
			yu.PathValue[string]{
				Path: yu.PathOf("image", "repository"), 
				Value: "anotherImage",
				},
			yu.PathValue[string]{
				Path: yu.PathOf("image", "tag"), 
				Value: "anotherTag@sha2560123456789",
				})
		_, err := deployment.Save()
		require.NoError(t, err)
		
		kustomization := readDoc(t, filepath.Join(dir, "kustomization.yaml"))
		assert.Equal(t, "testRegistry/anImage", yu.Get[string](kustomization, "images",0, "name").GetOr(""))
		assert.Equal(t, "testRegistry/anImage", yu.Get[string](kustomization, "images",0, "newName").GetOr(""))
		assert.Equal(t, "newTag", yu.Get[string](kustomization, "images",0, "newTag").GetOr(""))
		assert.Equal(t, "sha2561234567890", yu.Get[string](kustomization, "images",0, "digest").GetOr(""))

		assert.Equal(t, "testRegistry/anotherImage", yu.Get[string](kustomization, "images",1, "name").GetOr(""))
		assert.Equal(t, "testRegistry/anotherImage", yu.Get[string](kustomization, "images",1, "newName").GetOr(""))
		assert.Equal(t, "anotherTag", yu.Get[string](kustomization, "images",1, "newTag").GetOr(""))
		assert.Equal(t, "sha2560123456789", yu.Get[string](kustomization, "images",1, "digest").GetOr(""))
	})


}

func TestKustomizeDeploymentRender(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		numDocs   int
		image     string
		namespace string
	}{
		{
			name:      "basic kustomization",
			dir:       filepath.Join("tests", "basic"),
			numDocs:   2,
			image:     "nginx:1.27",
			namespace: "",
		},
		{
			name:      "overlay kustomization",
			dir:       filepath.Join("tests", "overlay"),
			numDocs:   2,
			image:     "nginx:1.28",
			namespace: "prod",
		},
		{
			name:      "kustomization with helm charts",
			dir:       filepath.Join("tests", "helm"),
			numDocs:   2,
			image:     "ghcr.io/example/demo:2.0.0",
			namespace: "helm-demo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := readKustomization(t, tc.dir)
			deployment := &KustomizeDeployment{}
			require.NoError(t, deployment.Load(doc, tc.dir))

			docs, err := deployment.Render()
			require.NoError(t, err)
			require.Len(t, docs, tc.numDocs)

			var workload *yaml.Node
			for _, rendered := range docs {
				if yu.Get[string](rendered, "kind").GetOr("") == "Deployment" {
					workload = rendered
					break
				}
			}
			require.NotNil(t, workload)
			assert.Equal(t, tc.image, yu.Get[string](workload, "spec", "template", "spec", "containers", 0, "image").GetOr(""))
			assert.Equal(t, tc.namespace, yu.Get[string](workload, "metadata", "namespace").GetOr(""))
		})
	}
}

func TestKustomizeDeploymentExpandLeavesNamespaceUnset(t *testing.T) {
	dir := filepath.Join("tests", "basic")
	kustomizeDeployment := &KustomizeDeployment{}
	require.NoError(t, kustomizeDeployment.Load(readKustomization(t, dir), dir))

	helmProc := digester.MakeKustomizationProcessor(kustomizeDeployment, noSkipOptions{}, digester.NonLockingImageDigester{})
	docs, err := helmProc.Expand()
	require.NoError(t, err)
	require.Len(t, docs, 2)
	for _, doc := range docs {
		assert.Empty(t, yu.Get[string](doc, "metadata", "namespace").GetOr(""))
	}
}

func TestKustomizeDeploymentRenderError(t *testing.T) {
	dir := filepath.Join("tests", "basic")
	deployment := &KustomizeDeployment{}
	require.NoError(t, deployment.Load(readKustomization(t, dir), filepath.Join("tests", "does-not-exist")))

	docs, err := deployment.Render()
	require.Error(t, err)
	assert.Empty(t, docs)
}

// func TestKustomizeHandler(t *testing.T) {
// 	t.Run("matches kustomization", func(t *testing.T) {
// 		dir := filepath.Join("tests", "basic")
// 		doc := readKustomization(t, dir)
// 		resource := KustomizeHandler{}.Match(doc, digester.NonLockingImageDigester{}, nil)
// 		require.NotNil(t, resource)
// 		_, ok := resource.(*digester.KustomizationProcessor)
// 		assert.True(t, ok)
// 		require.NoError(t, resource.Load(doc, dir))
// 	})

// 	t.Run("ignores other resources", func(t *testing.T) {
// 		doc := readDoc(t, filepath.Join("tests", "basic", "deployment.yaml"))
// 		resource := KustomizeHandler{}.Match(doc, digester.NonLockingImageDigester{}, nil)
// 		assert.Nil(t, resource)
// 	})
// }

// func TestDigest(t *testing.T) {
// 	type ass = *assert.Assertions
// 	type req = *require.Assertions
// 	type testFn = func(t *testing.T, assert ass, require req)
// 	runner := func(mode runhelpers.ScriptMode, testFn testFn) func(t *testing.T) {
// 		return func(t *testing.T) {
// 			defer test.ReportErr(t)
// 			images.MockDigest(t, imghelpers.CommonMockDigestFunc)
// 			runhelpers.Script(t, runhelpers.ScriptOpts{
// 				OutputFile: "tests/run-*.json",
// 				ArgCompare: runhelpers.ArgsCompare,
// 				Mode:       mode,
// 			})
// 			testFn(t, assert.New(t), require.New(t))
// 		}
// 	}

// 	run := func(testFn testFn) func(*testing.T) { return runner(runhelpers.ScriptModeAuto, testFn) }
// 	// runEmpty := func(testFn testFn) func(*testing.T) { return runner(runhelpers.ScriptModeEmpty, testFn) }

// 	t.Run("loading full values", run(func(t *testing.T, assert ass, require req) {
// 		dir := filepath.Join("tests", "harbor")
// 		docs := Try(yu.ReadDocs(filepath.Join(dir, "kustomization.yaml")))
// 		require.Equal(1, len(docs))
// 		kustomizeDeployment := KustomizeDeployment{}
// 		kustomizeDeployment.Load(docs[0], dir)
// 		kProc := digester.MakeKustomizationProcessor(&kustomizeDeployment, noSkipOptions{}, digester.NonLockingImageDigester{})
// 		Check(kProc.LoadImages())
// 		require.Equal(10, len(kProc.Images))
// 		image := kProc.Images[0].Repository.Value + ":" + kProc.Images[0].Tag.Value
// 		fmt.Println(image, kProc.Images[0].Repository.Path)
// 		digested, _, _ := Try3(images.Digest(image, images.IncludeTag, images.SkipTime))
// 		fmt.Println(digested)
// 	}))

	// t.Run("loading chart values", runEmpty(func(t *testing.T, assert ass, require req) {
	// 	dir := filepath.Join("tests", "harbor")
	// 	docs := Try(yu.ReadDocs(filepath.Join(dir, "kustomization.yaml")))
	// 	require.Equal(1, len(docs))
	// 	deployment := KustomizeDeployment{}
	// 	deployment.Load(docs[0], dir)
	// 	value := yu.Get[string](Try(deployment.Values()), "expose", "ingress", "hosts", "core")
	// 	assert.Equal(opt.Value("harbor.domain"), value)
	// }))

	// t.Run("loading overrides", run(func(t *testing.T, assert ass, require req) {
	// 	dir := filepath.Join("tests", "harbor")
	// 	docs := Try(yu.ReadDocs(filepath.Join(dir, "kustomization.yaml")))
	// 	require.Equal(1, len(docs))
	// 	deployment := KustomizeDeployment{}
	// 	deployment.Load(docs[0], dir)
	// 	helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
	// 	Check(helmProc.LoadDefaultImages())
	// 	Check(helmProc.ResolveOverrides())
	// 	assert.Equal(10, len(helmProc.DeploymentImages))
	// 	img := Try(images.Parse(helmProc.DeploymentImages[0].String()))
	// 	assert.Equal("v2.11.2", img.Tag)
	// }))

	// t.Run("test helm update", run(func(t *testing.T, assert ass, require req) {
	// 	dir := filepath.Join("tests", "harbor")
	// 	docs := Try(yu.ReadDocs(filepath.Join(dir, "kustomization.yaml")))
	// 	require.Equal(1, len(docs))
	// 	deployment := KustomizeDeployment{}
	// 	deployment.Load(docs[0], dir)
	// 	helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
	// 	Check(helmProc.Digest())
	// 	deployment.Save()
	// }))

	// t.Run("test helm value set", run(func(t *testing.T, assert ass, require req) {
	// 	dir := filepath.Join("tests", "nginx-ingress")
	// 	docs := Try(yu.ReadDocs("tests/nginx-ingress/kustomization.yaml"))
	// 	require.Equal(1, len(docs))
	// 	deployment := KustomizeDeployment{}
	// 	deployment.Load(docs[0], dir)
	// 	helmProc := digester.MakeHelmProcessor(&deployment, noSkipOptions{}, digester.NonLockingImageDigester{})
	// 	Check(helmProc.Digest())
	// 	deployment.Save()
	// 	resources := Try(deployment.Render())
	// 	assert.Equal(10, len(resources))
	// 	for _, resource := range resources {
	// 		if kind, ok := yu.Get[string](resource, "kind").GetOK(); ok && kind == "Service" {
	// 			type obj = map[string]any
	// 			type lst = []any
	// 			var service obj
	// 			Check(resource.Decode(&service))
	// 			port0 := service["spec"].(obj)["ports"].(lst)[0].(obj)["port"]
	// 			port1 := service["spec"].(obj)["ports"].(lst)[1].(obj)["port"]
	// 			assert.Equal(8080, port0)
	// 			assert.Equal(8443, port1)
	// 			break
	// 		}
	// 	}
	// }))

// }
