package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"go-monitoring/internal/collector"
	"go-monitoring/internal/monitor"
)

// formatTimeAgo returns a human-readable time format
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}

	now := time.Now()
	diff := now.Sub(t)

	// If less than a minute ago
	if diff < time.Minute {
		return "Just now"
	}

	// If less than an hour ago
	if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	// If less than a day ago
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	// If more than a day ago, show date and time
	return t.Format("Jan 02 15:04:05")
}

// getNetworkName maps network IDs to their names
func getNetworkName(network string) string {
	switch network {
	case "1":
		return "ethereum"
	case "8453":
		return "base"
	case "42161":
		return "arbitrum"
	case "100":
		return "gnosis"
	case "43114":
		return "avalanche"
	case "999":
		return "hyperevm"
	default:
		return network
	}
}

// checkEndpointHandler triggers a check for a specific endpoint
func checkEndpointHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Path[len("/check/"):]

	// Use the collector to update the endpoint directly
	updated := collector.UpdateEndpointByName(name, func(endpoint *collector.Endpoint) {
		monitor.CheckAPI(endpoint)
	})

	if !updated {
		http.Error(w, "Endpoint not found", http.StatusNotFound)
		return
	}

	// Redirect back to the dashboard
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Dashboard handler
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	// Get a copy of endpoints from the collector
	endpoints := collector.GetEndpointsCopy()

	// Group endpoints by BaseName
	endpointGroups := make(map[string][]collector.Endpoint)
	for _, endpoint := range endpoints {
		endpointGroups[endpoint.BaseName] = append(endpointGroups[endpoint.BaseName], endpoint)
	}

	// Sort base names for consistent display
	var baseNames []string
	for baseName := range endpointGroups {
		baseNames = append(baseNames, baseName)
	}
	sort.Strings(baseNames)

	fmt.Fprintln(w, `<html><head>
		<style>
			.status-up { background-color: #90EE90; }
			.status-down { background-color: #FFB6C1; }
			.status-unknown { background-color: #FFA500; }
			.status-disabled { background-color: #D3D3D3; }
			table { border-collapse: collapse; width: 100%; }
			th, td { padding: 8px; text-align: left; }
			.name-column { white-space: nowrap; }
			.token-info { font-family: monospace; }
			.check-button {
				background-color: #4CAF50;
				border: none;
				color: white;
				padding: 5px 10px;
				text-align: center;
				text-decoration: none;
				display: inline-block;
				font-size: 14px;
				margin: 4px 2px;
				cursor: pointer;
				border-radius: 4px;
			}
			.check-button:hover {
				background-color: #45a049;
			}
			.base-name-row {
				background-color: #e6f3ff;
				font-weight: bold;
			}
			.solver-row {
				background-color: #f9f9f9;
			}
		</style>
		<script>
			function checkEndpoint(name) {
				fetch('/check/' + name, {
					method: 'POST',
				}).then(() => {
					window.location.reload();
				});
			}
		</script>
	</head><body><h1>API Monitor</h1>`)
	fmt.Fprintln(w, "<table border='1'><tr><th class='name-column'>Name</th><th>Status</th><th>Message</th><th>Last Checked</th><th>Actions</th></tr>")

	for _, baseName := range baseNames {
		// Add base name row with token info
		networkName := getNetworkName(endpointGroups[baseName][0].Network)
		poolLink := fmt.Sprintf("https://balancer.fi/pools/%s/v3/%s", networkName, endpointGroups[baseName][0].ExpectedPool)
		fmt.Fprintf(w, "<tr class='base-name-row'><td colspan='5'>%s<br><span style='font-weight: normal; font-size: 0.9em; margin-top: 10px; display: inline-block;'>In: %s<br>Out: %s<br>Pool: <a href='%s' target='_blank'>%s</a></span></td></tr>",
			baseName,
			endpointGroups[baseName][0].TokenIn,
			endpointGroups[baseName][0].TokenOut,
			poolLink,
			endpointGroups[baseName][0].ExpectedPool)

		// Add solver rows
		for _, endpoint := range endpointGroups[baseName] {
			statusClass := "status-unknown"
			if endpoint.LastStatus == "up" {
				statusClass = "status-up"
			} else if endpoint.LastStatus == "down" {
				statusClass = "status-down"
			} else if endpoint.LastStatus == "disabled" {
				statusClass = "status-disabled"
			}
			fmt.Fprintf(w, "<tr class='solver-row'><td class='name-column'>%s</td><td class='%s'>%s</td><td>%s</td><td>%s</td><td><button class='check-button' onclick='checkEndpoint(\"%s\")'>Check Now</button></td></tr>",
				endpoint.SolverName,
				statusClass,
				endpoint.LastStatus,
				endpoint.Message,
				formatTimeAgo(endpoint.LastChecked),
				endpoint.Name)
		}
	}
	fmt.Fprintln(w, "</table></body></html>")
}

func init() {
	// Register the check endpoint handler
	http.HandleFunc("/check/", checkEndpointHandler)
}
