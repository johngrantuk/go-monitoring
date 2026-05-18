package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// DiscoveryConfig holds per-network configuration for Balancer V3 pool discovery.
type DiscoveryConfig struct {
	Network         string
	Enabled         bool
	TVLThresholdUSD float64
	TradePercent    float64 // percentage of TokenIn.balance used as trade size, e.g. 5 means 5%
}

// DiscoveryConfigs contains the discovery configuration per network.
var DiscoveryConfigs = []DiscoveryConfig{
	{Network: "1", Enabled: true, TVLThresholdUSD: 1_000_000, TradePercent: 5},
	{Network: "8453", Enabled: true, TVLThresholdUSD: 1_000_000, TradePercent: 5},
	{Network: "100", Enabled: true, TVLThresholdUSD: 1_000_000, TradePercent: 5},
}

// NetworkName maps a numeric network ID to a lowercase, human-readable name
// (e.g. "ethereum", "arbitrum"). Returns the input unchanged for unknown IDs
// so callers always get a non-empty string suitable for display / keys.
func NetworkName(network string) string {
	switch network {
	case "1":
		return "ethereum"
	case "8453":
		return "base"
	case "42161":
		return "arbitrum"
	case "10":
		return "optimism"
	case "100":
		return "gnosis"
	case "43114":
		return "avalanche"
	case "999":
		return "hyperevm"
	case "9745":
		return "plasma"
	case "143":
		return "monad"
	default:
		return network
	}
}

// BalancerAPIChain maps a numeric network ID to the Balancer GraphQL `GqlChain`
// enum value. Returns an empty string for networks not supported by the API.
func BalancerAPIChain(network string) string {
	switch network {
	case "1":
		return "MAINNET"
	case "8453":
		return "BASE"
	case "42161":
		return "ARBITRUM"
	case "10":
		return "OPTIMISM"
	case "100":
		return "GNOSIS"
	case "43114":
		return "AVALANCHE"
	case "999":
		return "HYPEREVM"
	case "9745":
		return "PLASMA"
	case "143":
		return "MONAD"
	default:
		return ""
	}
}

// GetDiscoveryIntervalHours returns the discovery interval in hours from the
// DISCOVERY_INTERVAL_HOURS environment variable. Defaults to 24 if unset or invalid.
func GetDiscoveryIntervalHours() int {
	envValue := os.Getenv("DISCOVERY_INTERVAL_HOURS")
	if envValue == "" {
		return 24
	}

	interval, err := strconv.Atoi(envValue)
	if err != nil || interval <= 0 {
		return 24
	}

	return interval
}

// GetDiscoveryTestPoolsPerGroup returns the maximum number of pools to select
// per (PoolType, HookType) group when building the daily test set, from the
// DISCOVERY_TEST_POOLS_PER_GROUP environment variable. Defaults to 1.
func GetDiscoveryTestPoolsPerGroup() int {
	envValue := os.Getenv("DISCOVERY_TEST_POOLS_PER_GROUP")
	if envValue == "" {
		return 1
	}

	n, err := strconv.Atoi(envValue)
	if err != nil || n <= 0 {
		return 1
	}

	return n
}

// BaseEndpoint represents the common configuration for an endpoint
type BaseEndpoint struct {
	Name             string
	Network          string
	TokenIn          string
	TokenOut         string
	TokenInDecimals  int
	TokenOutDecimals int
	ExpectedPool     string
	SwapAmount       string
	ExpectedNoHops   int
}

// RouteSolver represents a specific route solver configuration
type RouteSolver struct {
	Name              string
	Type              string // e.g. "paraswap", "1inch", "0x"
	SupportedNetworks []string
}

// GetEmailNotificationsEnabled checks if email notifications should be enabled
// based on environment variables at runtime
func GetEmailNotificationsEnabled() bool {
	envValue := os.Getenv("EMAIL_NOTIFICATIONS")
	if envValue == "" {
		return false // Default to false if not set
	}

	// Convert to lowercase for case-insensitive comparison
	envValue = strings.ToLower(envValue)

	// Check for various "true" values
	switch envValue {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// getRouteSolverEnabled checks if a specific route solver should be enabled
// based on environment variables. Returns true by default if no env var is found.
func getRouteSolverEnabled(solverType string) bool {
	envVarName := "DISABLE_" + strings.ToUpper(solverType)
	envValue := os.Getenv(envVarName)
	if envValue == "" {
		return true // Default to enabled if no env var is found
	}

	// Convert to lowercase for case-insensitive comparison
	envValue = strings.ToLower(envValue)

	// Check for various "true" values that would disable the solver
	switch envValue {
	case "true", "1", "yes", "on", "disable":
		return false
	default:
		return true
	}
}

// BaseEndpoints contains all base endpoint configurations
var BaseEndpoints = []BaseEndpoint{
	{
		Name:             "Base-Boosted-StableSurge(GHO/USDC)",
		Network:          "8453",
		TokenIn:          "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC
		TokenOut:         "0x6Bb7a212910682DCFdbd5BCBb3e28FB4E8da10Ee", // GHO
		TokenInDecimals:  6,
		TokenOutDecimals: 18,
		ExpectedPool:     "0x7ab124ec4029316c2a42f713828ddf2a192b36db",
		SwapAmount:       "100000000000", // 100000
		ExpectedNoHops:   1,
	},
	{
		Name:             "Arbitrum-Boosted-Stable(WETH/WSTETH)",
		Network:          "42161",
		TokenIn:          "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1", // WETH
		TokenOut:         "0x5979D7b546E38E414F7E9822514be443A4800529", // WSTETH
		TokenInDecimals:  18,
		TokenOutDecimals: 18,
		ExpectedPool:     "0xc072880e1bc0bcddc99db882c7f3e7a839281cf4",
		SwapAmount:       "10000000000000000000",
		ExpectedNoHops:   1,
	},
	{
		Name:             "Arbitrum-Boosted-StableSurge(GHO/USDC)",
		Network:          "42161",
		TokenIn:          "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", // USDC
		TokenOut:         "0x7dfF72693f6A4149b17e7C6314655f6A9F7c8B33", // GHO
		TokenInDecimals:  6,
		TokenOutDecimals: 18,
		ExpectedPool:     "0x19b001e6bc2d89154c18e2216eec5c8c6047b6d8",
		SwapAmount:       "100000000000", // 100000
		ExpectedNoHops:   1,
	},
	{
		Name:             "Arbitrum-Boosted-GyroE(eBTC/WETH)",
		Network:          "42161",
		TokenIn:          "0x657e8C867D8B37dCC18fA4Caead9C45EB088C642", // eBTC
		TokenOut:         "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1", // WETH
		TokenInDecimals:  8,
		TokenOutDecimals: 18,
		ExpectedPool:     "0xc6ac6abae59d58213800ace88d44526725d75f3a",
		ExpectedNoHops:   1,
		SwapAmount:       "100000",
	},
	{
		Name:             "Gnosis-Boosted-Stable(WETH/wstETH)",
		Network:          "100",
		TokenIn:          "0x6a023ccd1ff6f2045c3309768ead9e68f978f6e1", // WETH
		TokenOut:         "0x6c76971f98945ae98dd7d4dfca8711ebea946ea6", // wstETH
		TokenInDecimals:  18,
		TokenOutDecimals: 18,
		ExpectedPool:     "0x6e6bb18449fcf15b79efa2cfa70acf7593088029",
		SwapAmount:       "1000000000000000000",
		ExpectedNoHops:   1,
	},
	{
		Name:             "Avax-Boosted-GyroE(BTC.b/wAVAX)",
		Network:          "43114",
		TokenIn:          "0x152b9d0FdC40C096757F570A51E494bd4b943E50", // BTC.b
		TokenOut:         "0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7", // wAVAX
		TokenInDecimals:  8,
		TokenOutDecimals: 18,
		ExpectedPool:     "0x58374fff35d1f3023bbfc646fb9ecd2b180ca0b0",
		SwapAmount:       "10000000",
		ExpectedNoHops:   1,
	},
	{
		Name:             "Hyper-Boosted-StableSurge-(USDT/USDXL)",
		Network:          "999",
		TokenIn:          "0xb88339CB7199b77E23DB6E890353E22632Ba630f", // USDC
		TokenOut:         "0xBE65F0F410A72BeC163dC65d46c83699e957D588", // USDp
		TokenInDecimals:  6,
		TokenOutDecimals: 18,
		ExpectedPool:     "0xc5619cfcce9fae18eda1d1e923aa1fdea42d93b7",
		SwapAmount:       "100000000000", // 100k USDC
		ExpectedNoHops:   1,
	},
	{
		Name:             "Monad-Boosted-StableSurge-(USDT/AUSD/USDC)",
		Network:          "143",
		TokenIn:          "0x00000000efe302beaa2b3e6e1b18d08d69a9012a", // AUSD
		TokenOut:         "0x754704bc059f8c67012fed69bc8a327a5aafb603", // USDC
		TokenInDecimals:  6,
		TokenOutDecimals: 6,
		ExpectedPool:     "0x2daa146dfb7eaef0038f9f15b2ec1e4de003f72b",
		SwapAmount:       "10000000000", // 10k USDC
		ExpectedNoHops:   1,
	},
}

// RouteSolvers contains all available route solver configurations
var RouteSolvers = []RouteSolver{
	{
		Name:              "Paraswap",
		Type:              "paraswap",
		SupportedNetworks: []string{"1", "8453", "42161", "100", "43114"}, // Mainnet, Base, Arbitrum, Gnosis, Avalanche
	},
	{
		Name:              "1inch",
		Type:              "1inch",
		SupportedNetworks: []string{"1", "8453", "42161", "100", "43114"}, // Mainnet, Base, Arbitrum, Gnosis, Avalanche
	},
	{
		Name:              "0x",
		Type:              "0x",
		SupportedNetworks: []string{"1", "8453", "42161", "43114", "9745", "143"}, // Mainnet, Base, Arbitrum, Avalanche, Plasma, Monad
	},
	{
		Name:              "Odos",
		Type:              "odos",
		SupportedNetworks: []string{"1", "8453", "42161", "43114"}, // Mainnet, Base, Arbitrum, Avalanche
	},
	{
		Name:              "KyberSwap",
		Type:              "kyberswap",
		SupportedNetworks: []string{"1", "56", "42161", "137", "10", "43114", "8453", "324", "250", "59144", "534352", "5000", "81457", "146", "80094", "2020", "999", "9745", "143"}, // All supported networks
	},
	{
		Name:              "HyperBloom",
		Type:              "hyperbloom",
		SupportedNetworks: []string{"999"}, // HyperEVM
	},
	{
		Name:              "Balancer SOR",
		Type:              "balancer_sor",
		SupportedNetworks: []string{"1", "42161", "10", "8453", "43114", "100", "999", "9745", "143"}, // Mainnet, Arbitrum, Optimism, Base, Avalanche, Gnosis, HyperEVM, Plasma, Monad
	},
	{
		Name:              "Barter",
		Type:              "barter",
		SupportedNetworks: []string{"1"}, // Mainnet
	},
	{
		Name:              "OpenOcean",
		Type:              "openocean",
		SupportedNetworks: []string{"1", "8453", "42161", "43114", "100", "143"}, // Mainnet, Base, Arbitrum, Avalanche, Gnosis, Monad
	},
}

// GetEnabledRouteSolvers returns only the enabled route solvers based on environment variables
func GetEnabledRouteSolvers() []RouteSolver {
	var enabledSolvers []RouteSolver
	for _, solver := range RouteSolvers {
		if getRouteSolverEnabled(solver.Type) {
			enabledSolvers = append(enabledSolvers, solver)
		}
	}
	return enabledSolvers
}

// GetRouteSolverDelay returns the delay for a specific route solver based on environment variables
// Environment variable format: DELAY_<ROUTESOLVER> (e.g., DELAY_KYBERSWAP, DELAY_HYPERBLOOM)
// Defaults to 2 seconds if no environment variable is found
func GetRouteSolverDelay(routeSolver string) time.Duration {
	envVarName := "DELAY_" + strings.ToUpper(routeSolver)
	envValue := os.Getenv(envVarName)

	if envValue == "" {
		return 2 * time.Second // Default to 2 seconds
	}

	// Try to parse as seconds (integer)
	if seconds, err := strconv.Atoi(envValue); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}

	// If parsing fails, return default
	return 2 * time.Second
}

// GetRPCURL returns the RPC URL for a given network chain ID.
func GetRPCURL(network string) string {
	var envVarName string
	switch network {
	case "1":
		envVarName = "ETHEREUM_RPC_URL"
	case "42161":
		envVarName = "ARBITRUM_RPC_URL"
	case "10":
		envVarName = "OPTIMISM_RPC_URL"
	case "8453":
		envVarName = "BASE_RPC_URL"
	case "43114":
		envVarName = "AVALANCHE_RPC_URL"
	case "100":
		envVarName = "GNOSIS_RPC_URL"
	case "999":
		envVarName = "HYPEREVM_RPC_URL"
	case "9745":
		envVarName = "PLASMA_RPC_URL"
	case "143":
		envVarName = "MONAD_RPC_URL"
	default:
		return ""
	}
	return os.Getenv(envVarName)
}
