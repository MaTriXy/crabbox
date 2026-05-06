package cli

import "flag"

func init() {
	RegisterProvider(testHetznerProvider{})
	RegisterProvider(testAWSProvider{})
	RegisterProvider(testStaticSSHProvider{})
	RegisterProvider(testBlacksmithProvider{})
}

type testHetznerProvider struct{}

func (testHetznerProvider) Name() string      { return "hetzner" }
func (testHetznerProvider) Aliases() []string { return nil }
func (testHetznerProvider) Spec() ProviderSpec {
	return ProviderSpec{
		Name:        "hetzner",
		Kind:        ProviderKindSSHLease,
		Targets:     []TargetSpec{{OS: targetLinux}},
		Features:    FeatureSet{FeatureSSH, FeatureCrabboxSync, FeatureCleanup, FeatureDesktop, FeatureBrowser, FeatureCode, FeatureTailscale},
		Coordinator: CoordinatorSupported,
	}
}
func (testHetznerProvider) RegisterFlags(*flag.FlagSet, Config) any { return noProviderFlags{} }
func (testHetznerProvider) ApplyFlags(*Config, *flag.FlagSet, any) error {
	return nil
}
func (p testHetznerProvider) Configure(cfg Config, rt Runtime) (Backend, error) {
	return NewHetznerLeaseBackend(p.Spec(), cfg, rt), nil
}

type testAWSProvider struct{}

func (testAWSProvider) Name() string      { return "aws" }
func (testAWSProvider) Aliases() []string { return nil }
func (testAWSProvider) Spec() ProviderSpec {
	return ProviderSpec{
		Name: "aws",
		Kind: ProviderKindSSHLease,
		Targets: []TargetSpec{
			{OS: targetLinux},
			{OS: targetWindows, WindowsMode: windowsModeNormal},
			{OS: targetWindows, WindowsMode: windowsModeWSL2},
			{OS: targetMacOS},
		},
		Features:    FeatureSet{FeatureSSH, FeatureCrabboxSync, FeatureCleanup, FeatureDesktop, FeatureBrowser, FeatureCode},
		Coordinator: CoordinatorSupported,
	}
}
func (testAWSProvider) RegisterFlags(*flag.FlagSet, Config) any { return noProviderFlags{} }
func (testAWSProvider) ApplyFlags(*Config, *flag.FlagSet, any) error {
	return nil
}
func (p testAWSProvider) Configure(cfg Config, rt Runtime) (Backend, error) {
	return NewAWSLeaseBackend(p.Spec(), cfg, rt), nil
}

type testStaticSSHProvider struct{}

func (testStaticSSHProvider) Name() string { return staticProvider }
func (testStaticSSHProvider) Aliases() []string {
	return []string{"static", "static-ssh"}
}
func (testStaticSSHProvider) Spec() ProviderSpec {
	return ProviderSpec{
		Name: staticProvider,
		Kind: ProviderKindSSHLease,
		Targets: []TargetSpec{
			{OS: targetLinux},
			{OS: targetWindows, WindowsMode: windowsModeNormal},
			{OS: targetWindows, WindowsMode: windowsModeWSL2},
			{OS: targetMacOS},
		},
		Features:    FeatureSet{FeatureSSH, FeatureCrabboxSync, FeatureDesktop, FeatureBrowser, FeatureCode},
		Coordinator: CoordinatorNever,
	}
}
func (testStaticSSHProvider) RegisterFlags(*flag.FlagSet, Config) any { return noProviderFlags{} }
func (testStaticSSHProvider) ApplyFlags(*Config, *flag.FlagSet, any) error {
	return nil
}
func (p testStaticSSHProvider) Configure(cfg Config, rt Runtime) (Backend, error) {
	return NewStaticSSHLeaseBackend(p.Spec(), cfg, rt), nil
}

type testBlacksmithProvider struct{}

func (testBlacksmithProvider) Name() string { return blacksmithTestboxProvider }
func (testBlacksmithProvider) Aliases() []string {
	return []string{"blacksmith"}
}
func (testBlacksmithProvider) Spec() ProviderSpec {
	return ProviderSpec{
		Name:        blacksmithTestboxProvider,
		Kind:        ProviderKindDelegatedRun,
		Targets:     []TargetSpec{{OS: targetLinux}},
		Features:    nil,
		Coordinator: CoordinatorNever,
	}
}
func (testBlacksmithProvider) RegisterFlags(fs *flag.FlagSet, defaults Config) any {
	return RegisterBlacksmithProviderFlags(fs, defaults)
}
func (testBlacksmithProvider) ApplyFlags(cfg *Config, fs *flag.FlagSet, values any) error {
	return ApplyBlacksmithProviderFlags(cfg, fs, values)
}
func (p testBlacksmithProvider) Configure(cfg Config, rt Runtime) (Backend, error) {
	return NewBlacksmithBackend(p.Spec(), cfg, rt), nil
}
