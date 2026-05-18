package handlers

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"sort"
	"strings"

	"go-monitoring/internal/collector"
	"go-monitoring/internal/discovery"
)

// PoolsHandler renders the discovered Balancer V3 pool list at /pools.
func PoolsHandler(w http.ResponseWriter, r *http.Request) {
	pools := discovery.Get()
	lastSuccess := discovery.LastSuccessAt()

	fmt.Fprint(w, `<html><head>
<title>Discovered Pools</title>
<style>
	body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; margin: 20px; }
	h1 { margin-bottom: 4px; }
	.subhead { color: #555; margin-bottom: 16px; font-size: 0.95em; }
	.subhead a { color: #1565c0; text-decoration: none; }
	.subhead a:hover { text-decoration: underline; }
	.filters { margin: 12px 0 16px 0; display: flex; gap: 16px; flex-wrap: wrap; align-items: center; }
	.filters label { font-size: 0.9em; color: #333; display: flex; flex-direction: column; gap: 2px; }
	.filters select { padding: 4px 6px; font-size: 0.95em; }
	table { border-collapse: collapse; width: 100%; font-size: 0.93em; }
	th, td { padding: 6px 8px; text-align: left; border-bottom: 1px solid #eee; vertical-align: top; }
	thead th { background: #f5f5f5; border-bottom: 2px solid #ddd; white-space: nowrap; }
	tbody tr:hover { background: #fafafa; }
	.addr { font-family: ui-monospace, SFMono-Regular, monospace; }
	.addr a { color: #1565c0; text-decoration: none; }
	.addr a:hover { text-decoration: underline; }
	.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
	.tokens { font-family: ui-monospace, SFMono-Regular, monospace; font-size: 0.9em; }
	.badge { display: inline-block; padding: 1px 6px; border-radius: 10px; font-size: 0.8em; margin-right: 4px; }
	.badge-unique { background: #e3f2fd; color: #0d47a1; }
	.badge-highTVL { background: #e8f5e9; color: #1b5e20; }
	.badge-test-yes { background: #e8f5e9; color: #1b5e20; }
	.badge-test-no { background: #f5f5f5; color: #757575; }
	.badge-surging { background: #fdecea; color: #b71c1c; margin-left: 6px; }
	.sortable-header { cursor: pointer; user-select: none; position: relative; padding-right: 18px; }
	.sortable-header:hover { background: #ececec; }
	.sort-arrow { position: absolute; right: 4px; top: 50%; transform: translateY(-50%); font-size: 11px; color: #999; }
	.sort-arrow.active { color: #000; font-weight: bold; }
	.placeholder { padding: 24px; background: #fff8e1; border: 1px solid #ffe082; border-radius: 4px; color: #5d4037; }
</style>
</head><body>`)

	fmt.Fprint(w, `<h1>Discovered Pools</h1>`)
	fmt.Fprintf(w, `<div class="subhead"><a href="/">&larr; Back to monitor</a> &middot; Last refreshed: %s</div>`,
		html.EscapeString(formatTimeAgo(lastSuccess)))

	if lastSuccess.IsZero() {
		fmt.Fprint(w, `<div class="placeholder">Discovery has not run yet. First refresh in progress.</div>`)
		fmt.Fprint(w, `</body></html>`)
		return
	}

	renderFilters(w, pools)
	renderTable(w, pools)
	renderScripts(w)

	fmt.Fprint(w, `</body></html>`)
}

// renderFilters writes the filter dropdowns above the table. Options are
// derived from the rendered pool set.
func renderFilters(w http.ResponseWriter, pools []discovery.Pool) {
	networks := distinctSorted(pools, func(p discovery.Pool) string { return getNetworkName(p.Network) })
	poolTypes := distinctSorted(pools, func(p discovery.Pool) string { return p.Type })

	fmt.Fprint(w, `<div class="filters">`)

	fmt.Fprint(w, `<label>Network<select id="filter-network" onchange="applyFilters()"><option value="">All</option>`)
	for _, n := range networks {
		fmt.Fprintf(w, `<option value="%s">%s</option>`, html.EscapeString(n), html.EscapeString(n))
	}
	fmt.Fprint(w, `</select></label>`)

	fmt.Fprint(w, `<label>Category<select id="filter-category" onchange="applyFilters()">`)
	fmt.Fprint(w, `<option value="">All</option>`)
	fmt.Fprint(w, `<option value="unique">unique (any)</option>`)
	fmt.Fprint(w, `<option value="highTVL">highTVL (any)</option>`)
	fmt.Fprint(w, `<option value="both">both</option>`)
	fmt.Fprint(w, `<option value="untagged">untagged</option>`)
	fmt.Fprint(w, `</select></label>`)

	fmt.Fprint(w, `<label>Pool type<select id="filter-type" onchange="applyFilters()"><option value="">All</option>`)
	for _, t := range poolTypes {
		fmt.Fprintf(w, `<option value="%s">%s</option>`, html.EscapeString(t), html.EscapeString(t))
	}
	fmt.Fprint(w, `</select></label>`)

	fmt.Fprint(w, `<label>In test set<select id="filter-testset" onchange="applyFilters()">`)
	fmt.Fprint(w, `<option value="">All</option>`)
	fmt.Fprint(w, `<option value="yes">Yes</option>`)
	fmt.Fprint(w, `<option value="no">No</option>`)
	fmt.Fprint(w, `</select></label>`)

	fmt.Fprint(w, `<label>Surging<select id="filter-surging" onchange="applyFilters()">`)
	fmt.Fprint(w, `<option value="">All</option>`)
	fmt.Fprint(w, `<option value="yes">Yes</option>`)
	fmt.Fprint(w, `<option value="no">No</option>`)
	fmt.Fprint(w, `</select></label>`)

	fmt.Fprintf(w, `<span style="color:#666;font-size:0.9em;">%d pools</span>`, len(pools))
	fmt.Fprint(w, `</div>`)
}

// distinctSorted extracts unique non-empty key values from pools and returns
// them sorted alphabetically.
func distinctSorted(pools []discovery.Pool, key func(discovery.Pool) string) []string {
	seen := map[string]struct{}{}
	for _, p := range pools {
		k := key(p)
		if k == "" {
			continue
		}
		seen[k] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// renderTable writes the pools table. Default sort order is TVL desc with
// address asc as a deterministic tiebreaker.
func renderTable(w http.ResponseWriter, pools []discovery.Pool) {
	sorted := make([]discovery.Pool, len(pools))
	copy(sorted, pools)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TotalLiquidityUSD != sorted[j].TotalLiquidityUSD {
			return sorted[i].TotalLiquidityUSD > sorted[j].TotalLiquidityUSD
		}
		return strings.ToLower(sorted[i].Address) < strings.ToLower(sorted[j].Address)
	})

	fmt.Fprint(w, `<table><thead><tr>`)
	fmt.Fprint(w, `<th>Pool address</th>`)
	fmt.Fprint(w, `<th>Symbol</th>`)
	fmt.Fprint(w, `<th class="sortable-header" onclick="sortTable(2,'string')">Pool type<span class="sort-arrow" id="arrow-2">&#8597;</span></th>`)
	fmt.Fprint(w, `<th>Hook type</th>`)
	fmt.Fprint(w, `<th>Network</th>`)
	fmt.Fprint(w, `<th class="sortable-header" onclick="sortTable(5,'string')">Categories<span class="sort-arrow" id="arrow-5">&#8597;</span></th>`)
	fmt.Fprint(w, `<th class="sortable-header" onclick="sortTable(6,'number')">In test set<span class="sort-arrow" id="arrow-6">&#8597;</span></th>`)
	fmt.Fprint(w, `<th class="sortable-header num" onclick="sortTable(7,'number')">TVL USD<span class="sort-arrow active" id="arrow-7">&darr;</span></th>`)
	fmt.Fprint(w, `<th class="sortable-header num" onclick="sortTable(8,'number')">Volume 24h<span class="sort-arrow" id="arrow-8">&#8597;</span></th>`)
	fmt.Fprint(w, `<th class="num">Swap fee</th>`)
	fmt.Fprint(w, `<th>Tokens</th>`)
	fmt.Fprint(w, `</tr></thead><tbody>`)

	for _, p := range sorted {
		networkName := getNetworkName(p.Network)
		fullAddr := p.Address
		poolURL := fmt.Sprintf("https://balancer.fi/pools/%s/v3/%s", networkName, fullAddr)

		hookDisplay := p.HookType
		if hookDisplay == "" {
			hookDisplay = "—"
		}

		inTestSet := collector.IsPoolInTestSet(p.Network, p.Address)
		testSetSlot := "no"
		if inTestSet {
			testSetSlot = "yes"
		}

		surgingSlot := "no"
		if p.Surging {
			surgingSlot = "yes"
		}

		fmt.Fprintf(w,
			`<tr data-network="%s" data-type="%s" data-category="%s" data-testset="%s" data-surging="%s">`,
			html.EscapeString(networkName),
			html.EscapeString(p.Type),
			html.EscapeString(categorySlot(p.Categories)),
			testSetSlot,
			surgingSlot,
		)

		fmt.Fprintf(w,
			`<td class="addr"><a href="%s" target="_blank" title="%s">%s</a></td>`,
			html.EscapeString(poolURL),
			html.EscapeString(fullAddr),
			html.EscapeString(truncateAddress(fullAddr)),
		)

		fmt.Fprintf(w, `<td>%s</td>`, html.EscapeString(p.Symbol))
		fmt.Fprintf(w, `<td>%s</td>`, html.EscapeString(p.Type))
		fmt.Fprintf(w, `<td>%s%s</td>`, html.EscapeString(hookDisplay), renderSurgeBadge(p))
		fmt.Fprintf(w, `<td>%s</td>`, html.EscapeString(networkName))

		fmt.Fprintf(w, `<td data-sort="%s">%s</td>`,
			html.EscapeString(strings.Join(p.Categories, ",")),
			renderCategoryBadges(p.Categories),
		)

		fmt.Fprintf(w, `<td data-sort="%d">%s</td>`,
			boolAsInt(inTestSet), renderTestSetBadge(inTestSet))

		fmt.Fprintf(w, `<td class="num" data-sort="%.6f">%s</td>`,
			p.TotalLiquidityUSD, html.EscapeString(formatUSD(p.TotalLiquidityUSD)))
		fmt.Fprintf(w, `<td class="num" data-sort="%.6f">%s</td>`,
			p.Volume24hUSD, html.EscapeString(formatUSD(p.Volume24hUSD)))
		fmt.Fprintf(w, `<td class="num">%s</td>`,
			html.EscapeString(formatSwapFeePercent(p.SwapFeeFraction)))

		fmt.Fprintf(w, `<td class="tokens">%s</td>`, renderTokens(p.Tokens))

		fmt.Fprint(w, `</tr>`)
	}

	fmt.Fprint(w, `</tbody></table>`)
}

// categorySlot returns a stable slot string used by the client-side category
// filter: "both" / "unique" / "highTVL" / "untagged".
func categorySlot(cats []string) string {
	hasUnique := false
	hasHighTVL := false
	for _, c := range cats {
		switch c {
		case discovery.CategoryUnique:
			hasUnique = true
		case discovery.CategoryHighTVL:
			hasHighTVL = true
		}
	}
	switch {
	case hasUnique && hasHighTVL:
		return "both"
	case hasUnique:
		return "unique"
	case hasHighTVL:
		return "highTVL"
	default:
		return "untagged"
	}
}

// renderTestSetBadge renders the "Yes" / "No" pill for the In-test-set column.
func renderTestSetBadge(in bool) string {
	if in {
		return `<span class="badge badge-test-yes">Yes</span>`
	}
	return `<span class="badge badge-test-no">No</span>`
}

// renderSurgeBadge renders a red "surging" pill next to the Hook column when
// a StableSurge pool's current USD imbalance is at or near its configured
// trigger threshold. The tooltip surfaces the precise numbers so operators
// can sanity-check why the pool was excluded from the test set. Returns "" for
// non-surging or non-StableSurge pools.
func renderSurgeBadge(p discovery.Pool) string {
	if !p.Surging {
		return ""
	}
	tooltip := fmt.Sprintf("imbalance %.4f / threshold %.4f (skipped from test set)",
		p.SurgeImbalance, p.SurgeThreshold)
	return fmt.Sprintf(`<span class="badge badge-surging" title="%s">surging</span>`,
		html.EscapeString(tooltip))
}

// boolAsInt returns 1 for true, 0 for false. Used as the sort value for the
// In-test-set column so descending order puts test-set pools first.
func boolAsInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func renderCategoryBadges(cats []string) string {
	if len(cats) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range cats {
		class := "badge"
		switch c {
		case discovery.CategoryUnique:
			class = "badge badge-unique"
		case discovery.CategoryHighTVL:
			class = "badge badge-highTVL"
		}
		fmt.Fprintf(&b, `<span class="%s">%s</span>`, class, html.EscapeString(c))
	}
	return b.String()
}

// renderTokens produces the compact "waUSDC(USDC) / waUSDT(USDT)" list with a
// detailed `title=` hover. Underlyings appear in parens after the registered
// symbol when present.
func renderTokens(tokens []discovery.PoolToken) string {
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		label := t.Symbol
		if t.Underlying != nil {
			label = fmt.Sprintf("%s(%s)", t.Symbol, t.Underlying.Symbol)
		}
		parts = append(parts, html.EscapeString(label))
	}

	var tip strings.Builder
	for i, t := range tokens {
		if i > 0 {
			tip.WriteString("\n")
		}
		fmt.Fprintf(&tip, "%s  %s  decimals=%d  balance=%s", t.Symbol, t.Address, t.Decimals, t.Balance)
		if t.Underlying != nil {
			fmt.Fprintf(&tip, "\nunderlying: %s  %s  decimals=%d",
				t.Underlying.Symbol, t.Underlying.Address, t.Underlying.Decimals)
		}
	}
	return fmt.Sprintf(`<span title="%s">%s</span>`,
		html.EscapeString(tip.String()), strings.Join(parts, " / "))
}

// truncateAddress shortens a hex address to `0xabcd…wxyz`.
func truncateAddress(addr string) string {
	if len(addr) < 12 {
		return addr
	}
	return addr[:6] + "…" + addr[len(addr)-4:]
}

// formatUSD renders a dollar value in compact form: $20.3M, $1.2K, $523.
// Negative values (shouldn't happen) are rendered with a leading minus.
func formatUSD(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "—"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	switch {
	case v >= 1e9:
		return fmt.Sprintf("%s$%s", sign, trimTrailing(fmt.Sprintf("%.2f", v/1e9))+"B")
	case v >= 1e6:
		return fmt.Sprintf("%s$%s", sign, trimTrailing(fmt.Sprintf("%.2f", v/1e6))+"M")
	case v >= 1e3:
		return fmt.Sprintf("%s$%s", sign, trimTrailing(fmt.Sprintf("%.2f", v/1e3))+"K")
	default:
		return fmt.Sprintf("%s$%.0f", sign, v)
	}
}

// formatSwapFeePercent renders a fee fraction as a percentage with up to 4
// decimal places and trailing zeros trimmed. 0.0001 -> "0.01%"; 0.00002 -> "0.002%".
func formatSwapFeePercent(fraction float64) string {
	if math.IsNaN(fraction) || math.IsInf(fraction, 0) {
		return "—"
	}
	pct := fraction * 100
	s := fmt.Sprintf("%.4f", pct)
	return trimTrailing(s) + "%"
}

// trimTrailing removes trailing zeros and a trailing decimal point from a
// numeric string like "1.2300" -> "1.23", "1.0000" -> "1".
func trimTrailing(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// renderScripts emits the inline JS that powers client-side filter + sort.
func renderScripts(w http.ResponseWriter) {
	fmt.Fprint(w, `<script>
let currentSort = { column: 7, direction: 'desc', kind: 'number' };

function applyFilters() {
	const network = document.getElementById('filter-network').value;
	const category = document.getElementById('filter-category').value;
	const type = document.getElementById('filter-type').value;
	const testset = document.getElementById('filter-testset').value;
	const surging = document.getElementById('filter-surging').value;
	const rows = document.querySelectorAll('tbody tr');
	rows.forEach(row => {
		const matchNet = !network || row.dataset.network === network;
		const matchType = !type || row.dataset.type === type;
		let matchCat = true;
		if (category === 'unique') matchCat = row.dataset.category === 'unique' || row.dataset.category === 'both';
		else if (category === 'highTVL') matchCat = row.dataset.category === 'highTVL' || row.dataset.category === 'both';
		else if (category === 'both' || category === 'untagged') matchCat = row.dataset.category === category;
		const matchTest = !testset || row.dataset.testset === testset;
		const matchSurging = !surging || row.dataset.surging === surging;
		row.style.display = (matchNet && matchType && matchCat && matchTest && matchSurging) ? '' : 'none';
	});
}

function sortTable(column, kind) {
	const tbody = document.querySelector('tbody');
	const rows = Array.from(tbody.querySelectorAll('tr'));

	if (currentSort.column === column) {
		currentSort.direction = currentSort.direction === 'asc' ? 'desc' : 'asc';
	} else {
		currentSort.column = column;
		currentSort.direction = column === 7 ? 'desc' : 'asc';
		currentSort.kind = kind;
	}

	document.querySelectorAll('.sort-arrow').forEach(arrow => {
		arrow.classList.remove('active');
		arrow.innerHTML = '\u2195';
	});
	const activeArrow = document.getElementById('arrow-' + column);
	if (activeArrow) {
		activeArrow.classList.add('active');
		activeArrow.innerHTML = currentSort.direction === 'asc' ? '\u2191' : '\u2193';
	}

	rows.sort((a, b) => {
		const cellA = a.cells[column];
		const cellB = b.cells[column];
		let aVal, bVal;
		if (kind === 'number') {
			aVal = parseFloat(cellA.dataset.sort || cellA.textContent) || 0;
			bVal = parseFloat(cellB.dataset.sort || cellB.textContent) || 0;
		} else {
			aVal = (cellA.dataset.sort || cellA.textContent).toLowerCase();
			bVal = (cellB.dataset.sort || cellB.textContent).toLowerCase();
		}
		let cmp;
		if (aVal < bVal) cmp = -1;
		else if (aVal > bVal) cmp = 1;
		else {
			const addrA = a.cells[0].textContent.toLowerCase();
			const addrB = b.cells[0].textContent.toLowerCase();
			cmp = addrA < addrB ? -1 : addrA > addrB ? 1 : 0;
		}
		return currentSort.direction === 'asc' ? cmp : -cmp;
	});

	tbody.innerHTML = '';
	rows.forEach(row => tbody.appendChild(row));
}
</script>`)
}
