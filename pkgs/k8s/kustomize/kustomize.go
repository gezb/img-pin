package kustomize

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/k8s"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"go.yaml.in/yaml/v3"
	"sigs.k8s.io/kustomize/api/krusty"
	ktypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	k8syaml "sigs.k8s.io/yaml"
)

var KUSTOMIZE_FILE yu.Signature = yu.Signature{
	Tests: []yu.SigPath{
		{Path: []any{"apiVersion"}, Re: regexp.MustCompile(`^kustomize\.config\.k8s\.io/v1beta1$`)},
		{Path: []any{"kind"}, Re: regexp.MustCompile(`^Kustomization$`)},
	},
}

// KustomizeDeployment is a [types.Kustomization] based on the Kustomization
type KustomizeDeployment struct {
	options types.KustomizationOptions
	baseDirectory string
	kustomize     ktypes.Kustomization
	resources     []types.Resource
}

// Load loads the Kustomization document and its containing directory.
func (k *KustomizeDeployment) Load(doc *yaml.Node, baseDirectory string) (err error) {
	defer Catch(&err)
	k.baseDirectory = baseDirectory
	Check(doc.Decode(&k.kustomize))
	k.options.Namespace = k.kustomize.Namespace

	return nil
}


// Render returns the resources, as YAML nodes, from the Kustomize file.
func (k *KustomizeDeployment) Render() (docs []*yaml.Node, err error) {
	defer Catch(&err)
	opts := krusty.MakeDefaultOptions()
	opts.PluginConfig.HelmConfig.Enabled = true
	opts.PluginConfig.HelmConfig.Command = k8s.HelmBinary.Value()
	kustomizer := krusty.MakeKustomizer(opts)
	resMap := Try(kustomizer.Run(filesys.MakeFsOnDisk(), k.baseDirectory))
	output := Try(resMap.AsYaml())
	return yu.StreamDocsIn(bytes.NewBuffer(output))
}

func (k *KustomizeDeployment) Save() (doc *yaml.Node, err error) {
	defer Catch(&err)
	rawBytes, err := k8syaml.Marshal(k.kustomize)
	if err != nil {
		return nil, err
	}
	rNode, err :=kyaml.Parse(string(rawBytes))
	Check(yu.WriteDocsOut(filepath.Join(k.baseDirectory, "kustomization.yaml"),  rNode.YNode()))
	return rNode.YNode(), nil
}


func (k *KustomizeDeployment) Cleanup() error { return nil }

func (k *KustomizeDeployment) CRDs() error { return nil }


func (k *KustomizeDeployment) AddImageOverride(registry yu.PathValue[string], repository yu.PathValue[string], tag yu.PathValue[string]) { 
	idx := strings.Index(tag.Value, "@")
	k.kustomize.Images = append(k.kustomize.Images, ktypes.Image{
		Name: fmt.Sprintf("%s/%s", registry.Value, repository.Value),
		NewName: fmt.Sprintf("%s/%s", registry.Value, repository.Value),
		NewTag: tag.Value[:idx],
		Digest: tag.Value[idx+1:], 
	})
	
}

func (k *KustomizeDeployment) Options() *types.KustomizationOptions {
	return &k.options
}



// type KustomizeHandler struct{}

// func (KustomizeHandler) Match(doc *yaml.Node, imageDigester digester.ImageDigester, options skipping.ImageOptions) types.Resource {
// 	if KUSTOMIZE_FILE.Match(doc) {
// 		kproc := digester.MakeKustomizationProcessor(&KustomizeDeployment{}, options, imageDigester)
// 		return &kproc
// 	}
// 	return nil
// }

// func init() {
// 	digester.RegisterHandlerFirst("Kustomize", KustomizeHandler{})
// }
