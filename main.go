package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// BaseEndpoint represents the common configuration for an endpoint
type BaseEndpoint struct {
	Name                  string
	Network               string
	TokenIn               string
	TokenOut              string
	TokenInDecimals       int
	TokenOutDecimals      int
	ExpectedPool          string
	SwapAmount            string
	ExpectedNoHops        int
	CheckInterval         int
	NotificationType      string
	NotificationRecipient string
}

// RouteSolver represents a specific route solver configuration
type RouteSolver struct {
	Name              string
	Type              string // "paraswap", "1inch", "0x"
	SupportedNetworks []string
	Enabled           bool
}

// Endpoint represents a monitored API endpoint
type Endpoint struct {
	Name                  string
	BaseName              string
	SolverName            string
	RouteSolver           string
	Network               string
	TokenIn               string
	TokenOut              string
	TokenInDecimals       int
	TokenOutDecimals      int
	SwapAmount            string
	ExpectedPool          string
	ExpectedNoHops        int
	CheckInterval         int
	LastStatus            string
	LastChecked           time.Time
	NotificationType      string
	NotificationRecipient string
	Message               string
}

// API monitoring data
var (
	endpoints    = make([]Endpoint, 0)
	mu           sync.Mutex // Prevents race conditions
	routeSolvers = []RouteSolver{
		{
			Name:              "Paraswap",
			Type:              "paraswap",
			SupportedNetworks: []string{"1", "8453", "42161", "100"}, // Mainnet, Base, Arbitrum, Gnosis
			Enabled:           true,
		},
		{
			Name:              "1inch",
			Type:              "1inch",
			SupportedNetworks: []string{"1", "8453", "42161", "100"}, // Mainnet, Base, Arbitrum, Gnosis
			Enabled:           true,
		},
		{
			Name:              "0x",
			Type:              "0x",
			SupportedNetworks: []string{"1", "8453", "42161"}, // Mainnet, Base, Arbitrum
			Enabled:           true,
		},
	}
)

func main() {
	// Define base endpoint configurations
	baseEndpoints := []BaseEndpoint{
		{
			Name:                  "Mainet-Boosted",
			Network:               "1",
			TokenIn:               "0x40d16fc0246ad3160ccc09b8d0d3a2cd28ae6c2f", // GHO
			TokenOut:              "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", // USDC
			TokenInDecimals:       18,
			TokenOutDecimals:      6,
			ExpectedPool:          "0x85b2b559bc2d21104c4defdd6efca8a20343361d",
			SwapAmount:            "1000000000000000000000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
			ExpectedNoHops:        1,
		},
		{
			Name:                  "Mainet-Boosted-StableSurge",
			Network:               "1",
			TokenIn:               "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0", // wstETH
			TokenOut:              "0xd11c452fc99cf405034ee446803b6f6c1f6d5ed8", // tETH
			TokenInDecimals:       18,
			TokenOutDecimals:      18,
			ExpectedPool:          "0x9ed5175aecb6653c1bdaa19793c16fd74fbeeb37",
			SwapAmount:            "150000000000000000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
			ExpectedNoHops:        1,
		},
		{
			Name:                  "Base-Boosted-wstETH/ezETH",
			Network:               "8453",
			TokenIn:               "0xc1cba3fcea344f92d9239c08c0568f6f2f0ee452", // wstETH
			TokenOut:              "0x2416092f143378750bb29b79eD961ab195CcEea5", // ezETH
			TokenInDecimals:       18,
			TokenOutDecimals:      18,
			ExpectedPool:          "0xb5bfb5adb736ea852bd58fec71db3b356c2a3938",
			SwapAmount:            "10000000000000000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
			ExpectedNoHops:        1,
		},
		{
			Name:                  "Base-Boosted-StableSurge",
			Network:               "8453",
			TokenIn:               "0x6Bb7a212910682DCFdbd5BCBb3e28FB4E8da10Ee", // GHO
			TokenOut:              "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC
			TokenInDecimals:       18,
			TokenOutDecimals:      6,
			ExpectedPool:          "0x7ab124ec4029316c2a42f713828ddf2a192b36db",
			SwapAmount:            "1000000000000000000000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
			ExpectedNoHops:        1,
		},
		{
			Name:                  "Base-Boosted-StableSurge-WETH/USDC",
			Network:               "8453",
			TokenIn:               "0x4200000000000000000000000000000000000006", // WETH
			TokenOut:              "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC
			TokenInDecimals:       18,
			TokenOutDecimals:      6,
			ExpectedPool:          "0xcb80b1a9e7b319a4c5ec6b0666967ca1e309e40f",
			SwapAmount:            "1000000000000000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
			ExpectedNoHops:        1,
		},
		{
			Name:                  "Arbitrum-Boosted(WETH/WSTETH)",
			Network:               "42161",
			TokenIn:               "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1", // WETH
			TokenOut:              "0x5979D7b546E38E414F7E9822514be443A4800529", // WSTETH
			TokenInDecimals:       18,
			TokenOutDecimals:      18,
			ExpectedPool:          "0xc072880e1bc0bcddc99db882c7f3e7a839281cf4",
			SwapAmount:            "10000000000000000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
			ExpectedNoHops:        1,
		},
		{
			Name:                  "Arbitrum-Boosted-StableSurge(GHO/USDC)",
			Network:               "42161",
			TokenIn:               "0x7dfF72693f6A4149b17e7C6314655f6A9F7c8B33", // GHO
			TokenOut:              "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", // USDC
			TokenInDecimals:       18,
			TokenOutDecimals:      6,
			ExpectedPool:          "0x19b001e6bc2d89154c18e2216eec5c8c6047b6d8",
			SwapAmount:            "1000000000000000000000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
			ExpectedNoHops:        1,
		},
		{
			Name:                  "Arbitrum-Boosted-GyroE(eBTC/USDC)",
			Network:               "42161",
			TokenIn:               "0x657e8C867D8B37dCC18fA4Caead9C45EB088C642", // eBTC
			TokenOut:              "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", // USDC
			TokenInDecimals:       8,
			TokenOutDecimals:      6,
			ExpectedPool:          "0xa0e5e7728e026bde02810d255a6b94a9aa47b5f9",
			ExpectedNoHops:        1,
			SwapAmount:            "1000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                  "Gnosis-Boosted",
			Network:               "100",
			TokenIn:               "0x6a023ccd1ff6f2045c3309768ead9e68f978f6e1", // WETH
			TokenOut:              "0x6c76971f98945ae98dd7d4dfca8711ebea946ea6", // wstETH
			TokenInDecimals:       18,
			TokenOutDecimals:      18,
			ExpectedPool:          "0x6e6bb18449fcf15b79efa2cfa70acf7593088029",
			SwapAmount:            "1000000000000000000",
			CheckInterval:         10,
			NotificationType:      "email",
			NotificationRecipient: "test@example.com",
			ExpectedNoHops:        1,
		},
	}

	// Generate endpoints by combining base configurations with route solvers
	for _, base := range baseEndpoints {
		for _, solver := range routeSolvers {
			// Skip disabled solvers
			if !solver.Enabled {
				continue
			}

			// Check if the solver supports this network
			supported := false
			for _, network := range solver.SupportedNetworks {
				if network == base.Network {
					supported = true
					break
				}
			}

			if !supported {
				continue // Skip unsupported network combinations
			}

			endpoint := Endpoint{
				Name:                  fmt.Sprintf("%s-%s", solver.Name, base.Name),
				BaseName:              base.Name,
				SolverName:            solver.Name,
				RouteSolver:           solver.Type,
				Network:               base.Network,
				TokenIn:               base.TokenIn,
				TokenOut:              base.TokenOut,
				TokenInDecimals:       base.TokenInDecimals,
				TokenOutDecimals:      base.TokenOutDecimals,
				SwapAmount:            base.SwapAmount,
				ExpectedPool:          base.ExpectedPool,
				ExpectedNoHops:        base.ExpectedNoHops,
				CheckInterval:         base.CheckInterval,
				LastStatus:            "unknown",
				LastChecked:           time.Time{},
				NotificationType:      base.NotificationType,
				NotificationRecipient: base.NotificationRecipient,
				Message:               "",
			}
			endpoints = append(endpoints, endpoint)
		}
	}

	go monitorAPIs(endpoints) // Start monitoring in the background
	sendEmail("Service starting")
	http.HandleFunc("/", dashboardHandler)
	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
