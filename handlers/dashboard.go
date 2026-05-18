package handlers

import (
	"fmt"
	"math/big"
	"net/http"
	"sort"

	"go-monitoring/internal/collector"
	"go-monitoring/internal/discovery"
	"go-monitoring/internal/monitor"
)

// CheckEndpointHandler triggers a check for a specific endpoint. Tries the
// BaseEndpoints store first, falling back to the discovered-endpoints store
// so the "Check Now" button works for both sections of the dashboard.
func CheckEndpointHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Path[len("/check/"):]

	runCheck := func(endpoint *collector.Endpoint) {
		monitor.CheckAPI(endpoint, nil) // nil options will trigger both calls
	}

	if collector.UpdateEndpointByName(name, runCheck) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if collector.UpdateDiscoveredEndpointByName(name, runCheck) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Error(w, "Endpoint not found", http.StatusNotFound)
}

// DashboardHandler handles the main dashboard page. Renders two tables with
// identical layout: the BaseEndpoints results (driven by the hourly loop) and
// the discovered test set results (driven by the daily discovery loop).
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, dashboardHeader)
	fmt.Fprintf(w, `<div style="margin-bottom:12px;font-size:0.95em;"><a href="/pools" style="color:#1565c0;text-decoration:none;">Discovered pools &rarr;</a> <span style="color:#666;">(last refresh: %s)</span></div>`,
		formatTimeAgo(discovery.LastSuccessAt()))

	renderEndpointsTable(w, "endpoints-table", collector.GetEndpointsCopy())

	fmt.Fprintf(w, `<h2 style="margin-top:32px;">Discovered test set (daily)</h2>`)
	discovered := collector.GetDiscoveredEndpointsCopy()
	if len(discovered) == 0 {
		fmt.Fprint(w, `<div style="padding:16px;background:#fff8e1;border:1px solid #ffe082;border-radius:4px;color:#5d4037;margin-bottom:12px;">No discovered test rows yet; first daily run pending.</div>`)
	} else {
		renderEndpointsTable(w, "discovered-table", discovered)
	}

	fmt.Fprintln(w, "</body></html>")
}

// renderEndpointsTable renders one full <table>…</table> for a slice of
// endpoints grouped by BaseName. Both the BaseEndpoints and discovered
// sections share this implementation so the layout, sorting, and per-row
// highlighting logic can't drift.
func renderEndpointsTable(w http.ResponseWriter, tableID string, endpoints []collector.Endpoint) {
	groups := make(map[string][]collector.Endpoint)
	for _, e := range endpoints {
		groups[e.BaseName] = append(groups[e.BaseName], e)
	}
	baseNames := make([]string, 0, len(groups))
	for name := range groups {
		baseNames = append(baseNames, name)
	}
	sort.Strings(baseNames)

	fmt.Fprintf(w, `<table id="%s" border="1"><thead><tr>`, tableID)
	fmt.Fprint(w, `<th class='name-column'>Name</th><th>Status</th><th>Message</th>`)
	fmt.Fprintf(w, `<th class='sortable-header' onclick="sortTable('%s', 3)">Balancer Price<span class='sort-arrow' id='%s-arrow-3'>&#8597;</span></th>`, tableID, tableID)
	fmt.Fprintf(w, `<th class='sortable-header' onclick="sortTable('%s', 4)">Market Price<span class='sort-arrow' id='%s-arrow-4'>&#8597;</span></th>`, tableID, tableID)
	fmt.Fprint(w, `<th>Last Checked</th><th>Actions</th></tr></thead><tbody>`)

	for _, baseName := range baseNames {
		groupEndpoints := groups[baseName]
		networkName := getNetworkName(groupEndpoints[0].Network)
		poolLink := fmt.Sprintf("https://balancer.fi/pools/%s/v3/%s", networkName, groupEndpoints[0].ExpectedPool)
		fmt.Fprintf(w, "<tr class='base-name-row'><td colspan='7'>%s<br><span style='font-weight: normal; font-size: 0.9em; margin-top: 10px; display: inline-block;'>In: %s<br>Out: %s<br>Pool: <a href='%s' target='_blank'>%s</a><br>Amount: %s</span></td></tr>",
			baseName,
			groupEndpoints[0].TokenIn,
			groupEndpoints[0].TokenOut,
			poolLink,
			groupEndpoints[0].ExpectedPool,
			groupEndpoints[0].SwapAmount)

		sorted := make([]collector.Endpoint, len(groupEndpoints))
		copy(sorted, groupEndpoints)
		sort.Slice(sorted, func(i, j int) bool {
			return parseBigInt(sorted[i].ReturnAmount).Cmp(parseBigInt(sorted[j].ReturnAmount)) > 0
		})

		for _, endpoint := range sorted {
			renderSolverRow(w, endpoint)
		}
	}

	fmt.Fprint(w, `</tbody></table>`)
}

// renderSolverRow writes one solver-level <tr> with status, return amount,
// market/on-chain price, deviation highlighting, and the Check Now button.
func renderSolverRow(w http.ResponseWriter, endpoint collector.Endpoint) {
	statusClass := "status-unknown"
	switch endpoint.LastStatus {
	case "up":
		statusClass = "status-up"
	case "down":
		statusClass = "status-down"
	case "disabled":
		statusClass = "status-disabled"
	}

	returnAmountDisplay := "N/A"
	if endpoint.ReturnAmount != "" {
		returnAmountDisplay = endpoint.ReturnAmount
	}

	marketPriceDisplay := "N/A"
	priceLabel := ""
	returnAmountClass := ""
	marketPriceClass := ""

	if endpoint.RouteSolver == "balancer_sor" {
		switch {
		case endpoint.OnChainPrice != "":
			marketPriceDisplay = endpoint.OnChainPrice
			priceLabel = " (on-chain)"
		case endpoint.OnChainQueryError != "":
			marketPriceDisplay = "Query Failed"
			priceLabel = " (error)"
			marketPriceClass = " class='price-error'"
		default:
			marketPriceDisplay = "N/A"
			priceLabel = " (on-chain)"
		}
	} else if endpoint.MarketPrice != "" {
		marketPriceDisplay = endpoint.MarketPrice
	}

	returnAmountBig := parseBigInt(endpoint.ReturnAmount)
	var priceBig *big.Int
	if endpoint.RouteSolver == "balancer_sor" && endpoint.OnChainPrice != "" && endpoint.OnChainQueryError == "" {
		priceBig = parseBigInt(endpoint.OnChainPrice)
	} else {
		priceBig = parseBigInt(endpoint.MarketPrice)
	}

	if endpoint.RouteSolver == "balancer_sor" && endpoint.OnChainPrice != "" {
		if returnAmountBig.Sign() > 0 && priceBig.Sign() > 0 {
			diff := new(big.Int).Abs(new(big.Int).Sub(returnAmountBig, priceBig))
			diffFloat := new(big.Float).SetInt(diff)
			priceFloat := new(big.Float).SetInt(priceBig)
			if priceFloat.Sign() > 0 {
				percent := new(big.Float).Quo(diffFloat, priceFloat)
				percent.Mul(percent, big.NewFloat(100))
				pctVal, _ := percent.Float64()
				if pctVal > 0.5 {
					returnAmountClass = " class='price-warning'"
					marketPriceClass = " class='price-warning'"
				} else if returnAmountBig.Cmp(priceBig) > 0 {
					returnAmountClass = " class='highest-value'"
				} else if priceBig.Cmp(returnAmountBig) > 0 {
					marketPriceClass = " class='highest-value'"
				}
			}
		}
	} else if returnAmountBig.Sign() > 0 || priceBig.Sign() > 0 {
		if returnAmountBig.Cmp(priceBig) > 0 {
			returnAmountClass = " class='highest-value'"
		} else if priceBig.Cmp(returnAmountBig) > 0 {
			marketPriceClass = " class='highest-value'"
		}
	}

	fmt.Fprintf(w, "<tr class='solver-row'><td class='name-column'>%s</td><td class='%s'>%s</td><td>%s</td><td%s>%s</td><td%s>%s%s</td><td>%s</td><td><button class='check-button' onclick='checkEndpoint(\"%s\")'>Check Now</button></td></tr>",
		endpoint.SolverName,
		statusClass,
		endpoint.LastStatus,
		endpoint.Message,
		returnAmountClass,
		returnAmountDisplay,
		marketPriceClass,
		marketPriceDisplay,
		priceLabel,
		formatTimeAgo(endpoint.LastChecked),
		endpoint.Name)
}

// parseBigInt parses a decimal string into a *big.Int. Empty or "N/A" map to
// zero so sorting / comparison stay well-defined.
func parseBigInt(s string) *big.Int {
	v := new(big.Int)
	if s == "" || s == "N/A" {
		return v
	}
	if _, ok := v.SetString(s, 10); !ok {
		return new(big.Int)
	}
	return v
}

// dashboardHeader is the static <html><head>...<body><h1> prefix. Extracted
// so the body code stays compact.
const dashboardHeader = `<html><head>
		<style>
			.status-up { background-color: #90EE90; }
			.status-down { background-color: #FFB6C1; }
			.status-unknown { background-color: #FFA500; }
			.status-disabled { background-color: #D3D3D3; }
			.highest-value { background-color: #90EE90; font-weight: bold; }
			.price-warning { background-color: #FFB347; font-weight: bold; }
			.price-error { background-color: #FF6B6B; color: white; font-weight: bold; }
			table { border-collapse: collapse; width: 100%; margin-bottom: 24px; }
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
			.check-button:hover { background-color: #45a049; }
			.base-name-row { background-color: #e6f3ff; font-weight: bold; }
			.solver-row { background-color: #f9f9f9; }
			.sortable-header { cursor: pointer; user-select: none; position: relative; padding-right: 20px; }
			.sortable-header:hover { background-color: #e0e0e0; }
			.sort-arrow { position: absolute; right: 5px; top: 50%; transform: translateY(-50%); font-size: 12px; color: #666; }
			.sort-arrow.active { color: #000; font-weight: bold; }
		</style>
		<script>
			const sortState = {};

			function checkEndpoint(name) {
				fetch('/check/' + name, { method: 'POST' }).then(() => window.location.reload());
			}

			function sortTable(tableId, column) {
				const table = document.getElementById(tableId);
				if (!table) return;
				const tbody = table.querySelector('tbody');
				const allRows = Array.from(tbody.querySelectorAll('tr'));

				if (!sortState[tableId]) sortState[tableId] = { column: 4, direction: 'desc' };
				const state = sortState[tableId];

				if (state.column === column) {
					state.direction = state.direction === 'asc' ? 'desc' : 'asc';
				} else {
					state.column = column;
					state.direction = 'desc';
				}

				table.querySelectorAll('.sort-arrow').forEach(arrow => {
					arrow.classList.remove('active');
					arrow.textContent = '\u2195';
				});
				const activeArrow = document.getElementById(tableId + '-arrow-' + column);
				if (activeArrow) {
					activeArrow.classList.add('active');
					activeArrow.textContent = state.direction === 'asc' ? '\u2191' : '\u2193';
				}

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

				groups.forEach(group => {
					group.solvers.sort((a, b) => {
						const aVal = a.cells[column].textContent.trim();
						const bVal = b.cells[column].textContent.trim();
						if (aVal === 'N/A' && bVal === 'N/A') return 0;
						if (aVal === 'N/A') return 1;
						if (bVal === 'N/A') return -1;
						let aNum, bNum;
						try { aNum = BigInt(aVal); bNum = BigInt(bVal); }
						catch (e) { aNum = BigInt(0); bNum = BigInt(0); }
						if (state.direction === 'asc') return aNum < bNum ? -1 : aNum > bNum ? 1 : 0;
						return aNum > bNum ? -1 : aNum < bNum ? 1 : 0;
					});
				});

				tbody.innerHTML = '';
				groups.forEach(group => {
					tbody.appendChild(group.header);
					group.solvers.forEach(solver => tbody.appendChild(solver));
				});
			}

			document.addEventListener('DOMContentLoaded', function() {
				setTimeout(function() {
					document.querySelectorAll('table').forEach(t => {
						if (!t.id) return;
						sortState[t.id] = { column: 4, direction: 'asc' };
						sortTable(t.id, 4);
					});
				}, 100);
			});
		</script>
	</head><body><h1>API Monitor</h1>`
