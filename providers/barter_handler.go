package providers

import (
	"encoding/json"
	"fmt"
	"net/url"

	"go-monitoring/config"
	"go-monitoring/internal/api"
	"go-monitoring/internal/collector"
	"go-monitoring/notifications"
)

// BarterResponse represents the structure of the Barter API response
type BarterResponse struct {
	Status         string `json:"status,omitempty"`
	BlockNumber    int64  `json:"blockNumber,omitempty"`
	Chain          string `json:"chain,omitempty"`
	OutputAmount   string `json:"outputAmount,omitempty"`
	GasEstimation  int64  `json:"gasEstimation,omitempty"`
	TransactionFee string `json:"transactionFee,omitempty"`
	GasPrice       string `json:"gasPrice,omitempty"`
	Route          []struct {
		SourceToken string `json:"sourceToken,omitempty"`
		Swaps       []struct {
			InputAmount          string  `json:"inputAmount,omitempty"`
			OutputAmount         string  `json:"outputAmount,omitempty"`
			OriginalOutputAmount *string `json:"originalOutputAmount,omitempty"`
			Gas                  int64   `json:"gas,omitempty"`
			SwapInfo             struct {
				TargetToken string `json:"targetToken,omitempty"`
				Metadata    struct {
					Type        string `json:"type,omitempty"`
					PoolAddress string `json:"poolAddress,omitempty"`
					Flavor      string `json:"flavor,omitempty"`
				} `json:"metadata,omitempty"`
			} `json:"swapInfo,omitempty"`
		} `json:"swaps,omitempty"`
	} `json:"route,omitempty"`
	SourceToken string `json:"sourceToken,omitempty"`
	TargetToken string `json:"targetToken,omitempty"`
	InputAmount string `json:"inputAmount,omitempty"`
}

// BarterHandler implements the ResponseHandler interface for Barter API
type BarterHandler struct{}

// BarterURLBuilder implements the URLBuilder interface for Barter API
type BarterURLBuilder struct{}

// BarterRequestBodyBuilder implements the RequestBodyBuilder interface for Barter API
type BarterRequestBodyBuilder struct{}

// NewBarterHandler creates a new Barter response handler
func NewBarterHandler() *BarterHandler {
	return &BarterHandler{}
}

// HandleResponse processes the Barter API response and validates it according to business rules
func (h *BarterHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result BarterResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check if status is not "Normal"
	if result.Status != "Normal" {
		h.handleError(endpoint, "down", fmt.Sprintf("API status is %s, expected Normal", result.Status), string(response.Body))
		return fmt.Errorf("API status is %s, expected Normal", result.Status)
	}

	// Check if route is empty
	if len(result.Route) == 0 {
		h.handleError(endpoint, "down", "No routes found in response", string(response.Body))
		return fmt.Errorf("no routes found in response")
	}

	// Check if we have swaps
	if len(result.Route[0].Swaps) == 0 {
		h.handleError(endpoint, "down", "No swaps found in route", string(response.Body))
		return fmt.Errorf("no swaps found in route")
	}

	// Check all swaps are from BalancerV3 (when filtering for Balancer sources only)
	// For Barter, we check the metadata.type field
	allBalancerV3 := true
	for _, route := range result.Route {
		for _, swap := range route.Swaps {
			if swap.SwapInfo.Metadata.Type != "BalancerV3" {
				allBalancerV3 = false
				endpoint.Message = fmt.Sprintf("Found swap type %s, expected BalancerV3", swap.SwapInfo.Metadata.Type)
				prettyJSON, _ := json.MarshalIndent(result, "", "    ")
				h.handleError(endpoint, "down", fmt.Sprintf("Found swap type %s, expected BalancerV3", swap.SwapInfo.Metadata.Type), string(prettyJSON))
				return fmt.Errorf("found swap type %s, expected BalancerV3", swap.SwapInfo.Metadata.Type)
			}
		}
	}

	if !allBalancerV3 {
		endpoint.LastStatus = "down"
		return fmt.Errorf("not all swaps are from BalancerV3")
	}

	// Check number of hops
	expectedSwaps := endpoint.ExpectedNoHops
	if len(result.Route[0].Swaps) != expectedSwaps {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Expected %d swaps (hops), got %d", expectedSwaps, len(result.Route[0].Swaps))
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", fmt.Sprintf("Expected %d swaps (hops), got %d", expectedSwaps, len(result.Route[0].Swaps)), string(prettyJSON))
		return fmt.Errorf("expected %d swaps, got %d", expectedSwaps, len(result.Route[0].Swaps))
	}

	// Store the return amount if available
	if result.OutputAmount != "" {
		endpoint.ReturnAmount = result.OutputAmount
	}

	return nil
}

// HandleResponseForMarketPrice processes the Barter API response for market price (all sources)
func (h *BarterHandler) HandleResponseForMarketPrice(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result BarterResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// For market price, we don't validate swap types or hops - just extract the amount
	if result.OutputAmount != "" {
		endpoint.MarketPrice = result.OutputAmount
	}

	return nil
}

// GetIgnoreList returns the list of DEXs to ignore based on the network
// For Barter, we don't use ignore lists, we specify typeFilters instead
func (h *BarterHandler) GetIgnoreList(network string) (string, error) {
	return "", nil
}

// handleError updates endpoint status and sends notifications for Barter-specific errors
func (h *BarterHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewBarterURLBuilder creates a new Barter URL builder
func NewBarterURLBuilder() *BarterURLBuilder {
	return &BarterURLBuilder{}
}

// BuildURL builds the complete URL for Barter API requests
func (b *BarterURLBuilder) BuildURL(endpoint *collector.Endpoint, options api.RequestOptions) (string, error) {
	// Get the base URL based on the network
	baseURL, err := b.getBaseURL(endpoint.Network)
	if err != nil {
		return "", err
	}

	// Build parameters
	params := url.Values{}
	return fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil
}

// getBaseURL returns the appropriate base URL for the given network
func (b *BarterURLBuilder) getBaseURL(network string) (string, error) {
	switch network {
	case "1": // Ethereum Mainnet
		return "https://api2.eth.barterswap.xyz/route", nil
	case "42161": // Arbitrum
		return "https://api2.arb.barterswap.xyz/route", nil
	case "8453": // Base
		return "https://api2.base.barterswap.xyz/route", nil
	case "100": // Gnosis
		return "https://api2.gno.barterswap.xyz/route", nil
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

// NewBarterRequestBodyBuilder creates a new Barter request body builder
func NewBarterRequestBodyBuilder() *BarterRequestBodyBuilder {
	return &BarterRequestBodyBuilder{}
}

// BuildRequestBody builds the JSON request body for Barter API requests
func (rb *BarterRequestBodyBuilder) BuildRequestBody(endpoint *collector.Endpoint, options api.RequestOptions) ([]byte, error) {
	// Create the base request body
	requestBody := map[string]interface{}{
		"source":     endpoint.TokenIn,
		"target":     endpoint.TokenOut,
		"sellAmount": endpoint.SwapAmount,
	}

	// Add typeFilters only if we're filtering for Balancer sources only
	if options.IsBalancerSourceOnly {
		requestBody["typeFilters"] = []string{"BalancerV3"}
	}

	// Convert to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	return jsonBody, nil
}
