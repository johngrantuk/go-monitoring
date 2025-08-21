package main

import (
	"fmt"
	"net/http"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
	"go-monitoring/internal/monitor"
	"go-monitoring/notifications"
)

func main() {
	// Generate endpoints by combining base configurations with route solvers
	var generatedEndpoints []collector.Endpoint
	for _, base := range config.BaseEndpoints {
		for _, solver := range config.RouteSolvers {
			// Skip disabled solvers
			if !solver.Enabled {
				continue
			}

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
				Name:                  fmt.Sprintf("%s-%s", solver.Name, base.Name),
				BaseName:              base.Name,
				SolverName:            solver.Name,
				RouteSolver:           solver.Type,
				Network:               base.Network,
				TokenIn:               base.TokenIn,
				TokenOut:              base.TokenOut,
				TokenInDecimals:       base.TokenInDecimals,
				TokenOutDecimals:      base.TokenOutDecimals,
				SwapAmount:            base.SwapAmount,
				ExpectedPool:          base.ExpectedPool,
				ExpectedNoHops:        base.ExpectedNoHops,
				CheckInterval:         base.CheckInterval,
				LastStatus:            "unknown",
				LastChecked:           time.Time{},
				NotificationType:      base.NotificationType,
				NotificationRecipient: base.NotificationRecipient,
				Message:               "",
			}
			generatedEndpoints = append(generatedEndpoints, endpoint)
		}
	}

	// Initialize the collector with the generated endpoints
	collector.SetEndpoints(generatedEndpoints)

	go monitor.MonitorAPIs() // Start monitoring in the background
	notifications.SendEmail("Service starting")
	http.HandleFunc("/", dashboardHandler)
	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
