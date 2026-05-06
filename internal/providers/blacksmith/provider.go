package blacksmith

import (
	"flag"

	"github.com/openclaw/crabbox/internal/cli"
)

func init() {
	cli.RegisterProvider(Provider{})
}

type Provider struct{}

func (Provider) Name() string { return "blacksmith-testbox" }
func (Provider) Aliases() []string {
	return []string{"blacksmith"}
}
func (Provider) Spec() cli.ProviderSpec {
	return cli.ProviderSpec{
		Name:        "blacksmith-testbox",
		Kind:        cli.ProviderKindDelegatedRun,
		Targets:     []cli.TargetSpec{{OS: "linux"}},
		Features:    nil,
		Coordinator: cli.CoordinatorNever,
	}
}
func (Provider) RegisterFlags(fs *flag.FlagSet, defaults cli.Config) any {
	return cli.RegisterBlacksmithProviderFlags(fs, defaults)
}
func (Provider) ApplyFlags(cfg *cli.Config, fs *flag.FlagSet, values any) error {
	return cli.ApplyBlacksmithProviderFlags(cfg, fs, values)
}
func (p Provider) Configure(cfg cli.Config, rt cli.Runtime) (cli.Backend, error) {
	return cli.NewBlacksmithBackend(p.Spec(), cfg, rt), nil
}
