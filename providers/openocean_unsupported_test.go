package providers

import (
	"errors"
	"fmt"
	"testing"

	"go-monitoring/internal/api"
)

func TestOpenOceanNoBalancerV3ErrorWrapsUnsupported(t *testing.T) {
	err := fmt.Errorf("OpenOcean has no Balancer V3 in dexList for chain %q: %w", "base", api.ErrBuildURLUnsupported)
	if !errors.Is(err, api.ErrBuildURLUnsupported) {
		t.Fatalf("expected errors.Is(ErrBuildURLUnsupported): %v", err)
	}
}
