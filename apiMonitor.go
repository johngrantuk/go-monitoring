package main

import (
	"fmt"
	"time"
)

// Global flags to enable/disable specific API checks
var (
	enableParaswapChecks = true
	enable1inchChecks    = false
	enable0xChecks       = true
)

// Function to handle unsupported route solvers
func checkUnsupportedAPI(endpoint *Endpoint) {
	mu.Lock()
	defer mu.Unlock()

	endpoint.LastChecked = time.Now()
	endpoint.LastStatus = "unsupported"
	fmt.Printf("Unsupported route solver '%s' for endpoint %s\n", endpoint.RouteSolver, endpoint.Name)
}

// Function to check API status based on route solver
func checkAPI(endpoint *Endpoint) {
	switch endpoint.RouteSolver {
	case "paraswap":
		if enableParaswapChecks {
			checkParaswapAPI(endpoint)
		} else {
			mu.Lock()
			endpoint.LastStatus = "disabled"
			endpoint.Message = "Paraswap checks are disabled"
			endpoint.LastChecked = time.Now()
			mu.Unlock()
			fmt.Printf("%s[INFO]%s %s: Paraswap checks are disabled\n", colorYellow, colorReset, endpoint.Name)
		}
	case "1inch":
		if enable1inchChecks {
			check1inchAPI(endpoint)
		} else {
			mu.Lock()
			endpoint.LastStatus = "disabled"
			endpoint.Message = "1inch checks are disabled"
			endpoint.LastChecked = time.Now()
			mu.Unlock()
			fmt.Printf("%s[INFO]%s %s: 1inch checks are disabled\n", colorYellow, colorReset, endpoint.Name)
		}
	case "0x":
		if enable0xChecks {
			check0xAPI(endpoint)
		} else {
			mu.Lock()
			endpoint.LastStatus = "disabled"
			endpoint.Message = "0x checks are disabled"
			endpoint.LastChecked = time.Now()
			mu.Unlock()
			fmt.Printf("%s[INFO]%s %s: 0x checks are disabled\n", colorYellow, colorReset, endpoint.Name)
		}
	default:
		checkUnsupportedAPI(endpoint)
	}
}

// Function to periodically check API status
func monitorAPIs(endpoints []Endpoint) {
	// Create tickers for each endpoint based on their check intervals
	for i := range endpoints {
		go func(endpoint *Endpoint) {
			// Perform initial check immediately
			checkAPI(endpoint)
			
			ticker := time.NewTicker(time.Duration(endpoint.CheckInterval) * time.Hour)
			defer ticker.Stop()

			for range ticker.C {
				checkAPI(endpoint)
			}
		}(&endpoints[i])
	}
} 