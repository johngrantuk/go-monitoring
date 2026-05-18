package discovery

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
	"go-monitoring/internal/monitor"
	"go-monitoring/notifications"
)

// testSetRunner is invoked after each discovery refresh once the discovered
// endpoint store has been populated. Registered from main.go via
// SetTestSetRunner so discovery itself doesn't need to import a runner
// implementation. Nil-safe: missing runner just skips the run.
var (
	testSetRunnerMu sync.Mutex
	testSetRunner   func()
)

// SetTestSetRunner registers (or clears) the callback invoked after each
// discovery refresh has populated the discovered endpoint store.
func SetTestSetRunner(fn func()) {
	testSetRunnerMu.Lock()
	defer testSetRunnerMu.Unlock()
	testSetRunner = fn
}

func getTestSetRunner() func() {
	testSetRunnerMu.Lock()
	defer testSetRunnerMu.Unlock()
	return testSetRunner
}

// minTotalLiquidityUSD is the absolute floor below which a pool is skipped
// entirely, regardless of its category tags. See docs/discovery.md.
const minTotalLiquidityUSD = 10_000.0

// stableSurgeHookType is the hook.type enum value the Balancer API returns for
// pools using the StableSurge hook.
const stableSurgeHookType = "STABLE_SURGE"

// akronHookType is excluded from discovery entirely.
const akronHookType = "AKRON"

// excludedPoolTypes are Balancer API pool.type values dropped entirely in
// discovery (not surfaced on /pools or in the daily test set).
var excludedPoolTypes = map[string]struct{}{
	"FIXED_LBP":               {},
	"LIQUIDITY_BOOTSTRAPPING": {},
}

// excludedPoolAddresses are pool addresses dropped entirely in discovery
// (lowercase hex). Add entries here to hide specific pools from /pools and
// the daily test set.
var excludedPoolAddresses = map[string]struct{}{
	"0x83470106402ed0bc83f91bb13266d35bdb23f1b9": {},
}

// Run executes discovery immediately, then re-runs on the configured cadence.
// Designed to be invoked as `go discovery.Run(...)` from main. Fire-and-forget:
// the caller does not block on the first run.
//
// Each runOnce is wrapped in a deferred recover so a panic anywhere in the
// fetch / process / test-set / runner chain logs the failure, emails an
// alert, and lets the ticker fire again. Without this guard a single panic
// would kill the goroutine for the rest of the process lifetime and all
// future discovery refreshes would silently stop.
func Run(intervalHours int) {
	safeRunOnce()
	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		safeRunOnce()
	}
}

// safeRunOnce wraps runOnce with panic recovery so the discovery goroutine
// survives even if a downstream provider handler or test-set runner blows up.
// On panic we log a coloured banner with the full stack and send an email so
// operators notice; the next ticker fire still triggers a fresh attempt.
func safeRunOnce() {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			fmt.Printf("%s[DISCOVERY PANIC]%s recovered: %v\n%s\n",
				config.ColorRed, config.ColorReset, r, stack)
			notifications.SendEmail(fmt.Sprintf("Discovery goroutine panicked: %v", r))
		}
	}()
	runOnce()
}

// runOnce executes one discovery cycle across all enabled networks. Each
// network is fetched and processed independently so a single failure does not
// poison the snapshots for other networks. After the per-network loop the
// daily test set is rebuilt from the current cross-network snapshot and
// pushed into the collector's discovered-endpoints store; the registered
// runner (if any) then drives provider checks against it.
func runOnce() {
	fmt.Printf("%s[DISCOVERY]%s starting run\n", config.ColorBlue, config.ColorReset)

	for _, cfg := range config.DiscoveryConfigs {
		if !cfg.Enabled {
			continue
		}
		chainEnum := config.BalancerAPIChain(cfg.Network)
		if chainEnum == "" {
			fmt.Printf("%s[DISCOVERY]%s skipping network %s: unsupported by Balancer API\n",
				config.ColorYellow, config.ColorReset, cfg.Network)
			continue
		}

		fmt.Printf("%s[DISCOVERY]%s fetching network %s (%s)\n",
			config.ColorBlue, config.ColorReset, cfg.Network, chainEnum)

		raw, err := fetchPoolsWithRetry(chainEnum)
		if err != nil {
			// Differentiate "we had a snapshot from yesterday — keeping it" from
			// "first run failed — nothing to keep". Otherwise the operator
			// can't tell whether /pools should have data for this network.
			if last := networkLastSuccessAt(cfg.Network); !last.IsZero() {
				fmt.Printf("%s[DISCOVERY ERROR]%s network %s: %v (keeping previous snapshot from %s ago)\n",
					config.ColorRed, config.ColorReset, cfg.Network, err, time.Since(last).Round(time.Second))
			} else {
				fmt.Printf("%s[DISCOVERY ERROR]%s network %s: %v (no previous snapshot; will retry on next tick)\n",
					config.ColorRed, config.ColorReset, cfg.Network, err)
			}
			continue
		}

		pools := processNetwork(cfg, raw)
		setNetwork(cfg.Network, pools, time.Now())

		fmt.Printf("%s[DISCOVERY RESULT]%s network %s: %d pools (from %d raw)\n",
			config.ColorGreen, config.ColorReset, cfg.Network, len(pools), len(raw))
	}

	rebuildTestSet()

	if runner := getTestSetRunner(); runner != nil {
		runner()
	}
}

// rebuildTestSet recomputes the daily test set from the current snapshot,
// expands rows across enabled solvers, and replaces the discovered-endpoints
// store (carrying over prior result fields for keys that survive). Runs every
// discovery tick regardless of per-network success.
func rebuildTestSet() {
	snapshot := Get()

	tradePctByNetwork := map[string]float64{}
	for _, cfg := range config.DiscoveryConfigs {
		if !cfg.Enabled {
			continue
		}
		tradePctByNetwork[cfg.Network] = cfg.TradePercent
	}

	rows, poolKeys := Build(snapshot, config.GetDiscoveryTestPoolsPerGroup(), tradePctByNetwork)

	inputs := make([]monitor.ExpandInput, 0, len(rows))
	for _, r := range rows {
		baseName := formatDiscoveredBaseName(r)
		inputs = append(inputs, monitor.ExpandInput{
			BaseName:         baseName,
			Network:          r.Network,
			TokenIn:          r.TokenIn,
			TokenOut:         r.TokenOut,
			TokenInDecimals:  r.TokenInDec,
			TokenOutDecimals: r.TokenOutDec,
			SwapAmount:       r.SwapAmountRaw,
			ExpectedPool:     r.PoolAddress,
			ExpectedNoHops:   1, // boosted + non-boosted alike, per decision
			PoolType:         r.PoolType,
			HookType:         r.HookType,
			Variant:          r.Variant,
		})
	}

	eps := monitor.ExpandForSolvers(inputs)
	collector.SetDiscoveredEndpoints(eps, poolKeys)

	fmt.Printf("%s[DISCOVERY TESTSET]%s %d rows × solvers = %d endpoints (%d pools selected)\n",
		config.ColorBlue, config.ColorReset, len(rows), len(eps), len(poolKeys))
}

// shortAddr returns the first 8 hex chars (10 incl. "0x") of an address for
// use in deterministic, address-based BaseName keys. Inputs that are too
// short fall back to the original string.
func shortAddr(addr string) string {
	a := strings.ToLower(addr)
	if len(a) >= 10 {
		return a[:10]
	}
	return a
}

// processNetwork applies skip conditions and category tagging to the raw pools
// for a single network, returning the list to be stored.
func processNetwork(cfg config.DiscoveryConfig, raw []rawPool) []Pool {
	// Stage 1: parse + skip conditions. Pools with unparseable numerics or
	// failing the skip filter are dropped entirely.
	type parsed struct {
		pool       Pool
		totalUSD   float64
		swapFee    float64
		volume24h  float64
		tokenAddrs []string // lowercased registered token addresses, for pair-freq
	}
	survivors := make([]parsed, 0, len(raw))

	for _, r := range raw {
		if _, excluded := excludedPoolAddresses[strings.ToLower(r.Address)]; excluded {
			continue
		}

		if _, excluded := excludedPoolTypes[r.Type]; excluded {
			continue
		}

		if r.Hook != nil && r.Hook.Type == akronHookType {
			continue
		}

		if r.Type == "WEIGHTED" && r.Hook != nil && r.Hook.Type == stableSurgeHookType {
			continue
		}

		if r.DynamicData.IsPaused || r.DynamicData.IsInRecoveryMode {
			continue
		}

		tvl, err := strconv.ParseFloat(r.DynamicData.TotalLiquidity, 64)
		if err != nil {
			fmt.Printf("%s[DISCOVERY]%s skipping pool %s: parse totalLiquidity %q: %v\n",
				config.ColorYellow, config.ColorReset, r.Address, r.DynamicData.TotalLiquidity, err)
			continue
		}
		if tvl < minTotalLiquidityUSD {
			continue
		}

		fee, err := strconv.ParseFloat(r.DynamicData.SwapFee, 64)
		if err != nil {
			fmt.Printf("%s[DISCOVERY]%s skipping pool %s: parse swapFee %q: %v\n",
				config.ColorYellow, config.ColorReset, r.Address, r.DynamicData.SwapFee, err)
			continue
		}

		vol, err := strconv.ParseFloat(r.DynamicData.Volume24h, 64)
		if err != nil {
			fmt.Printf("%s[DISCOVERY]%s skipping pool %s: parse volume24h %q: %v\n",
				config.ColorYellow, config.ColorReset, r.Address, r.DynamicData.Volume24h, err)
			continue
		}

		tokens := make([]PoolToken, 0, len(r.PoolTokens))
		tokenAddrs := make([]string, 0, len(r.PoolTokens))
		for _, t := range r.PoolTokens {
			// balanceUSD is permitted to be missing/unparseable; we treat it
			// as 0 so the pool still surfaces on /pools, but rows that need a
			// USD price will be dropped downstream in computeSwapAmountRaw.
			balUSD, _ := strconv.ParseFloat(t.BalanceUSD, 64)
			tok := PoolToken{
				Address:    t.Address,
				Symbol:     t.Symbol,
				Decimals:   t.Decimals,
				Balance:    t.Balance,
				BalanceUSD: balUSD,
			}
			if t.UnderlyingToken != nil {
				tok.Underlying = &UnderlyingToken{
					Address:  t.UnderlyingToken.Address,
					Symbol:   t.UnderlyingToken.Symbol,
					Decimals: t.UnderlyingToken.Decimals,
				}
			}
			tokens = append(tokens, tok)
			tokenAddrs = append(tokenAddrs, strings.ToLower(t.Address))
		}

		hookType := ""
		var surgeThreshold, surgeImbalance float64
		var surging bool
		if r.Hook != nil {
			hookType = r.Hook.Type
			if hookType == stableSurgeHookType && r.Hook.Params != nil {
				thr, err := strconv.ParseFloat(r.Hook.Params.SurgeThresholdPercentage, 64)
				if err == nil && thr > 0 {
					surgeThreshold = thr
					surgeImbalance = stableSurgeImbalance(tokens)
					surging = surgeImbalance >= surgeThreshold-stableSurgeImbalanceBuffer
				}
			}
		}

		pool := Pool{
			Address:           r.Address,
			Network:           cfg.Network,
			Type:              r.Type,
			HookType:          hookType,
			Name:              r.Name,
			Symbol:            r.Symbol,
			TotalLiquidityUSD: tvl,
			SwapFeeFraction:   fee,
			Volume24hUSD:      vol,
			Tokens:            tokens,
			SurgeThreshold:    surgeThreshold,
			SurgeImbalance:    surgeImbalance,
			Surging:           surging,
		}

		survivors = append(survivors, parsed{
			pool:       pool,
			totalUSD:   tvl,
			swapFee:    fee,
			volume24h:  vol,
			tokenAddrs: tokenAddrs,
		})
	}

	// Stage 2: build pair-frequency map. Key is "addrLow|addrHigh" of registered
	// token pairs across all surviving pools for this network. Underlyings are
	// intentionally NOT normalised here.
	pairCount := map[string]int{}
	for _, p := range survivors {
		for _, key := range pairKeys(p.tokenAddrs) {
			pairCount[key]++
		}
	}

	// Stage 3: tag each pool with categories.
	out := make([]Pool, 0, len(survivors))
	for _, p := range survivors {
		categories := []string{}

		// `unique`: at least one of this pool's registered pairs appears in
		// exactly one pool across the network.
		for _, key := range pairKeys(p.tokenAddrs) {
			if pairCount[key] == 1 {
				categories = append(categories, CategoryUnique)
				break
			}
		}

		// `highTVL`: TVL strictly above threshold.
		if p.totalUSD > cfg.TVLThresholdUSD {
			categories = append(categories, CategoryHighTVL)
		}

		p.pool.Categories = categories
		out = append(out, p.pool)
	}

	return out
}

// stableSurgeImbalance mirrors StableSurgeMedianMath.calculateImbalance from the
// Balancer V3 StableSurge hook, using balanceUSD as a USD proxy for scaled18
// balances:
//
//	median     = findMedian(balanceUSD[])
//	totalDiff  = Σ |balanceUSD[i] - median|
//	imbalance  = totalDiff / Σ balanceUSD[i]
//
// Returns a value in [0, 1] where 0 means perfectly balanced. Pools with <2
// tokens, zero TVL, or any missing balanceUSD return 0 (we can't reason about
// surge state, so the caller treats it as "not surging").
func stableSurgeImbalance(tokens []PoolToken) float64 {
	n := len(tokens)
	if n < 2 {
		return 0
	}
	balances := make([]float64, n)
	var sum float64
	for i, t := range tokens {
		if t.BalanceUSD <= 0 {
			return 0
		}
		balances[i] = t.BalanceUSD
		sum += t.BalanceUSD
	}
	if sum <= 0 {
		return 0
	}
	median := medianUSD(balances)
	var totalDiff float64
	for _, b := range balances {
		d := b - median
		if d < 0 {
			d = -d
		}
		totalDiff += d
	}
	return totalDiff / sum
}

// medianUSD returns the median of USD balances, matching
// StableSurgeMedianMath.findMedian (average of two middle values when n is even).
func medianUSD(balances []float64) float64 {
	sorted := append([]float64(nil), balances...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// pairKeys returns the deterministic keys for every C(N,2) combination of the
// given lowercased token addresses. Each key is "lower|higher" so direction is
// normalised. Pools with <2 tokens produce no pairs.
func pairKeys(addrs []string) []string {
	n := len(addrs)
	if n < 2 {
		return nil
	}
	keys := make([]string, 0, n*(n-1)/2)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			a, b := addrs[i], addrs[j]
			if a > b {
				a, b = b, a
			}
			keys = append(keys, a+"|"+b)
		}
	}
	return keys
}
