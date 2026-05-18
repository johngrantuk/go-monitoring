package discovery

import (
	"math"
	"strconv"
	"testing"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
)

func TestStableSurgeImbalance(t *testing.T) {
	cases := []struct {
		name   string
		tokens []PoolToken
		want   float64
	}{
		{
			name: "balanced 50/50 pool returns 0",
			tokens: []PoolToken{
				{BalanceUSD: 100},
				{BalanceUSD: 100},
			},
			want: 0,
		},
		{
			name: "90/10 pool returns 0.8",
			tokens: []PoolToken{
				{BalanceUSD: 90},
				{BalanceUSD: 10},
			},
			want: 0.8,
		},
		{
			name: "live surge pool 99.76/0.24 returns ~0.9953",
			tokens: []PoolToken{
				{BalanceUSD: 20787261.55583828}, // vgUSDC
				{BalanceUSD: 49460.15549372429}, // xUSD
			},
			want: 0.9952526,
		},
		{
			name: "3-token imbalanced pool uses median",
			tokens: []PoolToken{
				{BalanceUSD: 100},
				{BalanceUSD: 1000},
				{BalanceUSD: 3000},
			},
			want: 2900.0 / 4100.0,
		},
		{
			name: "balanced 3-token pool returns 0",
			tokens: []PoolToken{
				{BalanceUSD: 100},
				{BalanceUSD: 100},
				{BalanceUSD: 100},
			},
			want: 0,
		},
		{
			name:   "single-token pool returns 0",
			tokens: []PoolToken{{BalanceUSD: 100}},
			want:   0,
		},
		{
			name:   "empty pool returns 0",
			tokens: nil,
			want:   0,
		},
		{
			name: "missing USD on any token returns 0",
			tokens: []PoolToken{
				{BalanceUSD: 100},
				{BalanceUSD: 0},
			},
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stableSurgeImbalance(tc.tokens)
			if math.Abs(got-tc.want) > 1e-6 {
				t.Fatalf("stableSurgeImbalance=%.10f, want %.10f", got, tc.want)
			}
		})
	}
}

// TestProcessNetwork_SurgeFlag exercises the surge detection path through
// processNetwork: a StableSurge pool whose USD imbalance is within
// stableSurgeImbalanceBuffer of its surgeThresholdPercentage is flagged
// Surging; a comfortably balanced pool with the same hook is not.
//
// A 2-token pool's maximum imbalance is 1.0 (one side at ~100%), so we use a
// configured surgeThresholdPercentage of 0.4 in the surging fixture — a 90/10
// USD split gives imb=0.8 ≥ 0.4 − 0.10 buffer = 0.3, which trips Surging.
// The calm fixture uses the live 0.6 threshold and a near-balanced split that
// stays well below the trigger band.
func TestProcessNetwork_SurgeFlag(t *testing.T) {
	cfg := config.DiscoveryConfig{Network: "1", Enabled: true, TVLThresholdUSD: 0, TradePercent: 5}

	makePool := func(addr, threshold string, balUSD0, balUSD1 float64) rawPool {
		var p rawPool
		p.Address = addr
		p.Type = "STABLE"
		p.Hook = &rawHook{
			Type: stableSurgeHookType,
			Params: &rawHookParams{
				SurgeThresholdPercentage: threshold,
				MaxSurgeFeePercentage:    "0.09",
			},
		}
		p.DynamicData.TotalLiquidity = formatDecimal(balUSD0 + balUSD1)
		p.DynamicData.SwapFee = "0.0001"
		p.DynamicData.Volume24h = "0"
		p.PoolTokens = []rawPoolToken{
			{Address: "0xaaa", Symbol: "A", Decimals: 18, Balance: "1", BalanceUSD: formatDecimal(balUSD0)},
			{Address: "0xbbb", Symbol: "B", Decimals: 18, Balance: "1", BalanceUSD: formatDecimal(balUSD1)},
		}
		return p
	}

	surging := makePool("0xsurging", "0.4", 9000, 1000) // imb=0.8 -> 0.8 >= 0.4-0.1 = 0.3 ✓
	calm := makePool("0xcalm", "0.6", 5500, 4500)       // imb=0.1 -> 0.1 < 0.6-0.1 = 0.5

	pools := processNetwork(cfg, []rawPool{surging, calm})

	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(pools))
	}
	byAddr := map[string]Pool{}
	for _, p := range pools {
		byAddr[p.Address] = p
	}

	gotSurging, ok := byAddr["0xsurging"]
	if !ok {
		t.Fatalf("surging pool missing from output")
	}
	if !gotSurging.Surging {
		t.Fatalf("expected Surging=true (imb=%.4f, threshold=%.4f, buffer=%.4f)",
			gotSurging.SurgeImbalance, gotSurging.SurgeThreshold, stableSurgeImbalanceBuffer)
	}
	if math.Abs(gotSurging.SurgeImbalance-0.8) > 1e-6 {
		t.Fatalf("SurgeImbalance=%.6f, want 0.8", gotSurging.SurgeImbalance)
	}
	if math.Abs(gotSurging.SurgeThreshold-0.4) > 1e-6 {
		t.Fatalf("SurgeThreshold=%.6f, want 0.4", gotSurging.SurgeThreshold)
	}

	gotCalm, ok := byAddr["0xcalm"]
	if !ok {
		t.Fatalf("calm pool missing from output")
	}
	if gotCalm.Surging {
		t.Fatalf("expected Surging=false (imb=%.4f, threshold=%.4f, buffer=%.4f)",
			gotCalm.SurgeImbalance, gotCalm.SurgeThreshold, stableSurgeImbalanceBuffer)
	}
}

func TestProcessNetwork_SkipsAkronHook(t *testing.T) {
	cfg := config.DiscoveryConfig{Network: "1", Enabled: true, TVLThresholdUSD: 0, TradePercent: 5}

	makePool := func(addr, hookType string) rawPool {
		var p rawPool
		p.Address = addr
		p.Type = "WEIGHTED"
		p.DynamicData.TotalLiquidity = "50000"
		p.DynamicData.SwapFee = "0.0001"
		p.DynamicData.Volume24h = "0"
		p.PoolTokens = []rawPoolToken{
			{Address: "0xaaa", Symbol: "A", Decimals: 18, Balance: "1", BalanceUSD: "25000"},
			{Address: "0xbbb", Symbol: "B", Decimals: 18, Balance: "1", BalanceUSD: "25000"},
		}
		if hookType != "" {
			p.Hook = &rawHook{Type: hookType}
		}
		return p
	}

	pools := processNetwork(cfg, []rawPool{
		makePool("0xakron", akronHookType),
		makePool("0xplain", ""),
	})

	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].Address != "0xplain" {
		t.Fatalf("got address %s, want 0xplain", pools[0].Address)
	}
}

func TestProcessNetwork_SkipsWeightedStableSurge(t *testing.T) {
	cfg := config.DiscoveryConfig{Network: "1", Enabled: true, TVLThresholdUSD: 0, TradePercent: 5}

	makePool := func(addr, poolType, hookType string) rawPool {
		var p rawPool
		p.Address = addr
		p.Type = poolType
		p.DynamicData.TotalLiquidity = "50000"
		p.DynamicData.SwapFee = "0.0001"
		p.DynamicData.Volume24h = "0"
		p.PoolTokens = []rawPoolToken{
			{Address: "0xaaa", Symbol: "A", Decimals: 18, Balance: "1", BalanceUSD: "25000"},
			{Address: "0xbbb", Symbol: "B", Decimals: 18, Balance: "1", BalanceUSD: "25000"},
		}
		if hookType != "" {
			p.Hook = &rawHook{Type: hookType}
		}
		return p
	}

	pools := processNetwork(cfg, []rawPool{
		makePool("0xweighted-surge", "WEIGHTED", stableSurgeHookType),
		makePool("0xstable-surge", "STABLE", stableSurgeHookType),
		makePool("0xweighted-plain", "WEIGHTED", ""),
	})

	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(pools))
	}
	byAddr := map[string]Pool{}
	for _, p := range pools {
		byAddr[p.Address] = p
	}
	if _, ok := byAddr["0xweighted-surge"]; ok {
		t.Fatal("WEIGHTED+STABLE_SURGE pool should be skipped")
	}
	if _, ok := byAddr["0xstable-surge"]; !ok {
		t.Fatal("STABLE+STABLE_SURGE pool should be kept")
	}
	if _, ok := byAddr["0xweighted-plain"]; !ok {
		t.Fatal("plain WEIGHTED pool should be kept")
	}
}

func TestProcessNetwork_SkipsExcludedPoolAddress(t *testing.T) {
	cfg := config.DiscoveryConfig{Network: "100", Enabled: true, TVLThresholdUSD: 0, TradePercent: 5}

	makePool := func(addr string) rawPool {
		var p rawPool
		p.Address = addr
		p.Type = "STABLE"
		p.DynamicData.TotalLiquidity = "50000"
		p.DynamicData.SwapFee = "0.0001"
		p.DynamicData.Volume24h = "0"
		p.PoolTokens = []rawPoolToken{
			{Address: "0xaaa", Symbol: "A", Decimals: 18, Balance: "1", BalanceUSD: "25000"},
			{Address: "0xbbb", Symbol: "B", Decimals: 18, Balance: "1", BalanceUSD: "25000"},
		}
		return p
	}

	pools := processNetwork(cfg, []rawPool{
		makePool("0x83470106402ed0bc83f91bb13266d35bdb23f1b9"),
		makePool("0x0000000000000000000000000000000000000001"),
	})

	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].Address != "0x0000000000000000000000000000000000000001" {
		t.Fatalf("got address %s, want 0x000...0001", pools[0].Address)
	}
}

func TestProcessNetwork_SkipsLBPPoolTypes(t *testing.T) {
	cfg := config.DiscoveryConfig{Network: "1", Enabled: true, TVLThresholdUSD: 0, TradePercent: 5}

	makePool := func(addr, poolType string) rawPool {
		var p rawPool
		p.Address = addr
		p.Type = poolType
		p.DynamicData.TotalLiquidity = "50000"
		p.DynamicData.SwapFee = "0.0001"
		p.DynamicData.Volume24h = "0"
		p.PoolTokens = []rawPoolToken{
			{Address: "0xaaa", Symbol: "A", Decimals: 18, Balance: "1", BalanceUSD: "25000"},
			{Address: "0xbbb", Symbol: "B", Decimals: 18, Balance: "1", BalanceUSD: "25000"},
		}
		return p
	}

	pools := processNetwork(cfg, []rawPool{
		makePool("0xfixed-lbp", "FIXED_LBP"),
		makePool("0xliquidity-lbp", "LIQUIDITY_BOOTSTRAPPING"),
		makePool("0xstable", "STABLE"),
	})

	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].Address != "0xstable" {
		t.Fatalf("got address %s, want 0xstable", pools[0].Address)
	}
}

// TestBuild_SkipsSurgingPool verifies that a Surging pool is excluded from
// test-set candidates while a calm StableSurge pool in the same group still
// produces a test row.
func TestBuild_SkipsSurgingPool(t *testing.T) {
	surging := Pool{
		Address:    "0xae255db04ba78519f33871c557d8fd6bafdb83bd",
		Network:    "1",
		Type:       "STABLE",
		HookType:   "STABLE_SURGE",
		Categories: []string{CategoryUnique},
		Tokens: []PoolToken{
			{Address: "0xaaa", Symbol: "vgUSDC", Decimals: 6, Balance: "401028294755", BalanceUSD: 20787261.55},
			{Address: "0xbbb", Symbol: "xUSD", Decimals: 6, Balance: "1207786", BalanceUSD: 49460.15},
		},
		SurgeThreshold: 0.6,
		SurgeImbalance: 0.9952526,
		Surging:        true,
	}
	calm := Pool{
		Address:    "0xcalm00000000000000000000000000000000000000",
		Network:    "1",
		Type:       "STABLE",
		HookType:   "STABLE_SURGE",
		Categories: []string{CategoryUnique},
		Tokens: []PoolToken{
			{Address: "0xccc", Symbol: "USDC", Decimals: 6, Balance: "5500", BalanceUSD: 5500},
			{Address: "0xddd", Symbol: "USDT", Decimals: 6, Balance: "4500", BalanceUSD: 4500},
		},
		SurgeThreshold: 0.6,
		SurgeImbalance: 0.05,
		Surging:        false,
	}

	rows, poolKeys := Build([]Pool{surging, calm}, 2, map[string]float64{"1": 5})

	if len(rows) != 1 {
		t.Fatalf("expected 1 row (calm only), got %d: %+v", len(rows), rows)
	}
	if rows[0].PoolAddress != calm.Address {
		t.Fatalf("row PoolAddress=%s, want calm %s", rows[0].PoolAddress, calm.Address)
	}
	if _, ok := poolKeys[collector.PoolKey(calm.Network, calm.Address)]; !ok {
		t.Fatalf("calm pool missing from poolKeys")
	}
	if _, ok := poolKeys[collector.PoolKey(surging.Network, surging.Address)]; ok {
		t.Fatalf("surging pool should not be in poolKeys")
	}
}

func formatDecimal(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
