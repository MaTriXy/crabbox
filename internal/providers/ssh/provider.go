package ssh

import (
	"flag"

	core "github.com/openclaw/crabbox/internal/cli"
)

func init() {
	core.RegisterProvider(Provider{})
}

type Provider struct{}

func (Provider) Name() string { return "ssh" }
func (Provider) Aliases() []string {
	return []string{"static", "static-ssh"}
}
func (Provider) Spec() core.ProviderSpec {
	return core.ProviderSpec{
		Name: "ssh",
		Kind: core.ProviderKindSSHLease,
		Targets: []core.TargetSpec{
			{OS: core.TargetLinux},
			{OS: core.TargetWindows, WindowsMode: "normal"},
			{OS: core.TargetWindows, WindowsMode: "wsl2"},
			{OS: core.TargetMacOS},
		},
		Features:    core.FeatureSet{core.FeatureSSH, core.FeatureCrabboxSync, core.FeatureDesktop, core.FeatureBrowser, core.FeatureCode},
		Coordinator: core.CoordinatorNever,
	}
}
func (Provider) RegisterFlags(*flag.FlagSet, core.Config) any { return core.NoProviderFlags() }
func (Provider) ApplyFlags(*core.Config, *flag.FlagSet, any) error {
	return nil
}
func (p Provider) Configure(cfg core.Config, rt core.Runtime) (core.Backend, error) {
	return NewStaticSSHLeaseBackend(p.Spec(), cfg, rt), nil
}
