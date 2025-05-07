package main

import (
	"fmt"
	"time"
)

// Global flags to enable/disable specific API checks
var (
	enableEmailSending = true
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
		checkParaswapAPI(endpoint)
	case "1inch":
		check1inchAPI(endpoint)
	case "0x":
		check0xAPI(endpoint)
	case "odos":
		checkOdosAPI(endpoint)
	default:
		checkUnsupportedAPI(endpoint)
	}
}

// Function to periodically check API status
func monitorAPIs(endpoints []Endpoint) {
	// Create a single ticker for all endpoints
	// Use the minimum check interval from all endpoints
	minInterval := endpoints[0].CheckInterval
	for _, endpoint := range endpoints {
		if endpoint.CheckInterval < minInterval {
			minInterval = endpoint.CheckInterval
		}
	}

	ticker := time.NewTicker(time.Duration(minInterval) * time.Hour)
	defer ticker.Stop()

	// Perform initial checks immediately
	for i := range endpoints {
		checkAPI(&endpoints[i])
		// Add 5 second delay between each endpoint check
		time.Sleep(2 * time.Second)
	}

	// Check all endpoints when ticker triggers
	for range ticker.C {
		for i := range endpoints {
			checkAPI(&endpoints[i])
			// Add 5 second delay between each endpoint check
			time.Sleep(1 * time.Second)
		}
	}
}
