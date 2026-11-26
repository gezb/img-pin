package types

import (
	"fmt"
	"strings"

	"github.com/robdavid/img-pin/pkgs/images"
	yu "github.com/robdavid/img-pin/pkgs/yaml"
	"go.yaml.in/yaml/v3"
)

// UpdateMethod describe methods for modifying a YAML file in place,
// one of [UpdateOverwrite], [UpdatePatch] or [UpdateSync]. The various
// methods are attempting, with varying degrees of success, to preserve
// blank line whitespace.
//
//go:generate stringer -type UpdateMethod -linecomment
type UpdateMethod int

const (
	// UpdateOverwrite indicates update output YAML with simple overwrite.
	// This will lose some blank line cosmetic whitespace
	UpdateOverwrite UpdateMethod = iota // overwrite
	// UpdatePatch uses a diff then patch technique to try to retain newline
	// whitespace. This is experimental.
	UpdatePatch // patch
	// UpdateSync attempts to synchronize output file with input file in terms
	// on blank lines. This does not always work.
	UpdateSync //sync
)

// ImageDetails holds image fields detected from chart and deployment values,
// including where they are located in the YAML AST.
type ImageDetails struct {
	Registry   yu.PathValue[string]
	Repository yu.PathValue[string]
	Tag        yu.PathValue[string]
}

func (d *ImageDetails) Parent() yu.Path {
	for _, pv := range []yu.PathValue[string]{d.Registry, d.Repository, d.Tag} {
		if pv.Path != nil {
			return pv.Path[:len(pv.Path)-1]
		}
	}
	panic("image details have no parent path!")
}

func (d *ImageDetails) Set(img *images.Image) {
	tag := img.Tag
	if img.Digest != "" {
		tag += "@" + img.Digest
	}
	if d.Registry.IsEmpty() {
		if d.Tag.IsEmpty() {
			d.Repository.Value = img.String()
		} else {
			d.Repository.Value = img.Registry + "/" + img.Repository
			d.Tag.Value = tag
		}
	} else {
		d.Registry.Value = img.Registry
		if d.Tag.IsEmpty() {
			d.Repository.Value = img.Repository + ":" + tag
		} else {
			d.Repository.Value = img.Repository
			d.Tag.Value = tag
		}
	}
}

func (d *ImageDetails) Where() string {
	for _, pv := range []*yu.PathValue[string]{&d.Registry, &d.Repository, &d.Tag} {
		if len(pv.Path) > 1 {
			parent := pv.Path[:len(pv.Path)-1]
			return fmt.Sprintf("at path %s", parent)
		}
	}
	return ""
}

func (d *ImageDetails) WriteInto(dep Deployment) error {
	for _, pathValue := range []yu.PathValue[string]{d.Registry, d.Repository, d.Tag} {
		if pathValue.Path != nil {
			if err := dep.WriteStringValue(pathValue.Value, pathValue.Path.Objects()...); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *ImageDetails) AddImageOverride(kustomization Kustomization) error {
	for _, pathValue := range []yu.PathValue[string]{d.Registry, d.Repository, d.Tag} {
		if pathValue.Path != nil {
			kustomization.AddImageOverride(d.Registry, d.Repository, d.Tag)
		}
	}
	return nil
}

func joinStr(prefix, delim, suffix string) string {
	if strings.HasSuffix(prefix, delim) || strings.HasPrefix(suffix, delim) {
		return prefix + suffix
	} else {
		return prefix + delim + suffix
	}
}

func (d *ImageDetails) String() string {
	if d.Tag.Value != "" {
		if d.Registry.Value != "" {
			return joinStr(joinStr(d.Registry.Value, "/", d.Repository.Value), ":", d.Tag.Value)
		}
		return joinStr(d.Repository.Value, ":", d.Tag.Value)
	} else {
		if d.Registry.Value != "" {
			return joinStr(d.Registry.Value, "/", d.Repository.Value)
		} else {
			return d.Repository.Value
		}
	}
}

type BaseResource interface {

	// Load populates the resource with raw YAML data provided in the doc parameter.
	Load(doc *yaml.Node, baseDirectory string) error

	// Save returns YAML data that represents the current state of the resource.
	Save() (*yaml.Node, error)

	// Performs any necessary cleanup actions after processing.
	Cleanup() error
}

// Resource is the basic common interface for all YAML resources.
type Resource interface {
	BaseResource

	// CanDigest returns true if the [Resource.Digest] and [Resource.Verify]
	// methods are supported.
	CanDigest() bool

	// Digest runs digests over the images in the resource
	Digest() error

	// Verify checks all digests are present and correct
	Verify() error

	// Expand returns a list of YAML nodes that represent all the YAML documents
	// the resource can be expanded into, if it can be so expanded. For example
	// a Helm will be expanded into YAML documents via the helm template
	// command. A resource that cannot be expanded simply returns itself as a
	// YAML node.
	Expand() ([]*yaml.Node, error)

	// CRDs returns any custom resource definitions associated with this resource, that
	// are not explicit in the resource itself. This typically applies to Helm charts.
	// Resources without such definitions will return an empty list (or nil).
	CRDs() ([]*yaml.Node, error)
}

// HelmOptions holds basic deployment options
type HelmOptions struct {
	ChartName     string
	InstanceName  string
	Repository    string
	Version       string
	Namespace     string
	BaseDirectory string
}

// Deployment represents a Helm deployment resource (such as a HelmChart).
type Deployment interface {
	BaseResource

	// Render generates the YAML resources that describes the deployment
	Render() ([]*yaml.Node, error)

	// DefaultValues returns the YAML resources that describe the Helm
	// chart's default values.
	DefaultValues() (*yaml.Node, error)

	// Values returns the parsed YAML nodes of the Helm values used in the
	// deployment.
	Values() (*yaml.Node, error)

	// WriteStringValue writes a string value into the deployment's values, at the
	// given path
	WriteStringValue(value string, path ...any) error

	// Options returns the options required for the deployment
	Options() *HelmOptions

	// CRDs returns crd documents as YAML nodes.
	CRDs() ([]*yaml.Node, error)
}

// KustomizationOptions holds base options
type KustomizationOptions struct {
	Namespace     string
	BaseDirectory string
}

// Kustomization represents a kustomization deployment resource
type Kustomization interface {
	BaseResource
	// Render generates the YAML resources that describes the deployment
	Render() ([]*yaml.Node, error)

	// AddImageOverride adds the given image as an override to the Kustomize file
	AddImageOverride(registry yu.PathValue[string], repository yu.PathValue[string], tag yu.PathValue[string])

	// Options returns the options required for the deployment
	Options() *KustomizationOptions
}
