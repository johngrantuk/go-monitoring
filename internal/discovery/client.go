package discovery

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go-monitoring/config"
)

// balancerAPIEndpoint is the hardcoded Balancer V3 GraphQL endpoint. No env
// override; see docs/discovery.md.
const balancerAPIEndpoint = "https://api-v3.balancer.fi/graphql"

// maxFetchAttempts is the total number of attempts a single network gets per
// runOnce. Cold-cache responses from the Balancer GraphQL API occasionally
// exceed the per-request timeout; in practice a retry succeeds because the
// first attempt has already warmed the backend cache.
const maxFetchAttempts = 3

// poolsQuery is the GraphQL query used to fetch V3 pools for a set of chains.
// Field names verified against the live schema (api-v3.balancer.fi/graphql).
const poolsQuery = `query Pools($chainIn: [GqlChain!]!) {
  poolGetPools(where: {chainIn: $chainIn, protocolVersionIn: [3]}, first: 1000) {
    id
    address
    name
    symbol
    type
    chain
    hook {
      address
      type
      params {
        ... on StableSurgeHookParams {
          maxSurgeFeePercentage
          surgeThresholdPercentage
        }
      }
    }
    dynamicData {
      isPaused
      isInRecoveryMode
      totalLiquidity
      swapFee
      volume24h
    }
    poolTokens {
      address
      symbol
      decimals
      balance
      balanceUSD
      underlyingToken { address symbol decimals }
    }
  }
}`

// rawPool mirrors the GraphQL response shape. All numeric fields with the
// BigDecimal scalar are strings on the wire.
type rawPool struct {
	ID          string   `json:"id"`
	Address     string   `json:"address"`
	Name        string   `json:"name"`
	Symbol      string   `json:"symbol"`
	Type        string   `json:"type"`
	Chain       string   `json:"chain"`
	Hook        *rawHook `json:"hook"`
	DynamicData struct {
		IsPaused         bool   `json:"isPaused"`
		IsInRecoveryMode bool   `json:"isInRecoveryMode"`
		TotalLiquidity   string `json:"totalLiquidity"`
		SwapFee          string `json:"swapFee"`
		Volume24h        string `json:"volume24h"`
	} `json:"dynamicData"`
	PoolTokens []rawPoolToken `json:"poolTokens"`
}

type rawHook struct {
	Address string         `json:"address"`
	Type    string         `json:"type"`
	Params  *rawHookParams `json:"params"`
}

// rawHookParams is the GraphQL `HookParams` union flattened to the fields we
// care about. Non-StableSurge hooks leave the surge fields as empty strings.
type rawHookParams struct {
	MaxSurgeFeePercentage    string `json:"maxSurgeFeePercentage"`
	SurgeThresholdPercentage string `json:"surgeThresholdPercentage"`
}

type rawPoolToken struct {
	Address         string              `json:"address"`
	Symbol          string              `json:"symbol"`
	Decimals        int                 `json:"decimals"`
	Balance         string              `json:"balance"`
	BalanceUSD      string              `json:"balanceUSD"`
	UnderlyingToken *rawUnderlyingToken `json:"underlyingToken"`
}

type rawUnderlyingToken struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data struct {
		PoolGetPools []rawPool `json:"poolGetPools"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// httpClient is reused across calls; 60s timeout per plan.
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

// fetchPoolsWithRetry runs fetchPools up to maxFetchAttempts times, sleeping
// 5s × attempt between retries. Returns on the first success, or the last
// error if every attempt fails. Each retry is logged so operators can tell
// from the trace that a transient cold-cache slowdown was recovered. The
// Balancer GraphQL API can take many seconds to respond when its server-side
// price cache is cold (TVL / balanceUSD require live price lookups), so a
// single attempt occasionally exceeds the 60s per-request timeout; the next
// attempt typically lands on a warm cache and succeeds in well under a second.
func fetchPoolsWithRetry(chainEnum string) ([]rawPool, error) {
	var lastErr error
	for attempt := 1; attempt <= maxFetchAttempts; attempt++ {
		raw, err := fetchPools(chainEnum)
		if err == nil {
			if attempt > 1 {
				fmt.Printf("%s[DISCOVERY]%s %s succeeded on attempt %d/%d\n",
					config.ColorGreen, config.ColorReset, chainEnum, attempt, maxFetchAttempts)
			}
			return raw, nil
		}
		lastErr = err
		if attempt == maxFetchAttempts {
			break
		}
		backoff := time.Duration(attempt*5) * time.Second
		fmt.Printf("%s[DISCOVERY]%s %s attempt %d/%d failed: %v (retrying in %s)\n",
			config.ColorYellow, config.ColorReset, chainEnum, attempt, maxFetchAttempts, err, backoff)
		time.Sleep(backoff)
	}
	return nil, fmt.Errorf("after %d attempts: %w", maxFetchAttempts, lastErr)
}

// fetchPools queries the Balancer GraphQL API for V3 pools on the given chain
// enum (e.g. "MAINNET") and returns the raw decoded result.
func fetchPools(chainEnum string) ([]rawPool, error) {
	reqBody, err := json.Marshal(graphqlRequest{
		Query:     poolsQuery,
		Variables: map[string]interface{}{"chainIn": []string{chainEnum}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, balancerAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, truncate(string(body), 256))
	}

	var decoded graphqlResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(decoded.Errors) > 0 {
		return nil, fmt.Errorf("graphql errors: %s", decoded.Errors[0].Message)
	}
	return decoded.Data.PoolGetPools, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
