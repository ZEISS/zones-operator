package provisioners

import (
	"context"
	"errors"
)

// Request ...
type Request struct {
	// ReleaseName is the name of the release to install.
	ReleaseName string
	// Namespace is the namespace to install the release in.
	Namespace string
	// ChartVersion is the version of the chart to install.
	ChartVersion string
	// KubernetesVersion is the version of Kubernetes to install.
	KubernetesVersion string
	// ValuesOverrides is a map of values to override in the chart.
	ValuesOverrides map[string]any
	// RepoURL is the URL of the Helm repository to use.
	RepoURL string
}

// NewRequest returns a new Request with the given release name and namespace.
func NewRequest() Request {
	return Request{
		ChartVersion: DefaultChartVersion,
		RepoURL:      DefaultChartRef,
	}
}

var (
	// ErrReleaseNotFound is returned when a release is not found.
	ErrReleaseNotFound = errors.New("release not found")
)

// Provisioner is responsible for provisioning Zones clusters.
type Provisioner interface {
	// Install is installing the vCluster chart.
	Install(ctx context.Context, req Request) error
	// Status is getting the status of the vCluster release.
	Status(ctx context.Context, req Request) (string, error)
	// Uninstall is uninstalling the vCluster release.
	Uninstall(ctx context.Context, req Request) error
}
