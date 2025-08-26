package providers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"go-monitoring/config"
	"go-monitoring/internal/api"
	"go-monitoring/internal/collector"
	"go-monitoring/notifications"
)

// KyberSwapRouteItem represents a single route item in the KyberSwap response
type KyberSwapRouteItem struct {
	Pool       string `json:"pool"`
	TokenIn    string `json:"tokenIn"`
	TokenOut   string `json:"tokenOut"`
	SwapAmount string `json:"swapAmount"`
	AmountOut  string `json:"amountOut"`
	Exchange   string `json:"exchange"`
	PoolType   string `json:"poolType"`
}

// KyberSwapResponse represents the response structure from the KyberSwap quote endpoint
type KyberSwapResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
	Data      struct {
		RouteSummary struct {
			TokenIn      string `json:"tokenIn"`
			AmountIn     string `json:"amountIn"`
			AmountInUsd  string `json:"amountInUsd"`
			TokenOut     string `json:"tokenOut"`
			AmountOut    string `json:"amountOut"`
			AmountOutUsd string `json:"amountOutUsd"`
			Gas          string `json:"gas"`
			GasPrice     string `json:"gasPrice"`
			GasUsd       string `json:"gasUsd"`
			ExtraFee     struct {
				FeeAmount   string `json:"feeAmount"`
				ChargeFeeBy string `json:"chargeFeeBy"`
				IsInBps     bool   `json:"isInBps"`
				FeeReceiver string `json:"feeReceiver"`
			} `json:"extraFee"`
			Route     [][]KyberSwapRouteItem `json:"route"`
			RouteID   string                 `json:"routeID"`
			Checksum  string                 `json:"checksum"`
			Timestamp int64                  `json:"timestamp"`
		} `json:"routeSummary"`
		RouterAddress string `json:"routerAddress"`
	} `json:"data"`
}

// KyberSwapHandler implements the ResponseHandler interface for KyberSwap API
type KyberSwapHandler struct{}

// KyberSwapURLBuilder implements the URLBuilder interface for KyberSwap API
type KyberSwapURLBuilder struct{}

// NewKyberSwapHandler creates a new KyberSwap response handler
func NewKyberSwapHandler() *KyberSwapHandler {
	return &KyberSwapHandler{}
}

// HandleResponse processes the KyberSwap API response and validates it according to business rules
func (h *KyberSwapHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {

	// Parse the JSON response
	var result KyberSwapResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check if it's an error response (code != 0)
	if result.Code != 0 {
		h.handleError(endpoint, "down", fmt.Sprintf("kyberswap API error: %s (code: %d, requestId: %s)", result.Message, result.Code, result.RequestID), string(response.Body))
		return fmt.Errorf("kyberswap API error: %s (code: %d, requestId: %s)", result.Message, result.Code, result.RequestID)
	}

	// Check if we have route summary data
	if result.Data.RouteSummary.AmountOut == "" {
		h.handleError(endpoint, "down", "no amountOut in route summary", string(response.Body))
		return fmt.Errorf("no amountOut in route summary")
	}

	// Check if amountOut is greater than 0
	if result.Data.RouteSummary.AmountOut == "0" {
		h.handleError(endpoint, "down", "amountOut is 0", string(response.Body))
		return fmt.Errorf("amountOut is 0")
	}

	// Check if we have a route ID (indicates successful route calculation)
	if result.Data.RouteSummary.RouteID == "" {
		h.handleError(endpoint, "down", "no route ID in response", string(response.Body))
		return fmt.Errorf("no route ID in response")
	}

	// Determine expected source type based on endpoint name
	var expectedSource string
	switch {
	case strings.Contains(endpoint.Name, "Quant"):
		expectedSource = "balancer-v3-quantamm"
	case strings.Contains(endpoint.Name, "Stable"):
		expectedSource = "balancer-v3-stable"
	case strings.Contains(endpoint.Name, "Gyro"):
		expectedSource = "balancer-v3-eclp"
	default:
		h.handleError(endpoint, "down", "unsupported pool type for validation", string(response.Body))
		return fmt.Errorf("unsupported pool type for validation")
	}

	// Check if route contains the expected pool and only the expected source type
	foundExpectedPool := false
	foundExpectedSource := false
	var foundExchanges []string

	for _, routeStep := range result.Data.RouteSummary.Route {
		for _, routeItem := range routeStep {
			// Track all exchanges for debugging
			foundExchanges = append(foundExchanges, routeItem.Exchange)

			// Check for expected pool
			if routeItem.Pool == endpoint.ExpectedPool {
				foundExpectedPool = true
			}

			// Check for expected source type
			if routeItem.Exchange == expectedSource {
				foundExpectedSource = true
			}
		}
	}

	// Validate that expected pool was found
	if !foundExpectedPool {
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", fmt.Sprintf("expected pool %s not found in route", endpoint.ExpectedPool), string(prettyJSON))
		return fmt.Errorf("expected pool %s not found in route", endpoint.ExpectedPool)
	}

	// Validate that expected source type was found
	if !foundExpectedSource {
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", fmt.Sprintf("expected source %s not found in route. Found exchanges: %v", expectedSource, foundExchanges), string(prettyJSON))
		return fmt.Errorf("expected source %s not found in route. Found exchanges: %v", expectedSource, foundExchanges)
	}

	// Validate that only the expected source type is found
	for _, exchange := range foundExchanges {
		if exchange != expectedSource {
			prettyJSON, _ := json.MarshalIndent(result, "", "    ")
			h.handleError(endpoint, "down", fmt.Sprintf("unexpected source found in route: %s. Expected: %s, All exchanges: %v", exchange, expectedSource, foundExchanges), string(prettyJSON))
			return fmt.Errorf("unexpected source found in route: %s. Expected: %s, All exchanges: %v", exchange, expectedSource, foundExchanges)
		}
	}

	return nil
}

// GetIgnoreList returns the list of DEXs to ignore based on the network
// For KyberSwap, we don't use ignore lists, we specify specific included sources instead
func (h *KyberSwapHandler) GetIgnoreList(network string) (string, error) {
	return "", nil
}

// GetChainName maps chain ID to KyberSwap chain name
func (h *KyberSwapHandler) GetChainName(chainID string) string {
	switch chainID {
	case "1":
		return "ethereum"
	case "56":
		return "bsc"
	case "42161":
		return "arbitrum"
	case "137":
		return "polygon"
	case "10":
		return "optimism"
	case "43114":
		return "avalanche"
	case "8453":
		return "base"
	case "324":
		return "zksync"
	case "250":
		return "fantom"
	case "59144":
		return "linea"
	case "534352":
		return "scroll"
	case "5000":
		return "mantle"
	case "81457":
		return "blast"
	case "146":
		return "sonic"
	case "80094":
		return "berachain"
	case "2020":
		return "ronin"
	case "999":
		return "hyperevm"
	default:
		return "ethereum" // default fallback
	}
}

// GetIncludedSources determines included sources based on endpoint name
func (h *KyberSwapHandler) GetIncludedSources(endpointName string) (string, error) {
	switch {
	case strings.Contains(endpointName, "Quant"):
		return "balancer-v3-quantamm", nil
	case strings.Contains(endpointName, "Stable"):
		return "balancer-v3-stable", nil
	case strings.Contains(endpointName, "Gyro"):
		return "balancer-v3-eclp", nil
	default:
		return "", fmt.Errorf("unsupported pool type")
	}
}

// handleError updates endpoint status and sends notifications for KyberSwap-specific errors
func (h *KyberSwapHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewKyberSwapURLBuilder creates a new KyberSwap URL builder
func NewKyberSwapURLBuilder() *KyberSwapURLBuilder {
	return &KyberSwapURLBuilder{}
}

// BuildURL builds the complete URL for KyberSwap API requests
func (b *KyberSwapURLBuilder) BuildURL(endpoint *collector.Endpoint, ignoreList string, options api.RequestOptions) (string, error) {
	// Get chain name for the API endpoint
	handler := &KyberSwapHandler{}
	chainName := handler.GetChainName(endpoint.Network)

	// Build the base API URL
	baseURL := fmt.Sprintf("https://aggregator-api.kyberswap.com/%s/api/v1/routes", chainName)

	// Determine included sources based on endpoint name
	includedSources, err := handler.GetIncludedSources(endpoint.Name)
	if err != nil {
		return "", fmt.Errorf("error getting included sources: %v", err)
	}

	// Build parameters
	params := url.Values{}
	params.Add("tokenIn", endpoint.TokenIn)
	params.Add("tokenOut", endpoint.TokenOut)
	params.Add("amountIn", endpoint.SwapAmount)
	params.Add("includedSources", includedSources)

	return fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil
}
