package discovery

import "testing"

func TestOrderBySmallestUSDBalance(t *testing.T) {
	t.Run("smaller USD balance is token in, regardless of token-count balance", func(t *testing.T) {
		// PAXG-style: small token count, huge USD. USDC-style: large token count, modest USD.
		paxg := PoolToken{Address: "0xaaa", Balance: "25", BalanceUSD: 119146}
		usdc := PoolToken{Address: "0xbbb", Balance: "3793", BalanceUSD: 3792}
		in, out := orderBySmallestUSDBalance(paxg, usdc)
		if in.Address != usdc.Address || out.Address != paxg.Address {
			t.Fatalf("got in=%s out=%s, want in=%s out=%s", in.Address, out.Address, usdc.Address, paxg.Address)
		}
	})

	t.Run("equal USD falls back to lexicographic address order", func(t *testing.T) {
		a := PoolToken{Address: "0xbbb", Balance: "10", BalanceUSD: 100}
		b := PoolToken{Address: "0xaaa", Balance: "10", BalanceUSD: 100}
		in, out := orderBySmallestUSDBalance(a, b)
		if in.Address != "0xaaa" || out.Address != "0xbbb" {
			t.Fatalf("got in=%s out=%s, want lower address first", in.Address, out.Address)
		}
	})

	t.Run("zero USD on one side falls back to address order", func(t *testing.T) {
		missing := PoolToken{Address: "0xaaa", Balance: "10", BalanceUSD: 0}
		known := PoolToken{Address: "0xbbb", Balance: "1", BalanceUSD: 5}
		in, out := orderBySmallestUSDBalance(missing, known)
		if in.Address != "0xaaa" || out.Address != "0xbbb" {
			t.Fatalf("got in=%s out=%s, want lexicographic fallback", in.Address, out.Address)
		}
	})

	t.Run("zero USD on both sides falls back to address order", func(t *testing.T) {
		a := PoolToken{Address: "0xbbb", Balance: "10", BalanceUSD: 0}
		b := PoolToken{Address: "0xaaa", Balance: "10", BalanceUSD: 0}
		in, out := orderBySmallestUSDBalance(a, b)
		if in.Address != "0xaaa" || out.Address != "0xbbb" {
			t.Fatalf("got in=%s out=%s, want lexicographic fallback", in.Address, out.Address)
		}
	})
}
