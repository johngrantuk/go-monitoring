package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"go-monitoring/internal/collector"
	"go-monitoring/internal/monitor"
	"math/big"
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
	case "9745":
		return "plasma"
	case "143":
		return "monad"
	default:
		return network
	}
}

// CheckEndpointHandler triggers a check for a specific endpoint
func CheckEndpointHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Path[len("/check/"):]

	// Use the collector to update the endpoint directly
	updated := collector.UpdateEndpointByName(name, func(endpoint *collector.Endpoint) {
		// Make both calls: Balancer-only and market price
		monitor.CheckAPI(endpoint, nil) // nil options will trigger both calls
	})

	if !updated {
		http.Error(w, "Endpoint not found", http.StatusNotFound)
		return
	}

	// Redirect back to the dashboard
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// DashboardHandler handles the main dashboard page
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
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
			.highest-value { background-color: #90EE90; font-weight: bold; }
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
			.sortable-header {
				cursor: pointer;
				user-select: none;
				position: relative;
				padding-right: 20px;
			}
			.sortable-header:hover {
				background-color: #e0e0e0;
			}
			.sort-arrow {
				position: absolute;
				right: 5px;
				top: 50%;
				transform: translateY(-50%);
				font-size: 12px;
				color: #666;
			}
			.sort-arrow.active {
				color: #000;
				font-weight: bold;
			}
		</style>
		<script>
			let currentSort = { column: 4, direction: 'desc' }; // Default sort by Market Price desc
			
			function checkEndpoint(name) {
				fetch('/check/' + name, {
					method: 'POST',
				}).then(() => {
					window.location.reload();
				});
			}
			
			function sortTable(column) {
				const table = document.querySelector('table');
				const tbody = table.querySelector('tbody');
				const allRows = Array.from(tbody.querySelectorAll('tr'));
				
				// Determine sort direction
				if (currentSort.column === column) {
					currentSort.direction = currentSort.direction === 'asc' ? 'desc' : 'asc';
				} else {
					currentSort.column = column;
					currentSort.direction = 'desc';
				}
				
				// Update arrow indicators
				document.querySelectorAll('.sort-arrow').forEach(arrow => {
					arrow.classList.remove('active');
					arrow.textContent = '↕';
				});
				
				const activeArrow = document.getElementById('arrow-' + column);
				activeArrow.classList.add('active');
				activeArrow.textContent = currentSort.direction === 'asc' ? '↑' : '↓';
				
				// Group rows by base name rows
				const groups = [];
				let currentGroup = null;
				
				allRows.forEach(row => {
					if (row.classList.contains('base-name-row')) {
						currentGroup = { header: row, solvers: [] };
						groups.push(currentGroup);
					} else if (row.classList.contains('solver-row') && currentGroup) {
						currentGroup.solvers.push(row);
					}
				});
				
				// Sort solver rows within each group
				groups.forEach(group => {
					group.solvers.sort((a, b) => {
						const aVal = a.cells[column].textContent.trim();
						const bVal = b.cells[column].textContent.trim();
						
						// Handle N/A values
						if (aVal === 'N/A' && bVal === 'N/A') return 0;
						if (aVal === 'N/A') return 1;
						if (bVal === 'N/A') return -1;
						
						// Parse as BigInt for proper large number comparison
						let aNum, bNum;
						try {
							aNum = BigInt(aVal);
							bNum = BigInt(bVal);
						} catch (e) {
							// Fallback to 0 if parsing fails
							aNum = BigInt(0);
							bNum = BigInt(0);
						}
						
						if (currentSort.direction === 'asc') {
							return aNum < bNum ? -1 : aNum > bNum ? 1 : 0;
						} else {
							return aNum > bNum ? -1 : aNum < bNum ? 1 : 0;
						}
					});
				});
				
				// Clear tbody and re-append sorted groups
				tbody.innerHTML = '';
				groups.forEach(group => {
					tbody.appendChild(group.header);
					group.solvers.forEach(solver => {
						tbody.appendChild(solver);
					});
				});
			}
			
			// Initialize default sort on page load
			document.addEventListener('DOMContentLoaded', function() {
				// Small delay to ensure all content is rendered
				setTimeout(function() {
					// Force the sort state and apply sorting
					currentSort.column = 4;
					currentSort.direction = 'asc';
					sortTable(4); // Sort by Market Price desc by default
				}, 100);
			});
		</script>
	</head><body><h1>API Monitor</h1>`)
	fmt.Fprintln(w, "<table border='1'><thead><tr><th class='name-column'>Name</th><th>Status</th><th>Message</th><th class='sortable-header' onclick='sortTable(3)'>Balancer Price<span class='sort-arrow' id='arrow-3'>↕</span></th><th class='sortable-header' onclick='sortTable(4)'>Market Price<span class='sort-arrow' id='arrow-4'>↕</span></th><th>Last Checked</th><th>Actions</th></tr></thead><tbody>")

	for _, baseName := range baseNames {
		// Add base name row with token info
		networkName := getNetworkName(endpointGroups[baseName][0].Network)
		poolLink := fmt.Sprintf("https://balancer.fi/pools/%s/v3/%s", networkName, endpointGroups[baseName][0].ExpectedPool)
		fmt.Fprintf(w, "<tr class='base-name-row'><td colspan='7'>%s<br><span style='font-weight: normal; font-size: 0.9em; margin-top: 10px; display: inline-block;'>In: %s<br>Out: %s<br>Pool: <a href='%s' target='_blank'>%s</a><br>Amount: %s</span></td></tr>",
			baseName,
			endpointGroups[baseName][0].TokenIn,
			endpointGroups[baseName][0].TokenOut,
			poolLink,
			endpointGroups[baseName][0].ExpectedPool,
			endpointGroups[baseName][0].SwapAmount)

		// Add solver rows
		// Sort endpoints by return amount (largest first)
		sortedEndpoints := make([]collector.Endpoint, len(endpointGroups[baseName]))
		copy(sortedEndpoints, endpointGroups[baseName])

		// Sort by return amount in descending order
		sort.Slice(sortedEndpoints, func(i, j int) bool {
			// Convert return amounts to big.Int for proper numeric comparison
			amountI := sortedEndpoints[i].ReturnAmount
			amountJ := sortedEndpoints[j].ReturnAmount

			// If either amount is empty/N/A, treat as 0
			if amountI == "" || amountI == "N/A" {
				amountI = "0"
			}
			if amountJ == "" || amountJ == "N/A" {
				amountJ = "0"
			}

			// Parse as big.Int for comparison
			bigI := new(big.Int)
			bigJ := new(big.Int)

			if _, ok := bigI.SetString(amountI, 10); !ok {
				bigI.SetString("0", 10)
			}
			if _, ok := bigJ.SetString(amountJ, 10); !ok {
				bigJ.SetString("0", 10)
			}

			// Return true if i should come before j (larger amount first)
			return bigI.Cmp(bigJ) > 0
		})

		for _, endpoint := range sortedEndpoints {
			statusClass := "status-unknown"
			if endpoint.LastStatus == "up" {
				statusClass = "status-up"
			} else if endpoint.LastStatus == "down" {
				statusClass = "status-down"
			} else if endpoint.LastStatus == "disabled" {
				statusClass = "status-disabled"
			}

			// Format return amount display
			returnAmountDisplay := "N/A"
			if endpoint.ReturnAmount != "" {
				returnAmountDisplay = endpoint.ReturnAmount
			}

			// Format market price display
			marketPriceDisplay := "N/A"
			if endpoint.MarketPrice != "" {
				marketPriceDisplay = endpoint.MarketPrice
			}

			// Compare return amount vs market price within this row and highlight the larger value
			returnAmountClass := ""
			marketPriceClass := ""

			// Parse return amount
			returnAmountStr := endpoint.ReturnAmount
			if returnAmountStr == "" || returnAmountStr == "N/A" {
				returnAmountStr = "0"
			}
			returnAmountBig := new(big.Int)
			if _, ok := returnAmountBig.SetString(returnAmountStr, 10); !ok {
				returnAmountBig.SetString("0", 10)
			}

			// Parse market price
			marketPriceStr := endpoint.MarketPrice
			if marketPriceStr == "" || marketPriceStr == "N/A" {
				marketPriceStr = "0"
			}
			marketPriceBig := new(big.Int)
			if _, ok := marketPriceBig.SetString(marketPriceStr, 10); !ok {
				marketPriceBig.SetString("0", 10)
			}

			// Compare and highlight the larger value (only if both are valid non-zero values)
			if returnAmountBig.Cmp(big.NewInt(0)) > 0 || marketPriceBig.Cmp(big.NewInt(0)) > 0 {
				if returnAmountBig.Cmp(marketPriceBig) > 0 {
					returnAmountClass = " class='highest-value'"
				} else if marketPriceBig.Cmp(returnAmountBig) > 0 {
					marketPriceClass = " class='highest-value'"
				}
				// If they're equal, don't highlight either
			}

			fmt.Fprintf(w, "<tr class='solver-row'><td class='name-column'>%s</td><td class='%s'>%s</td><td>%s</td><td%s>%s</td><td%s>%s</td><td>%s</td><td><button class='check-button' onclick='checkEndpoint(\"%s\")'>Check Now</button></td></tr>",
				endpoint.SolverName,
				statusClass,
				endpoint.LastStatus,
				endpoint.Message,
				returnAmountClass,
				returnAmountDisplay,
				marketPriceClass,
				marketPriceDisplay,
				formatTimeAgo(endpoint.LastChecked),
				endpoint.Name)
		}
	}
	fmt.Fprintln(w, "</tbody></table></body></html>")
}
