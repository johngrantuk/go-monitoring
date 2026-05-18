package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"go-monitoring/config"
	"go-monitoring/handlers"
	"go-monitoring/internal/collector"
	"go-monitoring/internal/discovery"
	"go-monitoring/internal/monitor"
	"go-monitoring/notifications"

	"github.com/joho/godotenv"
)

// getCheckIntervalHours returns the check interval in hours from environment variable
// Defaults to 1 hour if not set or invalid
func getCheckIntervalHours() int {
	envValue := os.Getenv("CHECK_INTERVAL_HOURS")
	if envValue == "" {
		return 1 // Default to 1 hour
	}

	interval, err := strconv.Atoi(envValue)
	if err != nil || interval <= 0 {
		return 1 // Default to 1 hour if invalid
	}

	return interval
}

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist, just log it
		fmt.Println("No .env file found, using system environment variables")
	}

	// Expand BaseEndpoints across every enabled route solver that supports
	// the endpoint's network. Shared with the discovered test set builder so
	// the network-support filter cannot drift between the two paths.
	baseInputs := make([]monitor.ExpandInput, 0, len(config.BaseEndpoints))
	for _, base := range config.BaseEndpoints {
		baseInputs = append(baseInputs, monitor.ExpandInput{
			BaseName:         base.Name,
			Network:          base.Network,
			TokenIn:          base.TokenIn,
			TokenOut:         base.TokenOut,
			TokenInDecimals:  base.TokenInDecimals,
			TokenOutDecimals: base.TokenOutDecimals,
			SwapAmount:       base.SwapAmount,
			ExpectedPool:     base.ExpectedPool,
			ExpectedNoHops:   base.ExpectedNoHops,
		})
	}
	collector.SetEndpoints(monitor.ExpandForSolvers(baseInputs))

	// Initialize the provider registry
	monitor.InitializeRegistry()

	// Get check interval from environment variable in main thread
	checkIntervalHours := getCheckIntervalHours()
	discoveryIntervalHours := config.GetDiscoveryIntervalHours()

	// Register the discovered test set runner before starting discovery so the
	// first refresh's results are exercised against the providers.
	discovery.SetTestSetRunner(monitor.RunDiscoveredOnce)

	go monitor.MonitorAPIs(checkIntervalHours) // Start monitoring in the background
	go discovery.Run(discoveryIntervalHours)   // Start Balancer V3 pool discovery
	notifications.SendEmail("Service starting")

	// Register HTTP handlers
	http.HandleFunc("/", handlers.DashboardHandler)
	http.HandleFunc("/check/", handlers.CheckEndpointHandler)
	http.HandleFunc("/pools", handlers.PoolsHandler)

	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
