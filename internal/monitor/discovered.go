package monitor

import (
	"fmt"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
)

// RunDiscoveredOnce iterates the discovered-endpoints store and runs the same
// provider check pipeline as the hourly BaseEndpoints loop. Designed to be
// invoked from the discovery goroutine after each daily refresh so the test
// set is exercised at the discovery cadence — not the hourly one.
//
// Mirrors checkAllEndpoints' shape (Get copy, update by name, per-row delay)
// but reads/writes the discovered store.
func RunDiscoveredOnce() {
	eps := collector.GetDiscoveredEndpointsCopy()
	if len(eps) == 0 {
		fmt.Printf("%s[DISCOVERY RUN]%s no discovered test rows to check\n",
			config.ColorBlue, config.ColorReset)
		return
	}

	fmt.Printf("%s[DISCOVERY RUN]%s checking %d discovered test rows\n",
		config.ColorBlue, config.ColorReset, len(eps))

	for _, endpoint := range eps {
		name := endpoint.Name
		safeCheck(name, func() {
			collector.UpdateDiscoveredEndpointByName(name, func(e *collector.Endpoint) {
				CheckAPI(e, nil) // nil triggers Balancer-only + market price calls
			})
		})
		time.Sleep(endpoint.Delay)
	}

	fmt.Printf("%s[DISCOVERY RUN]%s finished checking %d rows\n",
		config.ColorGreen, config.ColorReset, len(eps))
}
