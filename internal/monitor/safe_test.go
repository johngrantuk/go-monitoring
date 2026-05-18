package monitor

import (
	"strings"
	"testing"

	"go-monitoring/internal/collector"
)

// TestSafeCheck_RecoversAndRecordsStatus verifies the two contracts of
// safeCheck: (a) a panicking handler does not propagate, and (b) when the
// panicked endpoint exists in one of the stores, its status is updated to
// "panic" with the recovered value in Message so the dashboard surfaces
// the failure instead of silently keeping a stale verdict.
func TestSafeCheck_RecoversAndRecordsStatus(t *testing.T) {
	t.Run("recovers from panic without propagating", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("safeCheck propagated panic: %v", r)
			}
		}()
		safeCheck("nonexistent-endpoint", func() {
			panic("simulated handler boom")
		})
	})

	t.Run("records panic on the base endpoint store when the name matches", func(t *testing.T) {
		const name = "panic-test-base"
		collector.SetEndpoints([]collector.Endpoint{{Name: name, LastStatus: "up"}})

		safeCheck(name, func() {
			panic("base boom")
		})

		got := collector.GetEndpointByName(name)
		if got == nil {
			t.Fatalf("endpoint %q missing after safeCheck", name)
		}
		if got.LastStatus != "panic" {
			t.Fatalf("LastStatus=%q, want %q", got.LastStatus, "panic")
		}
		if !strings.Contains(got.Message, "base boom") {
			t.Fatalf("Message=%q, want it to contain %q", got.Message, "base boom")
		}

		collector.SetEndpoints(nil)
	})

	t.Run("records panic on the discovered endpoint store when the name matches", func(t *testing.T) {
		const name = "panic-test-discovered"
		collector.SetDiscoveredEndpoints([]collector.Endpoint{{Name: name, LastStatus: "up"}}, nil)

		safeCheck(name, func() {
			panic("discovered boom")
		})

		eps := collector.GetDiscoveredEndpointsCopy()
		var got *collector.Endpoint
		for i := range eps {
			if eps[i].Name == name {
				got = &eps[i]
				break
			}
		}
		if got == nil {
			t.Fatalf("discovered endpoint %q missing after safeCheck", name)
		}
		if got.LastStatus != "panic" {
			t.Fatalf("LastStatus=%q, want %q", got.LastStatus, "panic")
		}
		if !strings.Contains(got.Message, "discovered boom") {
			t.Fatalf("Message=%q, want it to contain %q", got.Message, "discovered boom")
		}

		collector.SetDiscoveredEndpoints(nil, nil)
	})
}
