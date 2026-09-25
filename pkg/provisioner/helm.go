package provisioner

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeiss/pkg/utilx"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultChartVersion is the default version of the Helm chart to use.
	DefaultChartVersion = "0.37.1"
	// DefaultChartRef is the default reference to the Helm chart to use.
	DefaultChartRef = "https://charts.loft.sh"
	// DefaultChartName is the default name of the Helm chart to use.
	DefaultChartName = "vcluster"
)

var (
	// StatusUnknown is the status of a release that is unknown.
	StatusUnknown = ""
)

var _ Provisioner = (*HelmProvisioner)(nil)

// HelmProvisioner is responsible for provisioning Zones clusters using Helm.
type HelmProvisioner struct {
	Opts *Opts
}

// Opts is a struct holding the options for the HelmProvisioner.
type Opts struct {
	// ChartVersion is the version of the Helm chart to use.
	ChartVersion string
	// ChartRef is the reference to the Helm chart to use.
	ChartRef string
	// ChartName is the name of the Helm chart to use.
	ChartName string
	// RestConfig is the REST config for the HelmProvisioner.
	RestConfig *rest.Config
}

// Opt is a function that modifies the options for the HelmProvisioner.
type Opt func(*Opts)

// WithRestConfig sets the REST config for the HelmProvisioner.
func WithRestConfig(config *rest.Config) Opt {
	return func(o *Opts) {
		o.RestConfig = config
	}
}

// NewHelmProvisioner creates a new HelmProvisioner.
func NewHelmProvisioner(opts ...Opt) *HelmProvisioner {
	options := new(Opts)

	for _, opt := range opts {
		opt(options)
	}

	return &HelmProvisioner{
		Opts: options,
	}
}

// Install installs the Helm chart using the specified options.
func (h *HelmProvisioner) Install(ctx context.Context, req Request) error {
	history := action.NewHistory(nil)
	history.Max = 1

	_, err := history.Run(req.ReleaseName)
	releaseExists := utilx.NotNil(err)
	if err != nil && !errors.Is(err, driver.ErrReleaseExists) {
		return err
	}

	if !releaseExists {
		return h.install(ctx, req)
	}

	return nil
}

// Status gets the status of the Helm release.
func (h *HelmProvisioner) Status(ctx context.Context, req Request) (string, error) {
	log := logf.FromContext(ctx).WithName("helm")

	cfg := new(action.Configuration)
	getter := &restClientGetter{
		namespace: req.Namespace,
	}

	err := cfg.Init(getter, req.Namespace, driver.SecretsDriverName, func(format string, v ...interface{}) {
		log.V(1).Info(fmt.Sprintf(format, v...))
	})
	if err != nil {
		return StatusUnknown, err
	}

	rc, err := registry.NewClient()
	if err != nil {
		return StatusUnknown, fmt.Errorf("creating helm registry client: %w", err)
	}

	cfg.RegistryClient = rc

	status := action.NewStatus(cfg)

	rel, err := status.Run(req.ReleaseName)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return StatusUnknown, fmt.Errorf("getting status of release %q: %w", req.ReleaseName, err)
	}

	return rel.Info.Status.String(), nil
}

func (h *HelmProvisioner) install(ctx context.Context, req Request) error {
	install := action.NewInstall(nil)
	install.ReleaseName = req.ReleaseName
	install.Namespace = req.Namespace
	install.CreateNamespace = false
	install.Wait = false
	install.RepoURL = req.RepoURL
	install.Version = req.ChartVersion

	values := map[string]interface{}{}

	_, err := install.RunWithContext(ctx, nil, values)
	if err != nil {
		return fmt.Errorf("installing vcluster release: %q: %w", req.ReleaseName, err)
	}

	return nil
}
