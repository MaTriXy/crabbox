package ssh

import (
	"flag"

	"github.com/openclaw/crabbox/internal/cli"
)

func init() {
	cli.RegisterProvider(Provider{})
}

type Provider struct{}

func (Provider) Name() string { return "ssh" }
func (Provider) Aliases() []string {
	return []string{"static", "static-ssh"}
}
func (Provider) Spec() cli.ProviderSpec {
	return cli.ProviderSpec{
		Name: "ssh",
		Kind: cli.ProviderKindSSHLease,
		Targets: []cli.TargetSpec{
			{OS: "linux"},
			{OS: "windows", WindowsMode: "normal"},
			{OS: "windows", WindowsMode: "wsl2"},
			{OS: "macos"},
		},
		Features:    cli.FeatureSet{cli.FeatureSSH, cli.FeatureCrabboxSync, cli.FeatureDesktop, cli.FeatureBrowser, cli.FeatureCode},
		Coordinator: cli.CoordinatorNever,
	}
}
func (Provider) RegisterFlags(*flag.FlagSet, cli.Config) any { return cli.NoProviderFlags() }
func (Provider) ApplyFlags(*cli.Config, *flag.FlagSet, any) error {
	return nil
}
func (p Provider) Configure(cfg cli.Config, rt cli.Runtime) (cli.Backend, error) {
	return cli.NewStaticSSHLeaseBackend(p.Spec(), cfg, rt), nil
}
