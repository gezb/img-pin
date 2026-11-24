package digester

import (
	"fmt"
	"log/slog"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/slices"
	"github.com/robdavid/img-pin/pkgs/digester/skipping"
	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/ferrors"
	"github.com/robdavid/img-pin/pkgs/images"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"go.yaml.in/yaml/v3"
)


type KustomizationProcessor struct {
	Kustomization types.Kustomization
	// Images holds the default image values found in the
	// helm chart's values file
	Images []types.ImageDetails
	// DigestedImages are the overridden images with digests applied
	DigestedImages []types.ImageDetails
	options        skipping.ImageOptions
	digester       ImageDigester
}

func MakeKustomizationProcessor(deployment types.Kustomization, options skipping.ImageOptions, digester ImageDigester) KustomizationProcessor {
	return KustomizationProcessor{Kustomization: deployment, options: options, digester: digester}
}

func (*KustomizationProcessor) CanDigest() bool {
	return true
}


func (kp *KustomizationProcessor) Digest() (err error) {
	options := kp.options.ImageOptions()
	slog.Debug("digesting images")
	kp.DigestedImages = make([]types.ImageDetails, len(kp.Images))
	for i := range kp.Images {
		var img *images.Image
		deployImg := &kp.Images[i]
		if img, err = images.Parse(kp.Images[i].String()); err != nil {
			return fmt.Errorf("%w: %s %s", err, deployImg, deployImg.Where())
		}
		imgOpts := slices.Affix(options, images.SkipTime, images.IncludeTag)
		if _, err = kp.digester.GetDigest(img, imgOpts...); err != nil {
			err = skipping.SkipError(kp.options, err, img)
			if err == nil {
				kp.DigestedImages[i] = *deployImg
				continue
			}
			return fmt.Errorf("%w: %s %s", err, deployImg, deployImg.Where())
		}
		slog.Debug("patching {{.before}} => {{.after}}", "before", deployImg, "after", img)
		kp.DigestedImages[i] = *deployImg
		kp.DigestedImages[i].Set(img)
	}
	for _, id := range kp.DigestedImages {
		if err := id.AddImageOverride(kp.Kustomization); err != nil {
			return err
		}
	}
	return nil
}

func (kp *KustomizationProcessor) Verify() (err error) {
	defer Catch(&err)
	yd := MakeYamlDigester(kp.digester)
	docs := Try(kp.Kustomization.Render())
	for docn, doc := range docs {
		targets := yu.MatchImagePaths(doc)
		if verErr := yd.VerifyTargets(targets, kp.options); verErr != nil {
			err = ferrors.Join(err, fmt.Errorf("[doc %d] %w", docn, verErr))
		}
	}
	return
}

func (kp *KustomizationProcessor) Load(doc *yaml.Node, baseDirectory string) error {
	return kp.Kustomization.Load(doc, baseDirectory)
}

func (kp *KustomizationProcessor) Save() (*yaml.Node, error) {
	return kp.Kustomization.Save()
}

func (kp *KustomizationProcessor) Cleanup() error {
	return kp.Kustomization.Cleanup()
}

func (kp *KustomizationProcessor) Expand() (docs []*yaml.Node, err error) {
	defer Catch(&err)
	docs = Try(kp.Kustomization.Render())
	namespace := kp.Kustomization.Options().Namespace
	for _, doc := range docs {
		Check(SetResourceNamespace(doc, namespace))
	}
	return
}

func (kp *KustomizationProcessor) CRDs() (crds []*yaml.Node, err error) {
	return 
}