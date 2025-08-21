package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

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
		Name:             "Mainet-Boosted-Stable(GHO/USDC)",
		Network:          "1",
		TokenIn:          "0x40d16fc0246ad3160ccc09b8d0d3a2cd28ae6c2f", // GHO
		TokenOut:         "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", // USDC
		TokenInDecimals:  18,
		TokenOutDecimals: 6,
		ExpectedPool:     "0x85b2b559bc2d21104c4defdd6efca8a20343361d",
		SwapAmount:       "1000000000000000000000000",
		ExpectedNoHops:   1,
	},
	{
		Name:             "Mainet-Boosted-StableSurge(wstETH/tETH)",
		Network:          "1",
		TokenIn:          "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0", // wstETH
		TokenOut:         "0xd11c452fc99cf405034ee446803b6f6c1f6d5ed8", // tETH
		TokenInDecimals:  18,
		TokenOutDecimals: 18,
		ExpectedPool:     "0x9ed5175aecb6653c1bdaa19793c16fd74fbeeb37",
		SwapAmount:       "150000000000000000000",
		ExpectedNoHops:   1,
	},
	{
		Name:             "Base-Boosted-Stable(wstETH/ezETH)",
		Network:          "8453",
		TokenIn:          "0xc1cba3fcea344f92d9239c08c0568f6f2f0ee452", // wstETH
		TokenOut:         "0x2416092f143378750bb29b79eD961ab195CcEea5", // ezETH
		TokenInDecimals:  18,
		TokenOutDecimals: 18,
		ExpectedPool:     "0xb5bfb5adb736ea852bd58fec71db3b356c2a3938",
		SwapAmount:       "10000000000000000000",
		ExpectedNoHops:   1,
	},
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
		SwapAmount:       "1000000",
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
		Name:             "Avax-Boosted-StableSurge(USDT/USDC)",
		Network:          "43114",
		TokenIn:          "0x9702230A8Ea53601f5cD2dc00fDBc13d4dF4A8c7", // USDT
		TokenOut:         "0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E", // USDC
		TokenInDecimals:  6,
		TokenOutDecimals: 6,
		ExpectedPool:     "0x31ae873544658654ce767bde179fd1bbcb84850b",
		SwapAmount:       "1000000000000",
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
		Name:             "Mainnet-Quant-BTF(PAXG/WBTC)",
		Network:          "1",
		TokenIn:          "0x45804880de22913dafe09f4980848ece6ecbaf78", // PAXG
		TokenOut:         "0x2260fac5e5542a773aa44fbcfedf7c193bc2c599", // WBTC
		TokenInDecimals:  18,
		TokenOutDecimals: 8,
		ExpectedPool:     "0x6b61d8680c4f9e560c8306807908553f95c749c5",
		SwapAmount:       "100000000000000000",
		ExpectedNoHops:   1,
	},
	{
		Name:             "Base-reCLAMM-(WETH/COW)",
		Network:          "8453",
		TokenIn:          "0x4200000000000000000000000000000000000006", // WETH
		TokenOut:         "0xc694a91e6b071bf030a18bd3053a7fe09b6dae69", // COW
		TokenInDecimals:  18,
		TokenOutDecimals: 18,
		ExpectedPool:     "0xff028c1ec4559d3aa2b0859aa582925b5cc28069",
		SwapAmount:       "1000000000000000000", // 1 WETH
		ExpectedNoHops:   1,
	},
	{
		Name:             "Mainnet-Boosted-reCLAMM-(WETH/AAVE)",
		Network:          "1",
		TokenIn:          "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
		TokenOut:         "0x7fc66500c84a76ad7e9c93437bfc5ac33e2ddae9", // AAVE
		TokenInDecimals:  18,
		TokenOutDecimals: 18,
		ExpectedPool:     "0x6cc9ef68864cd4c2af5a40ffb027c4b5428674a1",
		SwapAmount:       "3000000000000000000", // 3 WETH
		ExpectedNoHops:   1,
	},
	{
		Name:             "Hyper-Boosted-StableSurge-(USDT/USR)",
		Network:          "999",
		TokenIn:          "0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb", // USDT
		TokenOut:         "0x0ad339d66bf4aed5ce31c64bc37b3244b6394a77", // USR
		TokenInDecimals:  6,
		TokenOutDecimals: 18,
		ExpectedPool:     "0x8207c7541ce31b38dbd46890f2a832cf1ef7c512",
		SwapAmount:       "100000000000", // 100k USDT
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
		SupportedNetworks: []string{"1", "8453", "42161", "43114"}, // Mainnet, Base, Arbitrum, Avalanche
	},
	{
		Name:              "Odos",
		Type:              "odos",
		SupportedNetworks: []string{"1", "8453", "42161", "43114"}, // Mainnet, Base, Arbitrum, Avalanche
	},
	{
		Name:              "KyberSwap",
		Type:              "kyberswap",
		SupportedNetworks: []string{"1", "56", "42161", "137", "10", "43114", "8453", "324", "250", "59144", "534352", "5000", "81457", "146", "80094", "2020", "999"}, // All supported networks
	},
	{
		Name:              "HyperBloom",
		Type:              "hyperbloom",
		SupportedNetworks: []string{"999"}, // HyperEVM
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
