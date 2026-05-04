package cli

import (
	"strings"
	"testing"
)

func TestValidateProviderTargetRejectsUnsupportedAWSTargets(t *testing.T) {
	t.Run("macOS needs dedicated host", func(t *testing.T) {
		cfg := baseConfig()
		cfg.Provider = "aws"
		cfg.TargetOS = targetMacOS
		err := validateProviderTarget(cfg)
		if err == nil || !strings.Contains(err.Error(), "requires CRABBOX_AWS_MAC_HOST_ID") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("Windows WSL2 is not brokered by AWS", func(t *testing.T) {
		cfg := baseConfig()
		cfg.Provider = "aws"
		cfg.TargetOS = targetWindows
		cfg.WindowsMode = windowsModeWSL2
		err := validateProviderTarget(cfg)
		if err == nil || !strings.Contains(err.Error(), "currently supports target=linux only") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestValidateProviderTargetAllowsAWSNativeWindows(t *testing.T) {
	cfg := baseConfig()
	cfg.Provider = "aws"
	cfg.TargetOS = targetWindows
	cfg.WindowsMode = windowsModeNormal
	if err := validateProviderTarget(cfg); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestValidateProviderTargetAllowsStaticNonLinux(t *testing.T) {
	for _, target := range []string{targetMacOS, targetWindows} {
		cfg := baseConfig()
		cfg.Provider = staticProvider
		cfg.TargetOS = target
		if err := validateProviderTarget(cfg); err != nil {
			t.Fatalf("target=%s err=%v", target, err)
		}
	}
}
