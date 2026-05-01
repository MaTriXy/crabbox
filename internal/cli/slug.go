package cli

import (
	"fmt"
	"hash/fnv"
	"strings"
)

var leaseSlugAdjectives = []string{
	"amber",
	"blue",
	"brisk",
	"coral",
	"crimson",
	"golden",
	"harbor",
	"jade",
	"pearl",
	"quick",
	"silver",
	"swift",
	"tidal",
	"violet",
}

var leaseSlugNouns = []string{
	"barnacle",
	"crab",
	"crayfish",
	"hermit",
	"krill",
	"lobster",
	"prawn",
	"shrimp",
}

func newLeaseSlug(leaseID string) string {
	hash := leaseSlugHash(leaseID)
	adjective := leaseSlugAdjectives[int(hash%uint32(len(leaseSlugAdjectives)))]
	noun := leaseSlugNouns[int((hash/uint32(len(leaseSlugAdjectives)))%uint32(len(leaseSlugNouns)))]
	return adjective + "-" + noun
}

func slugWithCollisionSuffix(base, seed string) string {
	base = normalizeLeaseSlug(base)
	if base == "" {
		base = newLeaseSlug(seed)
	}
	return fmt.Sprintf("%s-%04x", base, leaseSlugHash(seed)&0xffff)
}

func normalizeLeaseSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func leaseProviderName(leaseID, slug string) string {
	if slug = normalizeLeaseSlug(slug); slug != "" {
		return "crabbox-" + slug
	}
	return strings.ReplaceAll("crabbox-"+leaseID, "_", "-")
}

func allocateDirectLeaseSlug(leaseID string, servers []Server) string {
	base := newLeaseSlug(leaseID)
	slug := base
	for attempt := 0; attempt < 20; attempt++ {
		if !serverSlugInUse(slug, servers) {
			return slug
		}
		slug = slugWithCollisionSuffix(base, fmt.Sprintf("%s-%d", leaseID, attempt))
	}
	return slugWithCollisionSuffix(base, leaseID)
}

func serverSlugInUse(slug string, servers []Server) bool {
	slug = normalizeLeaseSlug(slug)
	for _, server := range servers {
		if serverSlug(server) == slug {
			return true
		}
	}
	return false
}

func serverSlug(server Server) string {
	if server.Labels == nil {
		return ""
	}
	return normalizeLeaseSlug(server.Labels["slug"])
}

func leaseSlugHash(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}
