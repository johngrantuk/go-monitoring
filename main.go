package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Endpoint represents a monitored API endpoint
type Endpoint struct {
	Name                string
	RouteSolver         string
	Network             string
	TokenIn             string
	TokenInDecimals     int
	TokenOut            string
	TokenOutDecimals    int
	SwapAmount          string
	ExpectedPool        string
	ExpectedNoHops      int    // Number of hops expected in the route (0 for direct swap, 1 for one intermediate token, etc.)
	CheckInterval       int    // minutes
	LastStatus          string // "up", "down", or "unknown"
	LastChecked         time.Time
	NotificationType    string // "email" or "slack"
	NotificationRecipient string
	Message             string // Status message for the last check
}

// API monitoring data
var (
	endpoints = make([]Endpoint, 0)
	mu        sync.Mutex // Prevents race conditions
)

func main() {
	// Initialize endpoints
	endpoints = []Endpoint{
		{
			Name:                "Paraswap-Mainet-Boosted",
			RouteSolver:         "paraswap",
			Network:             "1",
			TokenIn:             "0x40d16fc0246ad3160ccc09b8d0d3a2cd28ae6c2f", // GHO
			TokenOut:            "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", // USDC
			TokenInDecimals:     18,
			TokenOutDecimals:    6,
			ExpectedPool:        "0x85b2b559bc2d21104c4defdd6efca8a20343361d",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "Paraswap-Mainet-Boosted-StableSurge",
			RouteSolver:         "paraswap",
			Network:             "1",
			TokenIn:             "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0", // wstETH
			TokenOut:            "0xd11c452fc99cf405034ee446803b6f6c1f6d5ed8", // tETH
			TokenInDecimals:     18,
			TokenOutDecimals:    18,
			ExpectedPool:        "0x9ed5175aecb6653c1bdaa19793c16fd74fbeeb37",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "Paraswap-Base-Boosted",
			RouteSolver:         "paraswap",
			Network:             "8453",
			TokenIn:             "0x4200000000000000000000000000000000000006", // weth
			TokenOut:            "0xc1CBa3fCea344f92D9239c08C0568f6F2F0ee452", // wstETH
			TokenInDecimals:     18,
			TokenOutDecimals:    18,
			ExpectedPool:        "0xacba78d745faae7751c09489d5f15a26eb27f1ad",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "Paraswap-Base-Boosted-StableSurge",
			RouteSolver:         "paraswap",
			Network:             "8453",
			TokenIn:             "0x6Bb7a212910682DCFdbd5BCBb3e28FB4E8da10Ee", // GHO
			TokenOut:            "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC
			TokenInDecimals:     18,
			TokenOutDecimals:    6,
			ExpectedPool:        "0x7ab124ec4029316c2a42f713828ddf2a192b36db",
			SwapAmount:          "10000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "Paraswap-Base-Boosted-StableSurge-WETH/USDC",
			RouteSolver:         "paraswap",
			Network:             "8453",
			TokenIn:             "0x4200000000000000000000000000000000000006", // WETH
			TokenOut:            "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC
			TokenInDecimals:     18,
			TokenOutDecimals:    6,
			ExpectedPool:        "0xcb80b1a9e7b319a4c5ec6b0666967ca1e309e40f",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "Paraswap-Arbitrum-Boosted",
			RouteSolver:         "paraswap",
			Network:             "42161",
			TokenIn:             "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1", // WETH
			TokenOut:            "0x5979D7b546E38E414F7E9822514be443A4800529", // WSTETH
			TokenInDecimals:     18,
			TokenOutDecimals:    18,
			ExpectedPool:        "0xc072880e1bc0bcddc99db882c7f3e7a839281cf4",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "Paraswap-Arbitrum-Boosted-StableSurge",
			RouteSolver:         "paraswap",
			Network:             "42161",
			TokenIn:             "0x7dfF72693f6A4149b17e7C6314655f6A9F7c8B33", // GHO
			TokenOut:            "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", // USDC
			TokenInDecimals:     18,
			TokenOutDecimals:    6,
			ExpectedPool:        "0x19b001e6bc2d89154c18e2216eec5c8c6047b6d8",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "Paraswap-Arbitrum-Boosted-GyroE",
			RouteSolver:         "paraswap",
			Network:             "42161",
			TokenIn:             "0x657e8C867D8B37dCC18fA4Caead9C45EB088C642", // eBTC
			TokenOut:            "0x35751007a407ca6FEFfE80b3cB397736D2cf4dbe", // weETH
			TokenInDecimals:     8,
			TokenOutDecimals:    18,
			ExpectedPool:        "0x2d39c2ddf0ae652071d0550718cc7aacaf647d39",
			SwapAmount:          "10000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "Paraswap-Gnosis-Boosted",
			RouteSolver:         "paraswap",
			Network:             "100",
			TokenIn:             "0x6a023ccd1ff6f2045c3309768ead9e68f978f6e1", // WETH
			TokenOut:            "0x6c76971f98945ae98dd7d4dfca8711ebea946ea6", // wstETH
			TokenInDecimals:     18,
			TokenOutDecimals:    18,
			ExpectedPool:        "0x6e6bb18449fcf15b79efa2cfa70acf7593088029",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		////////////////////////////////////////// 1inch
		{
			Name:                "1inch-Mainnet-Boosted",
			RouteSolver:         "1inch",
			Network:             "1",
			TokenIn:             "0x40d16fc0246ad3160ccc09b8d0d3a2cd28ae6c2f", // GHO
			TokenOut:            "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", // USDC
			ExpectedPool:        "0x85b2b559bc2d21104c4defdd6efca8a20343361d",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "1inch-Base-Boosted",
			RouteSolver:         "1inch",
			Network:             "8453",
			TokenIn:             "0x4200000000000000000000000000000000000006", // weth
			TokenOut:            "0xc1CBa3fCea344f92D9239c08C0568f6F2F0ee452", // wstETH
			ExpectedPool:        "0xacba78d745faae7751c09489d5f15a26eb27f1ad",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "1inch-Base-Boosted-StableSurge",
			RouteSolver:         "1inch",
			Network:             "8453",
			TokenIn:             "0x6Bb7a212910682DCFdbd5BCBb3e28FB4E8da10Ee", // GHO
			TokenOut:            "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC
			ExpectedPool:        "0x7ab124ec4029316c2a42f713828ddf2a192b36db",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "1inch-Arbitrum-Boosted",
			RouteSolver:         "1inch",
			Network:             "42161",
			TokenIn:             "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1", // WETH
			TokenOut:            "0x5979D7b546E38E414F7E9822514be443A4800529", // WSTETH
			ExpectedPool:        "0xc072880e1bc0bcddc99db882c7f3e7a839281cf4",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "1inch-Arbitrum-Boosted-StableSurge",
			RouteSolver:         "1inch",
			Network:             "42161",
			TokenIn:             "0x7dfF72693f6A4149b17e7C6314655f6A9F7c8B33", // GHO
			TokenOut:            "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", // USDC
			ExpectedPool:        "0x19b001e6bc2d89154c18e2216eec5c8c6047b6d8",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "1inch-Arbitrum-Boosted-GyroE",
			RouteSolver:         "1inch",
			Network:             "42161",
			TokenIn:             "0x657e8C867D8B37dCC18fA4Caead9C45EB088C642", // eBTC
			TokenOut:            "0x35751007a407ca6FEFfE80b3cB397736D2cf4dbe", // weETH
			ExpectedPool:        "0x2d39c2ddf0ae652071d0550718cc7aacaf647d39",
			SwapAmount:          "10000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "1inch-Gnosis-Boosted",
			RouteSolver:         "1inch",
			Network:             "100",
			TokenIn:             "0x6a023ccd1ff6f2045c3309768ead9e68f978f6e1", // WETH
			TokenOut:            "0x6c76971f98945ae98dd7d4dfca8711ebea946ea6", // wstETH
			ExpectedPool:        "0x6e6bb18449fcf15b79efa2cfa70acf7593088029",
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		////////////////////////////////////////// 0x
		{
			Name:                "0x-Mainet-Boosted",
			RouteSolver:         "0x",
			Network:             "1",
			TokenIn:             "0x40d16fc0246ad3160ccc09b8d0d3a2cd28ae6c2f", // GHO
			TokenOut:            "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", // USDC
			TokenInDecimals:     18,
			TokenOutDecimals:    6,
			ExpectedPool:        "0x85b2b559bc2d21104c4defdd6efca8a20343361d",
			ExpectedNoHops:      1,
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "0x-Mainet-Boosted-StableSurge",
			RouteSolver:         "0x",
			Network:             "1",
			TokenIn:             "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0", // wstETH
			TokenOut:            "0xd11c452fc99cf405034ee446803b6f6c1f6d5ed8", // tETH
			TokenInDecimals:     18,
			TokenOutDecimals:    18,
			ExpectedPool:        "0x9ed5175aecb6653c1bdaa19793c16fd74fbeeb37",
			ExpectedNoHops:      1,
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "0x-Base-Boosted",
			RouteSolver:         "0x",
			Network:             "8453",
			TokenIn:             "0x4200000000000000000000000000000000000006", // weth
			TokenOut:            "0xc1CBa3fCea344f92D9239c08C0568f6F2F0ee452", // wstETH
			TokenInDecimals:     18,
			TokenOutDecimals:    18,
			ExpectedPool:        "0xacba78d745faae7751c09489d5f15a26eb27f1ad",
			ExpectedNoHops:      1,
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "0x-Base-Boosted-StableSurge",
			RouteSolver:         "0x",
			Network:             "8453",
			TokenIn:             "0x6Bb7a212910682DCFdbd5BCBb3e28FB4E8da10Ee", // GHO
			TokenOut:            "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC
			TokenInDecimals:     18,
			TokenOutDecimals:    6,
			ExpectedPool:        "0x7ab124ec4029316c2a42f713828ddf2a192b36db",
			ExpectedNoHops:      1,
			SwapAmount:          "10000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "0x-Base-Boosted-StableSurge-WETH/USDC",
			RouteSolver:         "0x",
			Network:             "8453",
			TokenIn:             "0x4200000000000000000000000000000000000006", // WETH
			TokenOut:            "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC
			TokenInDecimals:     18,
			TokenOutDecimals:    6,
			ExpectedPool:        "0xcb80b1a9e7b319a4c5ec6b0666967ca1e309e40f",
			ExpectedNoHops:      1,
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "0x-Arbitrum-Boosted",
			RouteSolver:         "0x",
			Network:             "42161",
			TokenIn:             "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1", // WETH
			TokenOut:            "0x5979D7b546E38E414F7E9822514be443A4800529", // WSTETH
			TokenInDecimals:     18,
			TokenOutDecimals:    18,
			ExpectedPool:        "0xc072880e1bc0bcddc99db882c7f3e7a839281cf4",
			ExpectedNoHops:      1,
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "0x-Arbitrum-Boosted-StableSurge",
			RouteSolver:         "0x",
			Network:             "42161",
			TokenIn:             "0x7dfF72693f6A4149b17e7C6314655f6A9F7c8B33", // GHO
			TokenOut:            "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", // USDC
			TokenInDecimals:     18,
			TokenOutDecimals:    6,
			ExpectedPool:        "0x19b001e6bc2d89154c18e2216eec5c8c6047b6d8",
			ExpectedNoHops:      1,
			SwapAmount:          "1000000000000000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
		{
			Name:                "0x-Arbitrum-Boosted-GyroE",
			RouteSolver:         "0x",
			Network:             "42161",
			TokenIn:             "0x657e8C867D8B37dCC18fA4Caead9C45EB088C642", // eBTC
			TokenOut:            "0x35751007a407ca6FEFfE80b3cB397736D2cf4dbe", // weETH
			TokenInDecimals:     8,
			TokenOutDecimals:    18,
			ExpectedPool:        "0x2d39c2ddf0ae652071d0550718cc7aacaf647d39",
			ExpectedNoHops:      1,
			SwapAmount:          "10000000",
			CheckInterval:       10,
			LastStatus:          "unknown",
			LastChecked:         time.Time{},
			NotificationType:    "email",
			NotificationRecipient: "test@example.com",
		},
	}

	go monitorAPIs(endpoints) // Start monitoring in the background

	http.HandleFunc("/", dashboardHandler)
	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
