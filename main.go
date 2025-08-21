package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"go-monitoring/config"
	"go-monitoring/notifications"
)

// Endpoint represents a monitored API endpoint
type Endpoint struct {
	Name                  string
	BaseName              string
	SolverName            string
	RouteSolver           string
	Network               string
	TokenIn               string
	TokenOut              string
	TokenInDecimals       int
	TokenOutDecimals      int
	SwapAmount            string
	ExpectedPool          string
	ExpectedNoHops        int
	CheckInterval         int
	LastStatus            string
	LastChecked           time.Time
	NotificationType      string
	NotificationRecipient string
	Message               string
}

// API monitoring data
var (
	endpoints = make([]Endpoint, 0)
	mu        sync.Mutex // Prevents race conditions
)

func main() {
	// Generate endpoints by combining base configurations with route solvers
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

			endpoint := Endpoint{
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
			endpoints = append(endpoints, endpoint)
		}
	}

	go monitorAPIs(endpoints) // Start monitoring in the background
	notifications.SendEmail("Service starting")
	http.HandleFunc("/", dashboardHandler)
	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
