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

	// Check if paths exist and have at least 1 path
	if len(result.Data.SorGetSwapPaths.Paths) == 0 {
		h.handleError(endpoint, "down", "No paths found in response", string(response.Body))
		return fmt.Errorf("no paths found in response")
	}

	path := result.Data.SorGetSwapPaths.Paths[0]
	pools := path.Pools

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
func (b *BalancerSORURLBuilder) BuildURL(endpoint *collector.Endpoint, ignoreList string, options api.RequestOptions) (string, error) {
	// Balancer SOR uses a fixed GraphQL endpoint
	return "https://api-v3.balancer.fi/", nil
}

// NewBalancerSORRequestBodyBuilder creates a new Balancer SOR request body builder
func NewBalancerSORRequestBodyBuilder() *BalancerSORRequestBodyBuilder {
	return &BalancerSORRequestBodyBuilder{}
}

// BuildRequestBody builds the GraphQL query for Balancer SOR API requests
func (b *BalancerSORRequestBodyBuilder) BuildRequestBody(endpoint *collector.Endpoint, ignoreList string, options api.RequestOptions) ([]byte, error) {
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
	query := fmt.Sprintf(`{
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
				isBuffer
			}
		}
	}`, chain, decimalAmount, endpoint.TokenIn, endpoint.TokenOut)

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
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}
