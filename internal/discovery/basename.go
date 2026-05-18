package discovery

import (
	"strings"

	"go-monitoring/config"
)

const (
	literalNoHook   = "NO-HOOK"
	literalUnknown  = "UNKNOWN"
	suffixUnderlying = "underlying"
	suffixBoosted    = "boosted"
)

// slugSegment uppercases ASCII letters, keeps digits, splits other runes into
// single '-' separators, and trims leading/trailing dashes.
func slugSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			if prevDash {
				prevDash = false
			}
			b.WriteRune(r - ('a' - 'A'))
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			if prevDash {
				prevDash = false
			}
			b.WriteRune(r)
		default:
			if b.Len() > 0 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func slugTokenSymbol(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return literalUnknown
	}
	out := slugSegment(t)
	if out == "" {
		return literalUnknown
	}
	return out
}

// formatDiscoveredBaseName builds the human-readable BaseName for a discovery
// test row (before solver prefix). See docs/discovery.md.
func formatDiscoveredBaseName(r TestRow) string {
	net := slugSegment(config.NetworkName(r.Network))
	if net == "" {
		net = literalUnknown
	}

	poolType := slugSegment(r.PoolType)
	if poolType == "" {
		poolType = literalUnknown
	}

	var hook string
	if strings.TrimSpace(r.HookType) == "" {
		hook = literalNoHook
	} else {
		hook = slugSegment(r.HookType)
		if hook == "" {
			hook = literalNoHook
		}
	}

	symIn := slugTokenSymbol(r.TokenInSymbol)
	symOut := slugTokenSymbol(r.TokenOutSymbol)
	addr := shortAddr(r.PoolAddress)

	parts := []string{net, poolType, hook, symIn, symOut, addr}
	base := strings.Join(parts, "-")

	switch {
	case r.Variant == "underlying":
		base += "-" + suffixUnderlying
	case r.Boosted:
		base += "-" + suffixBoosted
	}
	return base
}
