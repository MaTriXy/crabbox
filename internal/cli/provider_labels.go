package cli

import (
	"fmt"
	"time"
)

func directLeaseLabels(cfg Config, leaseID, slug, provider, market string, keep bool, now time.Time) map[string]string {
	labels := map[string]string{
		"class":           cfg.Class,
		"crabbox":         "true",
		"created_by":      "crabbox",
		"keep":            fmt.Sprint(keep),
		"lease":           leaseID,
		"slug":            normalizeLeaseSlug(slug),
		"profile":         cfg.Profile,
		"provider_key":    cfg.ProviderKey,
		"provider":        provider,
		"server_type":     cfg.ServerType,
		"state":           "leased",
		"created_at":      now.Format(time.RFC3339),
		"last_touched_at": now.Format(time.RFC3339),
		"idle_timeout":    cfg.IdleTimeout.String(),
		"expires_at":      directLeaseExpiresAt(now, cfg).Format(time.RFC3339),
	}
	if market != "" {
		labels["market"] = market
	}
	return labels
}
