package providers

import (
	"strings"
	"testing"

	"go-monitoring/internal/collector"
)

func TestKyberIncludedBalancerV3Source_DiscoveredPoolType(t *testing.T) {
	tests := []struct {
		name     string
		endpoint collector.Endpoint
		want     string
	}{
		{
			name: "STABLE from API",
			endpoint: collector.Endpoint{
				Name:     "KyberSwap-ETHEREUM-COMPOSABLE-STABLE-NO-HOOK-WETH-USDC-0xabcdef12",
				PoolType: "COMPOSABLE_STABLE",
			},
			want: "balancer-v3-stable",
		},
		{
			name: "GYROE",
			endpoint: collector.Endpoint{
				Name:     "KyberSwap-ETHEREUM-GYROE-NO-HOOK-WETH-USDC-0xabcdef12",
				PoolType: "GYROE",
			},
			want: "balancer-v3-eclp",
		},
		{
			name: "ReCLAMM via hook",
			endpoint: collector.Endpoint{
				Name:     "KyberSwap-ETHEREUM-WEIGHTED-RECLAMM-WETH-USDC-0xabcdef12",
				PoolType: "WEIGHTED",
				HookType: "ReCLAMM",
			},
			want: "balancer-v3-reclamm",
		},
		{
			name: "QuantAMM",
			endpoint: collector.Endpoint{
				Name:     "KyberSwap-ETHEREUM-QUANT-AMM-NO-HOOK-WETH-USDC-0xabcdef12",
				PoolType: "QUANT_AMM",
			},
			want: "balancer-v3-quantamm",
		},
		{
			name: "weighted only",
			endpoint: collector.Endpoint{
				Name:     "KyberSwap-ETHEREUM-WEIGHTED-NO-HOOK-WETH-USDC-0xabcdef12",
				PoolType: "WEIGHTED",
			},
			want: "balancer-v3-weighted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kyberIncludedBalancerV3Source(&tt.endpoint)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestKyberIncludedBalancerV3Source_BaseNameFallback(t *testing.T) {
	ep := collector.Endpoint{
		Name:     "KyberSwap-Base-Boosted-StableSurge(GHO/USDC)",
		PoolType: "",
	}
	got, err := kyberIncludedBalancerV3Source(&ep)
	if err != nil {
		t.Fatal(err)
	}
	if got != "balancer-v3-stable" {
		t.Fatalf("got %q", got)
	}
}

func TestKyberIncludedBalancerV3Source_PoolTypePreferredOverName(t *testing.T) {
	// Name would match Stable, but PoolType is Gyro — API metadata wins.
	ep := collector.Endpoint{
		Name:     "KyberSwap-ETHEREUM-GYROE-NO-HOOK-WETH-USDC-0xabcdef12-boosted-StableSurge",
		PoolType: "GYROE",
		HookType: "",
	}
	got, err := kyberIncludedBalancerV3Source(&ep)
	if err != nil {
		t.Fatal(err)
	}
	if got != "balancer-v3-eclp" {
		t.Fatalf("got %q want eclp", got)
	}
}

func TestKyberIncludedBalancerV3Source_Unsupported(t *testing.T) {
	ep := collector.Endpoint{
		Name:     "KyberSwap-ETHEREUM-FX-NO-HOOK-WETH-USDC-0xabcdef12",
		PoolType: "FX",
	}
	_, err := kyberIncludedBalancerV3Source(&ep)
	if err == nil || !strings.Contains(err.Error(), "unsupported pool type") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
