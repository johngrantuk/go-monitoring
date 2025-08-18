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
	case "kyberswap":
		checkKyberSwapAPI(endpoint)
	case "hyperbloom":
		checkHyperBloomAPI(endpoint)
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
		// Add delay between each endpoint check based on route solver
		delay := getDelayForRouteSolver(endpoints[i].RouteSolver)
		time.Sleep(delay)
	}

	// Check all endpoints when ticker triggers
	for range ticker.C {
		for i := range endpoints {
			checkAPI(&endpoints[i])
			// Add delay between each endpoint check based on route solver
			delay := getDelayForRouteSolver(endpoints[i].RouteSolver)
			time.Sleep(delay)
		}
	}
}

// getDelayForRouteSolver returns the appropriate delay for each route solver
func getDelayForRouteSolver(routeSolver string) time.Duration {
	switch routeSolver {
	case "kyberswap":
		return 60 * time.Second // Longer delay for Kyber endpoints
	case "hyperbloom":
		return 30 * time.Second // Medium delay for HyperBloom endpoints
	default:
		return 1 * time.Second // Default delay for other endpoints
	}
}
