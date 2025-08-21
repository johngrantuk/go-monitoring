package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"go-monitoring/config"
	"go-monitoring/handlers"
	"go-monitoring/internal/collector"
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

	// Generate endpoints by combining base configurations with route solvers
	var generatedEndpoints []collector.Endpoint
	for _, base := range config.BaseEndpoints {
		for _, solver := range config.GetEnabledRouteSolvers() {
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

			endpoint := collector.Endpoint{
				Name:             fmt.Sprintf("%s-%s", solver.Name, base.Name),
				BaseName:         base.Name,
				SolverName:       solver.Name,
				RouteSolver:      solver.Type,
				Network:          base.Network,
				TokenIn:          base.TokenIn,
				TokenOut:         base.TokenOut,
				TokenInDecimals:  base.TokenInDecimals,
				TokenOutDecimals: base.TokenOutDecimals,
				SwapAmount:       base.SwapAmount,
				ExpectedPool:     base.ExpectedPool,
				ExpectedNoHops:   base.ExpectedNoHops,
				Delay:            config.GetRouteSolverDelay(solver.Type),
				LastStatus:       "unknown",
				LastChecked:      time.Time{},
				Message:          "",
			}
			generatedEndpoints = append(generatedEndpoints, endpoint)
		}
	}

	// Initialize the collector with the generated endpoints
	collector.SetEndpoints(generatedEndpoints)

	// Get check interval from environment variable in main thread
	checkIntervalHours := getCheckIntervalHours()

	go monitor.MonitorAPIs(checkIntervalHours) // Start monitoring in the background
	notifications.SendEmail("Service starting")

	// Register HTTP handlers
	http.HandleFunc("/", handlers.DashboardHandler)
	http.HandleFunc("/check/", handlers.CheckEndpointHandler)

	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
