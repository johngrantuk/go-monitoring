package monitor

import (
	"time"

	"go-monitoring/internal/collector"
)

// CheckAPI checks API status based on route solver
func CheckAPI(endpoint *collector.Endpoint, options *CheckOptions) {
	GlobalRegistry.CheckProvider(endpoint, options)
}

// MonitorAPIs periodically checks API status
func MonitorAPIs(checkIntervalHours int) {
	ticker := time.NewTicker(time.Duration(checkIntervalHours) * time.Hour)
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
			useIgnoreList := true
			CheckAPI(endpoint, &CheckOptions{UseIgnoreList: &useIgnoreList})
		})
		// Add delay between each endpoint check based on endpoint's configured delay
		time.Sleep(endpoint.Delay)
	}
}
