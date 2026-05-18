package collector

import (
	"strings"
	"sync"
	"time"
)

// Endpoint represents a monitored API endpoint
type Endpoint struct {
	Name              string
	BaseName          string
	SolverName        string
	RouteSolver       string
	Network           string
	TokenIn           string
	TokenOut          string
	TokenInDecimals   int
	TokenOutDecimals  int
	SwapAmount        string
	ExpectedPool      string
	ExpectedNoHops    int
	Delay             time.Duration
	LastStatus        string
	LastChecked       time.Time
	Message           string
	ReturnAmount      string
	MarketPrice       string
	OnChainPrice      string
	OnChainQueryError string // Error message if on-chain query failed
	SwapPathPools     []string
	SwapPathTokenOut  []string
	SwapPathIsBuffer  []bool
	// Discovered-only metadata. Empty for BaseEndpoints rows.
	PoolType string // Balancer API pool type enum (e.g. "STABLE", "GYROE")
	HookType string // Balancer API hook type, empty when no hook
	Variant  string // "" for base / registered; "underlying" for the boosted underlying row
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

// ----------------------------------------------------------------------------
// Discovered-endpoints store
//
// Lives alongside the BaseEndpoints store but with its own mutex so the hourly
// price-check loop and the daily discovery loop never block each other. Fully
// replaced on each discovery cycle, with per-row result carry-over for keys
// that survive.
// ----------------------------------------------------------------------------

var (
	discoveredEndpoints []Endpoint
	discoveredMu        sync.Mutex
	inTestSet           = map[string]struct{}{}
)

// SetDiscoveredEndpoints replaces the discovered store. Surviving rows
// (matched by Endpoint.Name) keep their prior result fields so the dashboard
// keeps showing yesterday's verdict until this cycle's run overwrites it.
// poolKeys is the (network, poolAddress) set used by IsPoolInTestSet.
func SetDiscoveredEndpoints(eps []Endpoint, poolKeys map[string]struct{}) {
	discoveredMu.Lock()
	defer discoveredMu.Unlock()

	prior := make(map[string]Endpoint, len(discoveredEndpoints))
	for _, e := range discoveredEndpoints {
		prior[e.Name] = e
	}

	merged := make([]Endpoint, len(eps))
	for i, e := range eps {
		if p, ok := prior[e.Name]; ok {
			e.LastStatus = p.LastStatus
			e.LastChecked = p.LastChecked
			e.Message = p.Message
			e.ReturnAmount = p.ReturnAmount
			e.MarketPrice = p.MarketPrice
			e.OnChainPrice = p.OnChainPrice
			e.OnChainQueryError = p.OnChainQueryError
			e.SwapPathPools = p.SwapPathPools
			e.SwapPathTokenOut = p.SwapPathTokenOut
			e.SwapPathIsBuffer = p.SwapPathIsBuffer
		} else if e.LastStatus == "" {
			e.LastStatus = "unknown"
		}
		merged[i] = e
	}
	discoveredEndpoints = merged

	if poolKeys == nil {
		inTestSet = map[string]struct{}{}
	} else {
		inTestSet = poolKeys
	}
}

// GetDiscoveredEndpointsCopy returns a copy of the discovered endpoints slice.
func GetDiscoveredEndpointsCopy() []Endpoint {
	discoveredMu.Lock()
	defer discoveredMu.Unlock()
	result := make([]Endpoint, len(discoveredEndpoints))
	copy(result, discoveredEndpoints)
	return result
}

// UpdateDiscoveredEndpointByName mirrors UpdateEndpointByName for the
// discovered store.
func UpdateDiscoveredEndpointByName(name string, fn func(*Endpoint)) bool {
	discoveredMu.Lock()
	defer discoveredMu.Unlock()

	for i := range discoveredEndpoints {
		if discoveredEndpoints[i].Name == name {
			fn(&discoveredEndpoints[i])
			return true
		}
	}
	return false
}

// IsPoolInTestSet reports whether the given pool (by network + address)
// landed in the current test set. Used by /pools to render the "In test set"
// badge without recomputing selection.
func IsPoolInTestSet(network, poolAddress string) bool {
	key := PoolKey(network, poolAddress)
	discoveredMu.Lock()
	defer discoveredMu.Unlock()
	_, ok := inTestSet[key]
	return ok
}

// PoolKey is the canonical key shape for the in-test-set lookup. Exposed so
// callers building the set use the same casing rules as the lookup.
func PoolKey(network, poolAddress string) string {
	return strings.ToLower(network) + "|" + strings.ToLower(poolAddress)
}
