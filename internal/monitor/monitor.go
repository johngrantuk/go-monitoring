package monitor

import (
	"fmt"
	"time"

	"go-monitoring/internal/collector"
	"go-monitoring/providers"
)

// CheckUnsupportedAPI handles unsupported route solvers
func CheckUnsupportedAPI(endpoint *collector.Endpoint) {
	endpoint.LastChecked = time.Now()
	endpoint.LastStatus = "unsupported"
	fmt.Printf("Unsupported route solver '%s' for endpoint %s\n", endpoint.RouteSolver, endpoint.Name)
}

// CheckAPI checks API status based on route solver
func CheckAPI(endpoint *collector.Endpoint) {
	switch endpoint.RouteSolver {
	case "paraswap":
		providers.CheckParaswapAPI(endpoint)
	case "1inch":
		providers.Check1inchAPI(endpoint)
	case "0x":
		providers.Check0xAPI(endpoint)
	case "odos":
		providers.CheckOdosAPI(endpoint)
	case "kyberswap":
		providers.CheckKyberSwapAPI(endpoint)
	case "hyperbloom":
		providers.CheckHyperBloomAPI(endpoint)
	default:
		CheckUnsupportedAPI(endpoint)
	}
}

// MonitorAPIs periodically checks API status
func MonitorAPIs(checkIntervalHours int) {
	ticker := time.NewTicker(time.Duration(checkIntervalHours) * time.Hour)
	defer ticker.Stop()

	// Perform initial checks immediately
	CheckAllEndpoints()

	// Check all endpoints when ticker triggers
	for range ticker.C {
		CheckAllEndpoints()
	}
}

// CheckAllEndpoints performs API checks for all endpoints with minimal mutex locking
func CheckAllEndpoints() {
	// Get a copy of endpoints to iterate over
	endpoints := collector.GetEndpointsCopy()

	// Do the actual API checks outside the lock
	for _, endpoint := range endpoints {
		collector.UpdateEndpointByName(endpoint.Name, func(endpoint *collector.Endpoint) {
			CheckAPI(endpoint)
		})
		// Add delay between each endpoint check based on endpoint's configured delay
		time.Sleep(endpoint.Delay)
	}
}
