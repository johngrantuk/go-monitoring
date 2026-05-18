package handlers

import (
	"fmt"
	"time"
)

// formatTimeAgo returns a human-readable relative time. Returns "Never" for the
// zero time.
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}

	diff := time.Since(t)

	if diff < time.Minute {
		return "Just now"
	}

	if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	return t.Format("Jan 02 15:04:05")
}

// getNetworkName maps a numeric network ID to its lowercase friendly name.
// Returns the input unchanged if no mapping is known.
func getNetworkName(network string) string {
	switch network {
	case "1":
		return "ethereum"
	case "8453":
		return "base"
	case "42161":
		return "arbitrum"
	case "10":
		return "optimism"
	case "100":
		return "gnosis"
	case "43114":
		return "avalanche"
	case "999":
		return "hyperevm"
	case "9745":
		return "plasma"
	case "143":
		return "monad"
	default:
		return network
	}
}
