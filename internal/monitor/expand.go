package monitor

import (
	"fmt"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
)

// ExpandInput is the network-agnostic shape consumed by ExpandForSolvers.
// One ExpandInput corresponds to a single (pool, direction, variant)
// combination; the helper fans it out across every enabled route solver that
// supports the input's network.
type ExpandInput struct {
	BaseName         string
	Network          string
	TokenIn          string
	TokenOut         string
	TokenInDecimals  int
	TokenOutDecimals int
	SwapAmount       string
	ExpectedPool     string
	ExpectedNoHops   int
	PoolType         string // empty for BaseEndpoints rows
	HookType         string // empty for BaseEndpoints rows
	Variant          string // "" for base / registered; "underlying" for the boosted underlying row
}

// ExpandForSolvers cross-joins inputs with the enabled route solvers, keeping
// only the (input, solver) pairs the solver actually supports for the input's
// network. Returns the resulting flat slice of collector.Endpoint values.
//
// Shared between BaseEndpoints startup and discovery integration so the
// network-support filter cannot drift between the two code paths.
func ExpandForSolvers(inputs []ExpandInput) []collector.Endpoint {
	enabled := config.GetEnabledRouteSolvers()

	var out []collector.Endpoint
	for _, in := range inputs {
		for _, solver := range enabled {
			supported := false
			for _, n := range solver.SupportedNetworks {
				if n == in.Network {
					supported = true
					break
				}
			}
			if !supported {
				continue
			}
			out = append(out, collector.Endpoint{
				Name:             fmt.Sprintf("%s-%s", solver.Name, in.BaseName),
				BaseName:         in.BaseName,
				SolverName:       solver.Name,
				RouteSolver:      solver.Type,
				Network:          in.Network,
				TokenIn:          in.TokenIn,
				TokenOut:         in.TokenOut,
				TokenInDecimals:  in.TokenInDecimals,
				TokenOutDecimals: in.TokenOutDecimals,
				SwapAmount:       in.SwapAmount,
				ExpectedPool:     in.ExpectedPool,
				ExpectedNoHops:   in.ExpectedNoHops,
				Delay:            config.GetRouteSolverDelay(solver.Type),
				LastStatus:       "unknown",
				LastChecked:      time.Time{},
				Message:          "",
				PoolType:         in.PoolType,
				HookType:         in.HookType,
				Variant:          in.Variant,
			})
		}
	}
	return out
}
