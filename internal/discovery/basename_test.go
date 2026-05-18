package discovery

import "testing"

func TestSlugSegment(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"WETH", "WETH"},
		{"weth/usdc", "WETH-USDC"},
		{"Composable Stable", "COMPOSABLE-STABLE"},
		{"a..b", "A-B"},
		{"___", ""},
	}
	for _, tt := range tests {
		if got := slugSegment(tt.in); got != tt.want {
			t.Errorf("slugSegment(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatDiscoveredBaseName(t *testing.T) {
	tests := []struct {
		name string
		row  TestRow
		want string
	}{
		{
			name: "standard registered non boosted",
			row: TestRow{
				Network: "1", PoolAddress: "0xabcdef1234567890abcdef1234567890abcdef12",
				PoolType: "WEIGHTED", HookType: "",
				TokenInSymbol: "WETH", TokenOutSymbol: "USDC",
				Variant: "", Boosted: false,
			},
			want: "ETHEREUM-WEIGHTED-NO-HOOK-WETH-USDC-0xabcdef12",
		},
		{
			name: "hook nonempty",
			row: TestRow{
				Network: "1", PoolAddress: "0x1111111111111111111111111111111111111111",
				PoolType: "WEIGHTED", HookType: "ReCLAMM",
				TokenInSymbol: "WETH", TokenOutSymbol: "USDC",
				Variant: "", Boosted: false,
			},
			want: "ETHEREUM-WEIGHTED-RECLAMM-WETH-USDC-0x11111111",
		},
		{
			name: "underlying suffix beats boosted flag",
			row: TestRow{
				Network: "1", PoolAddress: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PoolType: "COMPOSABLE_STABLE", HookType: "",
				TokenInSymbol: "USDC", TokenOutSymbol: "USDT",
				Variant: "underlying", Boosted: false,
			},
			want: "ETHEREUM-COMPOSABLE-STABLE-NO-HOOK-USDC-USDT-0xaaaaaaaa-underlying",
		},
		{
			name: "boosted registered",
			row: TestRow{
				Network: "42161", PoolAddress: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				PoolType: "STABLE", HookType: "",
				TokenInSymbol: "WA-ARB", TokenOutSymbol: "WB",
				Variant: "", Boosted: true,
			},
			want: "ARBITRUM-STABLE-NO-HOOK-WA-ARB-WB-0xbbbbbbbb-boosted",
		},
		{
			name: "weird symbols",
			row: TestRow{
				Network: "1", PoolAddress: "0xcccccccccccccccccccccccccccccccccccccccc",
				PoolType: "GYROE", HookType: "",
				TokenInSymbol: "cbETH/rETH", TokenOutSymbol: "WSTETH",
				Variant: "", Boosted: false,
			},
			want: "ETHEREUM-GYROE-NO-HOOK-CBETH-RETH-WSTETH-0xcccccccc",
		},
		{
			name: "missing one symbol",
			row: TestRow{
				Network: "1", PoolAddress: "0xdddddddddddddddddddddddddddddddddddddddd",
				PoolType: "WEIGHTED", HookType: "",
				TokenInSymbol: "", TokenOutSymbol: "USDC",
				Variant: "", Boosted: false,
			},
			want: "ETHEREUM-WEIGHTED-NO-HOOK-UNKNOWN-USDC-0xdddddddd",
		},
		{
			name: "collision different pool same meta",
			row: TestRow{
				Network: "1", PoolAddress: "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
				PoolType: "WEIGHTED", HookType: "",
				TokenInSymbol: "WETH", TokenOutSymbol: "USDC",
				Variant: "", Boosted: false,
			},
			want: "ETHEREUM-WEIGHTED-NO-HOOK-WETH-USDC-0xeeeeeeee",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDiscoveredBaseName(tt.row); got != tt.want {
				t.Fatalf("formatDiscoveredBaseName() = %q want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDiscoveredBaseName_collisionDistinct(t *testing.T) {
	a := TestRow{
		Network: "1", PoolAddress: "0x1000000000000000000000000000000000000000",
		PoolType: "WEIGHTED", HookType: "",
		TokenInSymbol: "WETH", TokenOutSymbol: "USDC",
	}
	b := TestRow{
		Network: "1", PoolAddress: "0x2000000000000000000000000000000000000000",
		PoolType: "WEIGHTED", HookType: "",
		TokenInSymbol: "WETH", TokenOutSymbol: "USDC",
	}
	if formatDiscoveredBaseName(a) == formatDiscoveredBaseName(b) {
		t.Fatal("expected different BaseNames for different pool addresses")
	}
}
