package namespace

import (
	"flag"
	"strings"
	"time"
)

type namespaceFlagValues struct {
	Image               *string
	Size                *string
	Repository          *string
	Site                *string
	VolumeSizeGB        *int
	AutoStopIdleTimeout *string
	WorkRoot            *string
	DeleteOnRelease     *bool
}

func RegisterNamespaceProviderFlags(fs *flag.FlagSet, defaults Config) any {
	return namespaceFlagValues{
		Image:               fs.String("namespace-image", defaults.Namespace.Image, "Namespace Devbox image"),
		Size:                fs.String("namespace-size", defaults.Namespace.Size, "Namespace Devbox size: S, M, L, or XL"),
		Repository:          fs.String("namespace-repository", defaults.Namespace.Repository, "Namespace Devbox repository checkout"),
		Site:                fs.String("namespace-site", defaults.Namespace.Site, "Namespace Devbox site"),
		VolumeSizeGB:        fs.Int("namespace-volume-size-gb", defaults.Namespace.VolumeSizeGB, "Namespace Devbox persistent volume size in GiB"),
		AutoStopIdleTimeout: fs.String("namespace-auto-stop-idle-timeout", defaults.Namespace.AutoStopIdleTimeout.String(), "Namespace Devbox idle auto-stop timeout"),
		WorkRoot:            fs.String("namespace-work-root", defaults.Namespace.WorkRoot, "Namespace Devbox Crabbox work root"),
		DeleteOnRelease:     fs.Bool("namespace-delete-on-release", defaults.Namespace.DeleteOnRelease, "delete Namespace Devbox on release instead of shutting it down"),
	}
}

func ApplyNamespaceProviderFlags(cfg *Config, fs *flag.FlagSet, values any) error {
	v, ok := values.(namespaceFlagValues)
	if !ok {
		return nil
	}
	if flagWasSet(fs, "namespace-image") {
		cfg.Namespace.Image = *v.Image
	}
	if flagWasSet(fs, "namespace-size") {
		cfg.Namespace.Size = strings.ToUpper(strings.TrimSpace(*v.Size))
		cfg.ServerType = cfg.Namespace.Size
		cfg.ServerTypeExplicit = true
	}
	if flagWasSet(fs, "namespace-repository") {
		cfg.Namespace.Repository = *v.Repository
	}
	if flagWasSet(fs, "namespace-site") {
		cfg.Namespace.Site = *v.Site
	}
	if flagWasSet(fs, "namespace-volume-size-gb") {
		cfg.Namespace.VolumeSizeGB = *v.VolumeSizeGB
	}
	if flagWasSet(fs, "namespace-auto-stop-idle-timeout") {
		applyNamespaceDuration(&cfg.Namespace.AutoStopIdleTimeout, *v.AutoStopIdleTimeout)
	}
	if flagWasSet(fs, "namespace-work-root") {
		cfg.Namespace.WorkRoot = *v.WorkRoot
		cfg.WorkRoot = *v.WorkRoot
	}
	if flagWasSet(fs, "namespace-delete-on-release") {
		cfg.Namespace.DeleteOnRelease = *v.DeleteOnRelease
	}
	return validateNamespaceConfig(*cfg)
}

func validateNamespaceConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Namespace.Size) != "" {
		switch strings.ToUpper(strings.TrimSpace(cfg.Namespace.Size)) {
		case "S", "M", "L", "XL":
		default:
			return exit(2, "namespace devbox size must be S, M, L, or XL")
		}
	}
	size := namespaceSize(cfg)
	switch size {
	case "S", "M", "L", "XL":
	default:
		return exit(2, "namespace devbox size must be S, M, L, or XL")
	}
	if cfg.Namespace.VolumeSizeGB < 0 {
		return exit(2, "namespace volume size must be non-negative")
	}
	return nil
}

func applyNamespaceDuration(target *time.Duration, value string) {
	if value == "" {
		return
	}
	if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
		*target = parsed
	}
}
