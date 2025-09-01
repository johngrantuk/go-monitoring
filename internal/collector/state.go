package collector

import (
	"sync"
	"time"
)

// Endpoint represents a monitored API endpoint
type Endpoint struct {
	Name             string
	BaseName         string
	SolverName       string
	RouteSolver      string
	Network          string
	TokenIn          string
	TokenOut         string
	TokenInDecimals  int
	TokenOutDecimals int
	SwapAmount       string
	ExpectedPool     string
	ExpectedNoHops   int
	Delay            time.Duration
	LastStatus       string
	LastChecked      time.Time
	Message          string
	ReturnAmount     string
}

var (
	endpoints []Endpoint
	mu        sync.Mutex
)

// WithEndpointsLock provides thread-safe access for writers (API checker functions)
func WithEndpointsLock(fn func([]Endpoint)) {
	mu.Lock()
	defer mu.Unlock()
	fn(endpoints)
}

// GetEndpointsCopy provides thread-safe access for readers (dashboard handler)
func GetEndpointsCopy() []Endpoint {
	mu.Lock()
	defer mu.Unlock()

	// Return a copy to avoid race conditions
	result := make([]Endpoint, len(endpoints))
	copy(result, endpoints)
	return result
}

// SetEndpoints initializes the endpoints slice
func SetEndpoints(eps []Endpoint) {
	mu.Lock()
	defer mu.Unlock()
	endpoints = eps
}

// GetEndpointByName returns a copy of a specific endpoint by name
func GetEndpointByName(name string) *Endpoint {
	mu.Lock()
	defer mu.Unlock()

	for i := range endpoints {
		if endpoints[i].Name == name {
			// Return a copy to avoid race conditions
			result := endpoints[i]
			return &result
		}
	}
	return nil
}

// UpdateEndpointByName updates a specific endpoint by name
func UpdateEndpointByName(name string, fn func(*Endpoint)) bool {
	mu.Lock()
	defer mu.Unlock()

	for i := range endpoints {
		if endpoints[i].Name == name {
			fn(&endpoints[i])
			return true
		}
	}
	return false
}
