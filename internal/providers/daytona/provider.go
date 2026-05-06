package daytona

import (
	"flag"

	"github.com/openclaw/crabbox/internal/cli"
)

func init() {
	cli.RegisterProvider(Provider{})
}

type Provider struct{}

func (Provider) Name() string { return "daytona" }
func (Provider) Aliases() []string {
	return nil
}
func (Provider) Spec() cli.ProviderSpec {
	return cli.ProviderSpec{
		Name:        "daytona",
		Kind:        cli.ProviderKindSSHLease,
		Targets:     []cli.TargetSpec{{OS: "linux"}},
		Features:    cli.FeatureSet{cli.FeatureSSH, cli.FeatureCrabboxSync},
		Coordinator: cli.CoordinatorNever,
	}
}
func (Provider) RegisterFlags(fs *flag.FlagSet, defaults cli.Config) any {
	return cli.RegisterDaytonaProviderFlags(fs, defaults)
}
func (Provider) ApplyFlags(cfg *cli.Config, fs *flag.FlagSet, values any) error {
	return cli.ApplyDaytonaProviderFlags(cfg, fs, values)
}
func (p Provider) Configure(cfg cli.Config, rt cli.Runtime) (cli.Backend, error) {
	return cli.NewDaytonaLeaseBackend(p.Spec(), cfg, rt), nil
}
