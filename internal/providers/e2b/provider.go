package e2b

import (
	"flag"

	core "github.com/openclaw/crabbox/internal/cli"
)

func init() {
	core.RegisterProvider(Provider{})
}

type Provider struct{}

func (Provider) Name() string      { return e2bProvider }
func (Provider) Aliases() []string { return nil }

func (Provider) Spec() core.ProviderSpec {
	return core.ProviderSpec{
		Name:        e2bProvider,
		Kind:        core.ProviderKindDelegatedRun,
		Targets:     []core.TargetSpec{{OS: core.TargetLinux}},
		Features:    nil,
		Coordinator: core.CoordinatorNever,
	}
}

func (Provider) RegisterFlags(fs *flag.FlagSet, defaults core.Config) any {
	return RegisterE2BProviderFlags(fs, defaults)
}

func (Provider) ApplyFlags(cfg *core.Config, fs *flag.FlagSet, values any) error {
	return ApplyE2BProviderFlags(cfg, fs, values)
}

func (p Provider) Configure(cfg core.Config, rt core.Runtime) (core.Backend, error) {
	return NewE2BBackend(p.Spec(), cfg, rt), nil
}
