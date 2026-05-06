package hetzner

import (
	"flag"

	"github.com/openclaw/crabbox/internal/cli"
)

func init() {
	cli.RegisterProvider(Provider{})
}

type Provider struct{}

func (Provider) Name() string      { return "hetzner" }
func (Provider) Aliases() []string { return nil }
func (Provider) Spec() cli.ProviderSpec {
	return cli.ProviderSpec{
		Name:        "hetzner",
		Kind:        cli.ProviderKindSSHLease,
		Targets:     []cli.TargetSpec{{OS: "linux"}},
		Features:    cli.FeatureSet{cli.FeatureSSH, cli.FeatureCrabboxSync, cli.FeatureCleanup, cli.FeatureDesktop, cli.FeatureBrowser, cli.FeatureCode, cli.FeatureTailscale},
		Coordinator: cli.CoordinatorSupported,
	}
}
func (Provider) RegisterFlags(*flag.FlagSet, cli.Config) any { return cli.NoProviderFlags() }
func (Provider) ApplyFlags(*cli.Config, *flag.FlagSet, any) error {
	return nil
}
func (p Provider) Configure(cfg cli.Config, rt cli.Runtime) (cli.Backend, error) {
	return cli.NewHetznerLeaseBackend(p.Spec(), cfg, rt), nil
}
