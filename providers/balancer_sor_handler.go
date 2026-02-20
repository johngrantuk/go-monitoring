package providers

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"

	"go-monitoring/config"
	"go-monitoring/internal/api"
	"go-monitoring/internal/collector"
	"go-monitoring/notifications"
)

// BalancerSORResponse represents the structure of the Balancer SOR API response
type BalancerSORResponse struct {
	Data struct {
		SorGetSwapPaths struct {
			SwapAmount   string `json:"swapAmount"`
			ReturnAmount string `json:"returnAmount"`
			Paths        []struct {
				Pools    []string `json:"pools"`
				Tokens   []struct {
					Address string `json:"address"`
				} `json:"tokens"`
				IsBuffer []bool   `json:"isBuffer"`
			} `json:"paths"`
		} `json:"sorGetSwapPaths"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// BalancerSORHandler implements the ResponseHandler interface for Balancer SOR API
type BalancerSORHandler struct{}

// BalancerSORURLBuilder implements the URLBuilder interface for Balancer SOR API
type BalancerSORURLBuilder struct{}

// BalancerSORRequestBodyBuilder implements the RequestBodyBuilder interface for Balancer SOR API
type BalancerSORRequestBodyBuilder struct{}

// NewBalancerSORHandler creates a new Balancer SOR response handler
func NewBalancerSORHandler() *BalancerSORHandler {
	return &BalancerSORHandler{}
}

// HandleResponse processes the Balancer SOR API response and validates it according to business rules
func (h *BalancerSORHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result BalancerSORResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check for GraphQL errors
	if len(result.Errors) > 0 {
		errorMessage := result.Errors[0].Message
		h.handleError(endpoint, "down", fmt.Sprintf("GraphQL error: %s", errorMessage), string(response.Body))
		return fmt.Errorf("GraphQL error: %s", errorMessage)
	}

	// Check if sorGetSwapPaths exists and has valid data
	if result.Data.SorGetSwapPaths.SwapAmount == "" {
		h.handleError(endpoint, "down", "No swap amount found in response", string(response.Body))
		return fmt.Errorf("no swap amount found in response")
	}

	// Check if return amount is valid
	if result.Data.SorGetSwapPaths.ReturnAmount == "" {
		h.handleError(endpoint, "down", "No return amount found in response", string(response.Body))
		return fmt.Errorf("no return amount found in response")
	}

	// Store the return amount
	endpoint.ReturnAmount = result.Data.SorGetSwapPaths.ReturnAmount

	// Convert return amount from decimal format to raw format using output token decimals
	rawReturnAmount, err := h.convertFromDecimalAmount(result.Data.SorGetSwapPaths.ReturnAmount, endpoint.TokenOutDecimals)
	if err != nil {
		// Log the error but don't fail the request - just use the original decimal amount
		fmt.Printf("Warning: Could not convert return amount to raw format: %v\n", err)
	} else {
		endpoint.ReturnAmount = rawReturnAmount
	}

	// Check if paths exist and have at least 1 path
	if len(result.Data.SorGetSwapPaths.Paths) == 0 {
		h.handleError(endpoint, "down", "No paths found in response", string(response.Body))
		return fmt.Errorf("no paths found in response")
	}

	path := result.Data.SorGetSwapPaths.Paths[0]
	pools := path.Pools

	// Store path information for on-chain query
	endpoint.SwapPathPools = pools
	endpoint.SwapPathIsBuffer = path.IsBuffer

	// Extract tokenOut for each step from tokens array
	// tokens array contains: [tokenIn, intermediate1, intermediate2, ..., tokenOut]
	// For step i, tokenOut is tokens[i+1].address
	if len(path.Tokens) > 0 {
		endpoint.SwapPathTokenOut = make([]string, len(pools))
		for i := 0; i < len(pools); i++ {
			if i+1 < len(path.Tokens) {
				endpoint.SwapPathTokenOut[i] = path.Tokens[i+1].Address
			} else {
				// Fallback: if tokens array is shorter than expected, use the last token
				endpoint.SwapPathTokenOut[i] = path.Tokens[len(path.Tokens)-1].Address
			}
		}
	}

	// Check that at least one of the pools matches the expected pool
	expectedPoolFound := false
	for _, pool := range pools {
		if pool == endpoint.ExpectedPool {
			expectedPoolFound = true
			break
		}
	}

	if !expectedPoolFound {
		h.handleError(endpoint, "down", fmt.Sprintf("Expected pool %s not found in pools: %v", endpoint.ExpectedPool, pools), string(response.Body))
		return fmt.Errorf("expected pool %s not found in pools: %v", endpoint.ExpectedPool, pools)
	}

	return nil
}

// HandleResponseForMarketPrice processes the Balancer SOR API response for market price (all sources)
func (h *BalancerSORHandler) HandleResponseForMarketPrice(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result BalancerSORResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// For market price, we don't validate pools - just extract the amount
	if result.Data.SorGetSwapPaths.ReturnAmount != "" {
		// Convert return amount from decimal format to raw format using output token decimals
		rawReturnAmount, err := h.convertFromDecimalAmount(result.Data.SorGetSwapPaths.ReturnAmount, endpoint.TokenOutDecimals)
		if err != nil {
			// Log the error but don't fail the request - just use the original decimal amount
			fmt.Printf("Warning: Could not convert market price amount to raw format: %v\n", err)
			endpoint.MarketPrice = result.Data.SorGetSwapPaths.ReturnAmount
		} else {
			endpoint.MarketPrice = rawReturnAmount
		}
	}

	return nil
}

// convertFromDecimalAmount converts a decimal amount back to raw format using the token decimals
func (h *BalancerSORHandler) convertFromDecimalAmount(decimalAmount string, decimals int) (string, error) {
	// Parse the decimal amount as a float
	decimalFloat, ok := new(big.Float).SetString(decimalAmount)
	if !ok {
		return "", fmt.Errorf("invalid decimal amount: %s", decimalAmount)
	}

	// Convert to raw by multiplying by 10^decimals
	multiplier := new(big.Float).SetFloat64(math.Pow10(decimals))
	result := new(big.Float).Mul(decimalFloat, multiplier)

	// Convert to integer and then to string
	resultInt := new(big.Int)
	result.Int(resultInt)
	return resultInt.String(), nil
}

// GetIgnoreList returns the list of DEXs to ignore based on the network
// For Balancer SOR, we don't need an ignore list as it's Balancer-specific
func (h *BalancerSORHandler) GetIgnoreList(network string) (string, error) {
	// Balancer SOR doesn't use an ignore list as it's Balancer-specific
	return "", nil
}

// handleError updates endpoint status and sends notifications for Balancer SOR-specific errors
func (h *BalancerSORHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewBalancerSORURLBuilder creates a new Balancer SOR URL builder
func NewBalancerSORURLBuilder() *BalancerSORURLBuilder {
	return &BalancerSORURLBuilder{}
}

// BuildURL builds the complete URL for Balancer SOR API requests
func (b *BalancerSORURLBuilder) BuildURL(endpoint *collector.Endpoint, options api.RequestOptions) (string, error) {
	// Balancer SOR uses a fixed GraphQL endpoint
	return "https://api-v3.balancer.fi/", nil
}

// NewBalancerSORRequestBodyBuilder creates a new Balancer SOR request body builder
func NewBalancerSORRequestBodyBuilder() *BalancerSORRequestBodyBuilder {
	return &BalancerSORRequestBodyBuilder{}
}

// BuildRequestBody builds the GraphQL query for Balancer SOR API requests
func (b *BalancerSORRequestBodyBuilder) BuildRequestBody(endpoint *collector.Endpoint, options api.RequestOptions) ([]byte, error) {
	// Convert network to Balancer chain format
	chain, err := b.convertNetworkToChain(endpoint.Network)
	if err != nil {
		return nil, fmt.Errorf("error converting network to chain: %v", err)
	}

	// Convert swap amount from raw token amount to decimal format
	decimalAmount, err := b.convertToDecimalAmount(endpoint.SwapAmount, endpoint.TokenInDecimals)
	if err != nil {
		return nil, fmt.Errorf("error converting swap amount to decimal: %v", err)
	}

	// Build the GraphQL query
	var query string
	if options.IsBalancerSourceOnly {
		// When IsBalancerSourceOnly is true, add poolIds parameter
		query = fmt.Sprintf(`{
			sorGetSwapPaths(
				chain: %s
				swapAmount: "%s"
				swapType: EXACT_IN
				tokenIn: "%s"
				tokenOut: "%s"
				considerPoolsWithHooks: true
				useProtocolVersion: 3
				poolIds: ["%s"]
			) {
				swapAmount
				returnAmount
				paths {
					pools
					tokens {
						address
					}
					isBuffer
				}
			}
		}`, chain, decimalAmount, endpoint.TokenIn, endpoint.TokenOut, endpoint.ExpectedPool)
	} else {
		// Default query without poolIds
		query = fmt.Sprintf(`{
			sorGetSwapPaths(
				chain: %s
				swapAmount: "%s"
				swapType: EXACT_IN
				tokenIn: "%s"
				tokenOut: "%s"
				considerPoolsWithHooks: true
				useProtocolVersion: 3
			) {
				swapAmount
				returnAmount
				paths {
					pools
					tokens {
						address
					}
					isBuffer
				}
			}
		}`, chain, decimalAmount, endpoint.TokenIn, endpoint.TokenOut)
	}

	// Create the GraphQL request body
	requestBody := map[string]string{
		"query": query,
	}

	// Marshal to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	return jsonBody, nil
}

// convertToDecimalAmount converts a raw token amount to decimal format using the token decimals
func (b *BalancerSORRequestBodyBuilder) convertToDecimalAmount(rawAmount string, decimals int) (string, error) {
	// Parse the raw amount as a big integer
	rawInt, ok := new(big.Int).SetString(rawAmount, 10)
	if !ok {
		return "", fmt.Errorf("invalid raw amount: %s", rawAmount)
	}

	// Convert to decimal by dividing by 10^decimals
	decimalValue := new(big.Float).SetInt(rawInt)
	divisor := new(big.Float).SetFloat64(math.Pow10(decimals))
	result := new(big.Float).Quo(decimalValue, divisor)

	// Format as string with appropriate precision
	return result.Text('f', decimals), nil
}

// convertNetworkToChain converts network ID to Balancer chain format
func (b *BalancerSORRequestBodyBuilder) convertNetworkToChain(network string) (string, error) {
	switch network {
	case "1": // Ethereum Mainnet
		return "MAINNET", nil
	case "42161": // Arbitrum
		return "ARBITRUM", nil
	case "10": // Optimism
		return "OPTIMISM", nil
	case "8453": // Base
		return "BASE", nil
	case "43114": // Avalanche
		return "AVALANCHE", nil
	case "100": // Gnosis
		return "GNOSIS", nil
	case "999":
		return "HYPEREVM", nil
	case "9745": // Plasma
		return "PLASMA", nil
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}
