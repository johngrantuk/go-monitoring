package discovery

import (
	"sort"
	"sync"
	"time"
)

// Category tag constants.
const (
	CategoryUnique  = "unique"
	CategoryHighTVL = "highTVL"
)

// stableSurgeImbalanceBuffer is the headroom (in absolute imbalance units, 0..1)
// applied below a StableSurge hook's surgeThresholdPercentage. A pool is treated
// as Surging — and dropped from the daily test set — when its current USD
// imbalance >= threshold - stableSurgeImbalanceBuffer. The buffer guards
// against pools sitting just below the trigger, where any worsening swap would
// immediately surge.
const stableSurgeImbalanceBuffer = 0.10

// Pool represents a Balancer V3 pool that survived discovery's skip filter.
type Pool struct {
	Address           string
	Network           string   // numeric chain id, e.g. "1"
	Type              string   // raw enum string, e.g. "STABLE", "COMPOSABLE_STABLE", "GYROE"
	HookType          string   // empty string when pool has no hook
	Name              string   // pool name from API
	Symbol            string   // LP token symbol from API
	Categories        []string // any of CategoryUnique, CategoryHighTVL; may be empty
	TotalLiquidityUSD float64
	SwapFeeFraction   float64 // as returned, e.g. 0.0001 means 0.01%
	Volume24hUSD      float64
	Tokens            []PoolToken

	// StableSurge hook state. All three are zero for non-StableSurge pools.
	SurgeThreshold float64 // surgeThresholdPercentage from hook.params (0..1)
	SurgeImbalance float64 // StableSurge totalImbalance proxy from USD balances (0..1)
	Surging        bool    // true when SurgeImbalance >= SurgeThreshold - stableSurgeImbalanceBuffer
}

// PoolToken describes a registered token in a pool.
type PoolToken struct {
	Address    string
	Symbol     string
	Decimals   int
	Balance    string  // human-readable decimal, as returned by the API
	BalanceUSD float64 // USD value of the token's pool balance, as returned by the API
	Underlying *UnderlyingToken
}

// UnderlyingToken is the wrapped (ERC4626 underlying) token of a registered
// token, when present.
type UnderlyingToken struct {
	Address  string
	Symbol   string
	Decimals int
}

// networkSnapshot holds the pools and last-success timestamp for a single
// network. Failed runs leave the previous snapshot untouched.
type networkSnapshot struct {
	pools         []Pool
	lastSuccessAt time.Time
}

var (
	stateMu sync.RWMutex
	state   = map[string]networkSnapshot{}
)

// Get returns a flat slice of all pools across all networks, sorted by network
// then address for stable iteration. The returned slice is a copy and safe to
// mutate.
func Get() []Pool {
	stateMu.RLock()
	defer stateMu.RUnlock()

	var total int
	for _, snap := range state {
		total += len(snap.pools)
	}

	out := make([]Pool, 0, total)
	networks := make([]string, 0, len(state))
	for net := range state {
		networks = append(networks, net)
	}
	sort.Strings(networks)
	for _, net := range networks {
		out = append(out, state[net].pools...)
	}
	return out
}

// LastSuccessAt returns the oldest per-network last-success timestamp so the
// freshness indicator honestly reflects the most stale network. Returns the
// zero time if discovery has not yet completed successfully for any network.
func LastSuccessAt() time.Time {
	stateMu.RLock()
	defer stateMu.RUnlock()

	if len(state) == 0 {
		return time.Time{}
	}
	var oldest time.Time
	first := true
	for _, snap := range state {
		if snap.lastSuccessAt.IsZero() {
			continue
		}
		if first || snap.lastSuccessAt.Before(oldest) {
			oldest = snap.lastSuccessAt
			first = false
		}
	}
	return oldest
}

// setNetwork atomically replaces the pools and last-success timestamp for a
// single network. Other networks' snapshots are untouched.
func setNetwork(network string, pools []Pool, at time.Time) {
	stateMu.Lock()
	defer stateMu.Unlock()
	state[network] = networkSnapshot{pools: pools, lastSuccessAt: at}
}

// networkLastSuccessAt returns the last successful refresh timestamp for a
// single network, or the zero time if that network has never produced a
// snapshot. Used by runOnce to distinguish "kept previous snapshot from N ago"
// from "no snapshot yet" when a fetch fails.
func networkLastSuccessAt(network string) time.Time {
	stateMu.RLock()
	defer stateMu.RUnlock()
	if snap, ok := state[network]; ok {
		return snap.lastSuccessAt
	}
	return time.Time{}
}
