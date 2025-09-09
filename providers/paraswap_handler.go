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

// ParaswapResponse represents the structure of the Paraswap API response
type ParaswapResponse struct {
	Error      string `json:"error,omitempty"`
	PriceRoute struct {
		DestAmount string `json:"destAmount,omitempty"`
		BestRoute  []struct {
			Swaps []struct {
				SwapExchanges []struct {
					Exchange      string   `json:"exchange"`
					PoolAddresses []string `json:"poolAddresses"`
				} `json:"swapExchanges"`
			} `json:"swaps"`
		} `json:"bestRoute"`
	} `json:"priceRoute"`
}

// ParaswapHandler implements the ResponseHandler interface for Paraswap API
type ParaswapHandler struct{}

// ParaswapURLBuilder implements the URLBuilder interface for Paraswap API
type ParaswapURLBuilder struct{}

// NewParaswapHandler creates a new Paraswap response handler
func NewParaswapHandler() *ParaswapHandler {
	return &ParaswapHandler{}
}

// HandleResponse processes the Paraswap API response and validates it according to business rules
func (h *ParaswapHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Check for no routes error message
	if string(response.Body) == `{"error":"No routes found with enough liquidity"}` {
		h.handleError(endpoint, "down", "No routes found with enough liquidity", string(response.Body))
		return fmt.Errorf("no routes found with enough liquidity")
	}

	// Parse the JSON response
	var result ParaswapResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check if there's an error in the response
	if result.Error != "" {
		h.handleError(endpoint, "down", fmt.Sprintf("API error: %s", result.Error), string(response.Body))
		return fmt.Errorf("API error: %s", result.Error)
	}

	// Check if priceRoute exists and has bestRoute
	if len(result.PriceRoute.BestRoute) == 0 {
		h.handleError(endpoint, "down", "No best route found", string(response.Body))
		return fmt.Errorf("no best route found")
	}

	// Check if the route uses the expected pool (Balancer V3)
	foundBalancerV3 := false
	for _, route := range result.PriceRoute.BestRoute {
		for _, swap := range route.Swaps {
			for _, exchange := range swap.SwapExchanges {
				if exchange.Exchange == "BalancerV3" {
					foundBalancerV3 = true
					break
				}
			}
		}
	}

	if !foundBalancerV3 {
		endpoint.Message = "Route does not use Balancer V3"
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", "Route does not use Balancer V3", string(prettyJSON))
		return fmt.Errorf("route does not use Balancer V3")
	}

	// Store the return amount if available
	if result.PriceRoute.DestAmount != "" {
		endpoint.ReturnAmount = result.PriceRoute.DestAmount
	}

	return nil
}

// HandleResponseForMarketPrice processes the Paraswap API response for market price (all sources)
func (h *ParaswapHandler) HandleResponseForMarketPrice(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result ParaswapResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// For market price, we don't validate exchanges - just extract the amount
	if result.PriceRoute.DestAmount != "" {
		endpoint.MarketPrice = result.PriceRoute.DestAmount
	}

	return nil
}

// GetIgnoreList returns an empty string since we now use includeDEXS instead of excludeDEXS
func (h *ParaswapHandler) GetIgnoreList(network string) (string, error) {
	// Return empty string since we use includeDEXS parameter instead
	return "", nil
}

// handleError updates endpoint status and sends notifications for Paraswap-specific errors
func (h *ParaswapHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewParaswapURLBuilder creates a new Paraswap URL builder
func NewParaswapURLBuilder() *ParaswapURLBuilder {
	return &ParaswapURLBuilder{}
}

// BuildURL builds the complete URL for Paraswap API requests
func (b *ParaswapURLBuilder) BuildURL(endpoint *collector.Endpoint, options api.RequestOptions) (string, error) {
	baseURL := "https://api.paraswap.io/prices/"

	// Build parameters
	params := url.Values{}
	params.Add("version", "6.2")
	params.Add("srcToken", endpoint.TokenIn)
	params.Add("destToken", endpoint.TokenOut)
	params.Add("amount", endpoint.SwapAmount)
	params.Add("srcDecimals", fmt.Sprintf("%d", endpoint.TokenInDecimals))
	params.Add("destDecimals", fmt.Sprintf("%d", endpoint.TokenOutDecimals))
	params.Add("side", "SELL")
	params.Add("network", endpoint.Network)
	params.Add("otherExchangePrices", "true")
	params.Add("partner", "paraswap.io")
	params.Add("userAddress", "0x0000000000000000000000000000000000000000")
	params.Add("ignoreBadUsdPrice", "true")

	// Only add includeDEXS if we're filtering for Balancer sources only
	if options.IsBalancerSourceOnly {
		params.Add("includeDEXS", "BalancerV3")
	}

	return fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil
}
