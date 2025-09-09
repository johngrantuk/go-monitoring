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

// OneInchResponse represents the structure of the 1inch API response
type OneInchResponse struct {
	Error       string `json:"error,omitempty"`
	Description string `json:"description,omitempty"`
	StatusCode  int    `json:"statusCode,omitempty"`
	Meta        []struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"meta,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	DstAmount string `json:"dstAmount,omitempty"`
	Protocols [][][]struct {
		Name             string `json:"name"`
		Part             int    `json:"part"`
		FromTokenAddress string `json:"fromTokenAddress"`
		ToTokenAddress   string `json:"toTokenAddress"`
	} `json:"protocols,omitempty"`
}

// OneInchHandler implements the ResponseHandler interface for 1inch API
type OneInchHandler struct{}

// OneInchURLBuilder implements the URLBuilder interface for 1inch API
type OneInchURLBuilder struct{}

// NewOneInchHandler creates a new 1inch response handler
func NewOneInchHandler() *OneInchHandler {
	return &OneInchHandler{}
}

// HandleResponse processes the 1inch API response and validates it according to business rules
func (h *OneInchHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {

	// Parse the JSON response
	var result OneInchResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check if this is an error response
	if result.Description == "insufficient liquidity" {
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", "insufficient liquidity", string(prettyJSON))
		return fmt.Errorf("insufficient liquidity")
	}

	// Check if protocols is null or empty
	if result.Protocols == nil {
		h.handleError(endpoint, "down", "1inch network support WIP", string(response.Body))
		return fmt.Errorf("1inch network support WIP")
	}

	// Check if we have any protocols
	if len(result.Protocols) == 0 || len(result.Protocols[0]) == 0 || len(result.Protocols[0][0]) == 0 {
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", "no protocols found in response", string(prettyJSON))
		return fmt.Errorf("no protocols found in response")
	}

	// Check all protocols are Balancer V3
	totalPart := 0
	for _, protocol := range result.Protocols[0][0] {
		if !strings.Contains(protocol.Name, "BALANCER_V3") {
			prettyJSON, _ := json.MarshalIndent(result, "", "    ")
			h.handleError(endpoint, "down", fmt.Sprintf("found protocol %s, expected protocol containing BALANCER_V3", protocol.Name), string(prettyJSON))
			return fmt.Errorf("found protocol %s, expected protocol containing BALANCER_V3", protocol.Name)
		}
		totalPart += protocol.Part
	}

	// Verify that parts sum up to 100
	if totalPart != 100 {
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", fmt.Sprintf("protocol parts sum to %d, expected 100", totalPart), string(prettyJSON))
		return fmt.Errorf("protocol parts sum to %d, expected 100", totalPart)
	}

	// Store the return amount if available
	if result.DstAmount != "" {
		endpoint.ReturnAmount = result.DstAmount
	}

	return nil
}

// HandleResponseForMarketPrice processes the 1inch API response for market price (all sources)
func (h *OneInchHandler) HandleResponseForMarketPrice(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result OneInchResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// For market price, we don't validate protocols - just extract the amount
	if result.DstAmount != "" {
		endpoint.MarketPrice = result.DstAmount
	}

	return nil
}

// GetIgnoreList returns the list of DEXs to ignore based on the network
// For 1inch, we don't use ignore lists, we specify specific protocols instead
func (h *OneInchHandler) GetIgnoreList(network string) (string, error) {
	return "", nil
}

// GetBalancerName returns the balancer name based on the network
func (h *OneInchHandler) GetBalancerName(network string) (string, error) {
	switch network {
	case "100": // Gnosis
		return "GNOSIS_BALANCER_V3", nil
	case "42161": // Arbitrum
		return "ARBITRUM_BALANCER_V3", nil
	case "8453": // Base
		return "BASE_BALANCER_V3", nil
	case "1": // Ethereum Mainnet
		return "BALANCER_V3", nil
	case "43114": // Avalanche
		return "AVALANCHE_BALANCER_V3", nil
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

// handleError updates endpoint status and sends notifications for 1inch-specific errors
func (h *OneInchHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewOneInchURLBuilder creates a new 1inch URL builder
func NewOneInchURLBuilder() *OneInchURLBuilder {
	return &OneInchURLBuilder{}
}

// BuildURL builds the complete URL for 1inch API requests
func (b *OneInchURLBuilder) BuildURL(endpoint *collector.Endpoint, options api.RequestOptions) (string, error) {
	baseURL := "https://api.1inch.dev/swap/v6.0/" + endpoint.Network + "/quote"

	// Build parameters
	params := url.Values{}
	params.Add("src", endpoint.TokenIn)
	params.Add("dst", endpoint.TokenOut)
	params.Add("amount", endpoint.SwapAmount)

	// Only add protocol filtering if we're filtering for Balancer sources only
	if options.IsBalancerSourceOnly {
		params.Add("includeProtocols", "true")

		// Get balancer name for the network
		handler := &OneInchHandler{}
		balancerName, err := handler.GetBalancerName(endpoint.Network)
		if err != nil {
			return "", fmt.Errorf("error getting 1inch balancer name: %v", err)
		}
		params.Add("protocols", balancerName)
	}

	return fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil
}
