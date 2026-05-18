package discovery

import "testing"

// TestBuild_QuantAMM_PAXG_USDC reproduces the live "Safe Haven-BTC:PAXG:USDC"
// QuantAMM pool that exposed the original token-count ordering bug: PAXG had
// the smallest token-count balance (25 tokens) but by far the largest USD
// balance (~$119K), and the smallest-side USDC reserve held only ~$3.8K. With
// the legacy heuristic, PAXG/USDC was picked with PAXG = TokenIn and 5% of
// 25 PAXG ≈ 1.27 PAXG ≈ $6K — way more USDC than the pool's $3.8K reserve
// could supply, so the trade was unroutable.
//
// Post-fix expectations:
//   - PAXG/USDC pair wins canonical selection (highest combined balanceUSD).
//   - USDC = TokenIn (smaller balanceUSD), PAXG = TokenOut.
//   - Trade size = 5% × $3,792 ≈ $190 ⇒ ~189 USDC ⇒ 189,000,000 raw (decimals 6).
func TestBuild_QuantAMM_PAXG_USDC(t *testing.T) {
	pool := Pool{
		Address:           "0x6b61d8680c4f9e560c8306807908553f95c749c5",
		Network:           "1",
		Type:              "QUANT_AMM_WEIGHTED",
		Symbol:            "Safe Haven-BTC:PAXG:USDC",
		Categories:        []string{CategoryUnique},
		TotalLiquidityUSD: 126749.83,
		Tokens: []PoolToken{
			{
				// WBTC is the smallest USD leg; nudged slightly under USDC
				// so PAXG+USDC is the unambiguous top-USD pair in this
				// fixture (live data has WBTC and USDC within $20 of each
				// other; the test would otherwise straddle the tie point).
				Address:    "0x2260fac5e5542a773aa44fbcfedf7c193bc2c599",
				Symbol:     "WBTC",
				Decimals:   8,
				Balance:    "0.047642",
				BalanceUSD: 3000.00,
			},
			{
				Address:    "0x45804880de22913dafe09f4980848ece6ecbaf78",
				Symbol:     "PAXG",
				Decimals:   18,
				Balance:    "25.423427981256008",
				BalanceUSD: 119146.39,
			},
			{
				Address:    "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
				Symbol:     "USDC",
				Decimals:   6,
				Balance:    "3793.41806",
				BalanceUSD: 3792.84,
			},
		},
	}

	rows, poolKeys := Build([]Pool{pool}, 1, map[string]float64{"1": 5})

	if got, want := len(rows), 1; got != want {
		t.Fatalf("len(rows)=%d, want %d (rows=%+v)", got, want, rows)
	}
	if got, want := len(poolKeys), 1; got != want {
		t.Fatalf("len(poolKeys)=%d, want %d", got, want)
	}
	row := rows[0]

	wantTokenIn := "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"  // USDC
	wantTokenOut := "0x45804880de22913dafe09f4980848ece6ecbaf78" // PAXG
	if row.TokenIn != wantTokenIn {
		t.Fatalf("TokenIn=%s, want USDC %s", row.TokenIn, wantTokenIn)
	}
	if row.TokenOut != wantTokenOut {
		t.Fatalf("TokenOut=%s, want PAXG %s", row.TokenOut, wantTokenOut)
	}
	if row.TokenInSymbol != "USDC" || row.TokenOutSymbol != "PAXG" {
		t.Fatalf("symbols: got in=%s out=%s, want USDC -> PAXG", row.TokenInSymbol, row.TokenOutSymbol)
	}

	// 5% of 3793.41806 USDC = 189.670903 USDC -> floor(189.670903 * 1e6) = 189670903
	const wantRaw = "189670903"
	if row.SwapAmountRaw != wantRaw {
		t.Fatalf("SwapAmountRaw=%s, want %s (≈5%%×$3,792 USDC)", row.SwapAmountRaw, wantRaw)
	}
}
