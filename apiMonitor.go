package main

import (
	"fmt"
	"time"

	"go-monitoring/internal/collector"
)

// Function to handle unsupported route solvers
func checkUnsupportedAPI(endpoint *collector.Endpoint) {
	endpoint.LastChecked = time.Now()
	endpoint.LastStatus = "unsupported"
	fmt.Printf("Unsupported route solver '%s' for endpoint %s\n", endpoint.RouteSolver, endpoint.Name)
}

// Function to check API status based on route solver
func checkAPI(endpoint *collector.Endpoint) {
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
func monitorAPIs() {
	// Get the minimum check interval and create ticker outside the lock
	var minInterval int
	collector.WithEndpointsLock(func(endpoints []collector.Endpoint) {
		minInterval = endpoints[0].CheckInterval
		for _, endpoint := range endpoints {
			if endpoint.CheckInterval < minInterval {
				minInterval = endpoint.CheckInterval
			}
		}
	})

	ticker := time.NewTicker(time.Duration(minInterval) * time.Hour)
	defer ticker.Stop()

	// Perform initial checks immediately
	checkAllEndpoints()

	// Check all endpoints when ticker triggers
	for range ticker.C {
		checkAllEndpoints()
	}
}

// checkAllEndpoints performs API checks for all endpoints with minimal mutex locking
func checkAllEndpoints() {
	// Get a copy of endpoints to iterate over
	endpoints := collector.GetEndpointsCopy()

	// Do the actual API checks outside the lock
	for _, endpoint := range endpoints {
		collector.UpdateEndpointByName(endpoint.Name, func(endpoint *collector.Endpoint) {
			checkAPI(endpoint)
		})
		// Add delay between each endpoint check based on route solver
		delay := getDelayForRouteSolver(endpoint.RouteSolver)
		time.Sleep(delay)
	}
}

// getDelayForRouteSolver returns the appropriate delay for each route solver
func getDelayForRouteSolver(routeSolver string) time.Duration {
	switch routeSolver {
	case "kyberswap":
		return 120 * time.Second // Longer delay for Kyber endpoints
	case "hyperbloom":
		return 30 * time.Second // Medium delay for HyperBloom endpoints
	default:
		return 1 * time.Second // Default delay for other endpoints
	}
}
