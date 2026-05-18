package monitor

import (
	"fmt"
	"runtime/debug"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
)

// safeCheck runs a single per-endpoint check with panic recovery so a bug in
// one provider handler can't kill the surrounding sweep. The endpoint's
// status is set to a generic "panic" sentinel with the recovered value as
// Message, mirroring how down-paths already record provider failures.
//
// Both the BaseEndpoints sweep (checkAllEndpoints) and the discovered sweep
// (RunDiscoveredOnce) use this so neither can be taken out by a single bad
// row.
func safeCheck(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			fmt.Printf("%s[CHECK PANIC]%s %s: %v\n%s\n",
				config.ColorRed, config.ColorReset, name, r, stack)
			collector.UpdateEndpointByName(name, recordPanic(r))
			collector.UpdateDiscoveredEndpointByName(name, recordPanic(r))
		}
	}()
	fn()
}

// recordPanic returns an updater that marks an endpoint as panicked so the
// dashboard surfaces the failure rather than silently keeping yesterday's
// status. Used by safeCheck against both stores; one of the two lookups is a
// no-op depending on which sweep the panic happened in.
func recordPanic(r any) func(*collector.Endpoint) {
	return func(e *collector.Endpoint) {
		e.LastStatus = "panic"
		e.Message = fmt.Sprintf("provider handler panicked: %v", r)
	}
}
