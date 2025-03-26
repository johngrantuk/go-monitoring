package main

import (
	"fmt"
	"time"
)

// Global flags to enable/disable specific API checks
var (
	enableParaswapChecks = true
	enable1inchChecks    = false
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
		}
	case "1inch":
		if enable1inchChecks {
			check1inchAPI(endpoint)
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