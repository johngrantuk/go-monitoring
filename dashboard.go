package main

import (
	"fmt"
	"net/http"
	"time"
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

// checkEndpointHandler triggers a check for a specific endpoint
func checkEndpointHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Path[len("/check/"):]
	
	mu.Lock()
	var targetEndpoint *Endpoint
	for i := range endpoints {
		if endpoints[i].Name == name {
			targetEndpoint = &endpoints[i]
			break
		}
	}
	mu.Unlock()

	if targetEndpoint == nil {
		http.Error(w, "Endpoint not found", http.StatusNotFound)
		return
	}

	// Trigger the check
	checkAPI(targetEndpoint)

	// Redirect back to the dashboard
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Dashboard handler
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

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
	fmt.Fprintln(w, "<table border='1'><tr><th class='name-column'>Name</th><th>Status</th><th>Message</th><th>Last Checked</th><th>Route Solver</th><th>Network</th><th>Token Info</th><th>Actions</th></tr>")
	for _, endpoint := range endpoints {
		statusClass := "status-unknown"
		if endpoint.LastStatus == "up" {
			statusClass = "status-up"
		} else if endpoint.LastStatus == "down" {
			statusClass = "status-down"
		} else if endpoint.LastStatus == "disabled" {
			statusClass = "status-disabled"
		}
		tokenInfo := fmt.Sprintf("In: %s<br>Out: %s<br>Pool: %s", endpoint.TokenIn, endpoint.TokenOut, endpoint.ExpectedPool)
		fmt.Fprintf(w, "<tr><td class='name-column'>%s</td><td class='%s'>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class='token-info'>%s</td><td><button class='check-button' onclick='checkEndpoint(\"%s\")'>Check Now</button></td></tr>",
			endpoint.Name,
			statusClass,
			endpoint.LastStatus,
			endpoint.Message,
			formatTimeAgo(endpoint.LastChecked),
			endpoint.RouteSolver,
			endpoint.Network,
			tokenInfo,
			endpoint.Name)
	}
	fmt.Fprintln(w, "</table></body></html>")
}

func init() {
	// Register the check endpoint handler
	http.HandleFunc("/check/", checkEndpointHandler)
}