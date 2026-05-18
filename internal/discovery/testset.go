package discovery

import (
	"math/big"
	"sort"
	"strings"

	"go-monitoring/internal/collector"
)

// TestRow is one item in the daily test set: a single direction of swap
// through a single discovered pool, sized at TradePercent of TokenIn's balance.
// TokenInSymbol / TokenOutSymbol are display symbols aligned with TokenIn /
// TokenOut. Boosted is true only on the registered-token row when the pool
// emits a second underlying row (used for BaseName suffix).
type TestRow struct {
	Network         string
	PoolAddress     string
	PoolType        string
	HookType        string
	PoolSymbol      string
	TokenIn         string
	TokenOut        string
	TokenInSymbol   string
	TokenOutSymbol  string
	TokenInDec      int
	TokenOutDec     int
	SwapAmountRaw   string // raw on-chain units (post decimal conversion)
	Variant         string // "" for the registered row, "underlying" for the boosted underlying row
	Boosted         bool   // true only on the registered-token row when the pool is boosted (two rows)
}

// poolCandidate carries a selected pool plus the canonical unique pair chosen
// for it and the boosted/non-boosted classification of that pair.
type poolCandidate struct {
	pool       Pool
	pairTokens [2]PoolToken
	boosted    bool
}

// Build groups pools by network, then by (PoolType, HookType), and picks up
// to perGroup pools per group (see docs/discovery.md). For each
// selected pool it emits one row (non-boosted) or two rows (boosted:
// registered + underlying). Rows whose raw swap amount rounds to zero are
// dropped.
//
// Returns the row list plus a (network, poolAddress) set in the same key
// shape as collector.PoolKey, suitable for collector.IsPoolInTestSet.
func Build(pools []Pool, perGroup int, tradePercentByNetwork map[string]float64) ([]TestRow, map[string]struct{}) {
	rows := []TestRow{}
	poolKeys := map[string]struct{}{}

	if perGroup <= 0 {
		return rows, poolKeys
	}

	byNetwork := map[string][]Pool{}
	for _, p := range pools {
		byNetwork[p.Network] = append(byNetwork[p.Network], p)
	}

	networks := make([]string, 0, len(byNetwork))
	for n := range byNetwork {
		networks = append(networks, n)
	}
	sort.Strings(networks)

	for _, network := range networks {
		networkPools := byNetwork[network]

		tradePct, ok := tradePercentByNetwork[network]
		if !ok || tradePct <= 0 {
			continue
		}

		pairCount := networkPairCount(networkPools)

		candidates := make([]poolCandidate, 0)
		for _, p := range networkPools {
			if !hasCategory(p, CategoryUnique) {
				continue
			}
			// Skip surging StableSurge pools entirely: they're unreliable for
			// provider testing because the worsening-imbalance direction gets
			// a surge fee (so solvers may decline) and the improving direction
			// often involves a depegged side that solvers won't quote either.
			// The pool still appears on /pools with a surging badge.
			if p.Surging {
				continue
			}
			pair, ok := canonicalUniquePair(p, pairCount)
			if !ok {
				continue
			}
			candidates = append(candidates, poolCandidate{
				pool:       p,
				pairTokens: pair,
				boosted:    pair[0].Underlying != nil && pair[1].Underlying != nil,
			})
		}

		type groupKey struct{ poolType, hookType string }
		groups := map[groupKey][]poolCandidate{}
		for _, c := range candidates {
			k := groupKey{c.pool.Type, c.pool.HookType}
			groups[k] = append(groups[k], c)
		}

		groupKeys := make([]groupKey, 0, len(groups))
		for k := range groups {
			groupKeys = append(groupKeys, k)
		}
		sort.Slice(groupKeys, func(i, j int) bool {
			if groupKeys[i].poolType != groupKeys[j].poolType {
				return groupKeys[i].poolType < groupKeys[j].poolType
			}
			return groupKeys[i].hookType < groupKeys[j].hookType
		})

		for _, k := range groupKeys {
			selected := selectFromGroup(groups[k], perGroup)
			for _, c := range selected {
				key := collector.PoolKey(c.pool.Network, c.pool.Address)
				poolKeys[key] = struct{}{}

				if registered := buildRegisteredRow(c.pool, c.pairTokens, tradePct, c.boosted); registered != nil {
					rows = append(rows, *registered)
				}
				if c.boosted {
					if underlying := buildUnderlyingRow(c.pool, c.pairTokens, tradePct); underlying != nil {
						rows = append(rows, *underlying)
					}
				}
			}
		}
	}

	return rows, poolKeys
}

// networkPairCount returns the C(N,2) registered-pair frequency map for the
// surviving pools of a single network. Key is "lower|higher" of lowercased
// addresses, matching pairKeys in discovery.go.
func networkPairCount(pools []Pool) map[string]int {
	counts := map[string]int{}
	for _, p := range pools {
		addrs := make([]string, 0, len(p.Tokens))
		for _, t := range p.Tokens {
			addrs = append(addrs, strings.ToLower(t.Address))
		}
		for _, k := range pairKeys(addrs) {
			counts[k]++
		}
	}
	return counts
}

// canonicalUniquePair picks the (TokenA, TokenB) pair from the pool that is
// unique within the network (count==1) and has the highest combined USD
// balance. Ranking on USD (rather than token-count balance) means we always
// pick the pair that represents most of the pool's value — e.g. for a
// QuantAMM 3/94/3 PAXG/WBTC/USDC pool, the PAXG/USDC pair wins on USD even
// though USDC has the largest token-count balance. Returns ok=false if no
// unique pair exists in this pool.
func canonicalUniquePair(p Pool, pairCount map[string]int) ([2]PoolToken, bool) {
	type pairCand struct {
		a, b       PoolToken
		balanceUSD float64
	}
	cands := []pairCand{}
	for i := 0; i < len(p.Tokens); i++ {
		for j := i + 1; j < len(p.Tokens); j++ {
			a, b := p.Tokens[i], p.Tokens[j]
			lo, hi := strings.ToLower(a.Address), strings.ToLower(b.Address)
			if lo > hi {
				lo, hi = hi, lo
			}
			if pairCount[lo+"|"+hi] != 1 {
				continue
			}
			cands = append(cands, pairCand{
				a:          a,
				b:          b,
				balanceUSD: a.BalanceUSD + b.BalanceUSD,
			})
		}
	}
	if len(cands) == 0 {
		return [2]PoolToken{}, false
	}
	sort.SliceStable(cands, func(i, j int) bool {
		return cands[i].balanceUSD > cands[j].balanceUSD
	})
	return [2]PoolToken{cands[0].a, cands[0].b}, true
}

// selectFromGroup applies test set selection step 4 (docs/discovery.md) to a (PoolType,
// HookType) group: split into boosted / non-boosted, sort each by TVL desc,
// pick top-1+top-1 when both non-empty, else top-perGroup from the lone bucket.
func selectFromGroup(in []poolCandidate, perGroup int) []poolCandidate {
	boosted := []poolCandidate{}
	nonBoosted := []poolCandidate{}
	for _, c := range in {
		if c.boosted {
			boosted = append(boosted, c)
		} else {
			nonBoosted = append(nonBoosted, c)
		}
	}
	sortByTVLDesc(boosted)
	sortByTVLDesc(nonBoosted)

	if len(boosted) > 0 && len(nonBoosted) > 0 {
		return []poolCandidate{boosted[0], nonBoosted[0]}
	}
	if len(boosted) > 0 {
		return takeFirst(boosted, perGroup)
	}
	return takeFirst(nonBoosted, perGroup)
}

func sortByTVLDesc(in []poolCandidate) {
	sort.SliceStable(in, func(i, j int) bool {
		return in[i].pool.TotalLiquidityUSD > in[j].pool.TotalLiquidityUSD
	})
}

func takeFirst(in []poolCandidate, n int) []poolCandidate {
	if n >= len(in) {
		return in
	}
	return in[:n]
}

func hasCategory(p Pool, cat string) bool {
	for _, c := range p.Categories {
		if c == cat {
			return true
		}
	}
	return false
}

// buildRegisteredRow constructs the registered-token row for a selected pool.
// Returns nil if the raw swap amount rounds to zero or USD sizing inputs are
// missing.
func buildRegisteredRow(p Pool, pair [2]PoolToken, tradePct float64, boosted bool) *TestRow {
	tokenIn, tokenOut := orderBySmallestUSDBalance(pair[0], pair[1])
	swap := computeSwapAmountRaw(tokenIn.BalanceUSD, tokenIn.Balance, tokenIn.Decimals, tradePct)
	if swap == "" {
		return nil
	}
	return &TestRow{
		Network:        p.Network,
		PoolAddress:    p.Address,
		PoolType:       p.Type,
		HookType:       p.HookType,
		PoolSymbol:     p.Symbol,
		TokenIn:        tokenIn.Address,
		TokenOut:       tokenOut.Address,
		TokenInSymbol:  tokenIn.Symbol,
		TokenOutSymbol: tokenOut.Symbol,
		TokenInDec:     tokenIn.Decimals,
		TokenOutDec:    tokenOut.Decimals,
		SwapAmountRaw:  swap,
		Variant:        "",
		Boosted:        boosted,
	}
}

// buildUnderlyingRow constructs the underlying-token row for a boosted pool.
// Returns nil if the raw swap amount rounds to zero or either token is
// missing an underlying.
func buildUnderlyingRow(p Pool, pair [2]PoolToken, tradePct float64) *TestRow {
	if pair[0].Underlying == nil || pair[1].Underlying == nil {
		return nil
	}
	// Each underlying inherits the registered token's BalanceUSD directly
	// (the USD value of the registered position) but uses its own decimals.
	// We keep the registered side's `balance` purely as a price proxy for
	// computeSwapAmountRaw — the implied price (balanceUSD/balance) is the
	// registered-side price, which is what aggregators actually quote
	// against when swapping the underlying through the buffer.
	u0 := PoolToken{
		Address:    pair[0].Underlying.Address,
		Symbol:     pair[0].Underlying.Symbol,
		Decimals:   pair[0].Underlying.Decimals,
		Balance:    pair[0].Balance,
		BalanceUSD: pair[0].BalanceUSD,
	}
	u1 := PoolToken{
		Address:    pair[1].Underlying.Address,
		Symbol:     pair[1].Underlying.Symbol,
		Decimals:   pair[1].Underlying.Decimals,
		Balance:    pair[1].Balance,
		BalanceUSD: pair[1].BalanceUSD,
	}
	tokenIn, tokenOut := orderBySmallestUSDBalance(u0, u1)
	swap := computeSwapAmountRaw(tokenIn.BalanceUSD, tokenIn.Balance, tokenIn.Decimals, tradePct)
	if swap == "" {
		return nil
	}
	return &TestRow{
		Network:        p.Network,
		PoolAddress:    p.Address,
		PoolType:       p.Type,
		HookType:       p.HookType,
		PoolSymbol:     p.Symbol,
		TokenIn:        tokenIn.Address,
		TokenOut:       tokenOut.Address,
		TokenInSymbol:  tokenIn.Symbol,
		TokenOutSymbol: tokenOut.Symbol,
		TokenInDec:     tokenIn.Decimals,
		TokenOutDec:    tokenOut.Decimals,
		SwapAmountRaw:  swap,
		Variant:        "underlying",
	}
}

// orderBySmallestUSDBalance returns (tokenIn, tokenOut) using the pool's USD
// balances: the token with the smaller USD balance is TokenIn (trade size is
// a percent of that side's USD value, so the trade is sized to what the
// thinner side can support). This also picks the "swap toward balance"
// direction for StableSurge pools (since swapping IN the underrepresented
// token reduces the pool's imbalance, avoiding the surge fee). Equal USD or
// missing/zero USD balances fall back to lexicographic address order for a
// stable direction.
func orderBySmallestUSDBalance(a, b PoolToken) (PoolToken, PoolToken) {
	if a.BalanceUSD > 0 && b.BalanceUSD > 0 && a.BalanceUSD != b.BalanceUSD {
		if a.BalanceUSD < b.BalanceUSD {
			return a, b
		}
		return b, a
	}
	return orderByAddress(a, b)
}

// orderByAddress returns (a, b) sorted by lowercased address (lexicographic).
func orderByAddress(a, b PoolToken) (PoolToken, PoolToken) {
	if strings.ToLower(a.Address) <= strings.ToLower(b.Address) {
		return a, b
	}
	return b, a
}

// computeSwapAmountRaw produces the raw on-chain swap amount as a decimal
// string sized at tradePct of tokenIn's USD balance. The USD balance gates
// whether we have enough information to size a row at all (drop if missing
// or zero); given a positive balanceUSD, the implied per-token price
// (balanceUSD / humanBalance) lets us convert the USD trade size to token
// units. The two cancel out — tradeUSD/price simplifies to (tradePct/100) *
// humanBalance — so we keep the math simple while preserving the USD guard.
// Returns "" if any of balanceUSD, humanBalance, or tradePct is non-positive,
// or the final raw amount rounds to zero (drop the row; see docs/discovery.md).
func computeSwapAmountRaw(balanceUSD float64, humanBalance string, decimals int, tradePct float64) string {
	if balanceUSD <= 0 || tradePct <= 0 {
		return ""
	}
	humanBal, ok := new(big.Float).SetString(humanBalance)
	if !ok || humanBal.Sign() <= 0 {
		return ""
	}

	pct := new(big.Float).SetFloat64(tradePct / 100.0)
	scaled := new(big.Float).Mul(humanBal, pct)

	mult := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	scaled.Mul(scaled, mult)

	raw, _ := scaled.Int(nil)
	if raw == nil || raw.Sign() <= 0 {
		return ""
	}
	return raw.String()
}
