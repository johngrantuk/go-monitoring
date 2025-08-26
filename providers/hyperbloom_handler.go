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

// HyperBloomSource represents a source in the HyperBloom response
type HyperBloomSource struct {
	Name       string `json:"name"`
	Proportion string `json:"proportion"`
}

// HyperBloomOrder represents an order in the HyperBloom response
type HyperBloomOrder struct {
	Type        int    `json:"type"`
	Source      string `json:"source"`
	MakerToken  string `json:"makerToken"`
	TakerToken  string `json:"takerToken"`
	MakerAmount string `json:"makerAmount"`
	TakerAmount string `json:"takerAmount"`
}

// HyperBloomResponse represents the response structure from the HyperBloom quote endpoint
type HyperBloomResponse struct {
	ChainID               int                `json:"chainId"`
	Price                 string             `json:"price"`
	EstimatedPriceImpact  string             `json:"estimatedPriceImpact"`
	Value                 string             `json:"value"`
	GasPrice              string             `json:"gasPrice"`
	Gas                   string             `json:"gas"`
	EstimatedGas          string             `json:"estimatedGas"`
	ProtocolFee           string             `json:"protocolFee"`
	IntegratorProtocolFee string             `json:"integratorProtocolFee"`
	BuyTokenAddress       string             `json:"buyTokenAddress"`
	BuyAmount             string             `json:"buyAmount"`
	SellTokenAddress      string             `json:"sellTokenAddress"`
	SellAmount            string             `json:"sellAmount"`
	Sources               []HyperBloomSource `json:"sources"`
	Orders                []HyperBloomOrder  `json:"orders"`
	AllowanceTarget       string             `json:"allowanceTarget"`
	SellTokenToEthRate    string             `json:"sellTokenToEthRate"`
	BuyTokenToEthRate     string             `json:"buyTokenToEthRate"`
}

// HyperBloomHandler implements the ResponseHandler interface for HyperBloom API
type HyperBloomHandler struct{}

// HyperBloomURLBuilder implements the URLBuilder interface for HyperBloom API
type HyperBloomURLBuilder struct{}

// NewHyperBloomHandler creates a new HyperBloom response handler
func NewHyperBloomHandler() *HyperBloomHandler {
	return &HyperBloomHandler{}
}

// HandleResponse processes the HyperBloom API response and validates it according to business rules
func (h *HyperBloomHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result HyperBloomResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check if we have a valid buy amount
	if result.BuyAmount == "" {
		h.handleError(endpoint, "down", "no buyAmount in response", string(response.Body))
		return fmt.Errorf("no buyAmount in response")
	}

	// Check if buyAmount is greater than 0
	if result.BuyAmount == "0" {
		h.handleError(endpoint, "down", "buyAmount is 0", string(response.Body))
		return fmt.Errorf("buyAmount is 0")
	}

	// Check if we have a valid price
	if result.Price == "" {
		h.handleError(endpoint, "down", "no price in response", string(response.Body))
		return fmt.Errorf("no price in response")
	}

	// Check if price is greater than 0
	if result.Price == "0" {
		h.handleError(endpoint, "down", "price is 0", string(response.Body))
		return fmt.Errorf("price is 0")
	}

	// Validate that sources only contains BalancerV3
	if len(result.Sources) == 0 {
		h.handleError(endpoint, "down", "no sources in response", string(response.Body))
		return fmt.Errorf("no sources in response")
	}

	// Check that all sources with proportion > 0 are BalancerV3
	foundBalancerV3 := false
	for _, source := range result.Sources {
		if source.Proportion != "0" {
			if source.Name != "BalancerV3" {
				prettyJSON, _ := json.MarshalIndent(result, "", "    ")
				h.handleError(endpoint, "down", fmt.Sprintf("unexpected source found: %s with proportion %s. Expected only BalancerV3", source.Name, source.Proportion), string(prettyJSON))
				return fmt.Errorf("unexpected source found: %s with proportion %s. Expected only BalancerV3", source.Name, source.Proportion)
			}
			foundBalancerV3 = true
		}
	}

	if !foundBalancerV3 {
		h.handleError(endpoint, "down", "no BalancerV3 source found with proportion > 0", string(response.Body))
		return fmt.Errorf("no BalancerV3 source found with proportion > 0")
	}

	// Validate token addresses match
	if result.SellTokenAddress != endpoint.TokenIn {
		h.handleError(endpoint, "down", fmt.Sprintf("sellTokenAddress mismatch: expected %s, got %s", endpoint.TokenIn, result.SellTokenAddress), string(response.Body))
		return fmt.Errorf("sellTokenAddress mismatch: expected %s, got %s", endpoint.TokenIn, result.SellTokenAddress)
	}

	if result.BuyTokenAddress != endpoint.TokenOut {
		h.handleError(endpoint, "down", fmt.Sprintf("buyTokenAddress mismatch: expected %s, got %s", endpoint.TokenOut, result.BuyTokenAddress), string(response.Body))
		return fmt.Errorf("buyTokenAddress mismatch: expected %s, got %s", endpoint.TokenOut, result.BuyTokenAddress)
	}

	return nil
}

// GetIgnoreList returns the list of DEXs to ignore based on the network
// For HyperBloom, we don't use ignore lists, we specify specific sources instead
func (h *HyperBloomHandler) GetIgnoreList(network string) (string, error) {
	return "", nil
}

// handleError updates endpoint status and sends notifications for HyperBloom-specific errors
func (h *HyperBloomHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewHyperBloomURLBuilder creates a new HyperBloom URL builder
func NewHyperBloomURLBuilder() *HyperBloomURLBuilder {
	return &HyperBloomURLBuilder{}
}

// BuildURL builds the complete URL for HyperBloom API requests
func (b *HyperBloomURLBuilder) BuildURL(endpoint *collector.Endpoint, ignoreList string, options api.RequestOptions) (string, error) {
	baseURL := "https://api.hyperbloom.xyz/swap/v1/price"

	// Build parameters
	params := url.Values{}
	params.Add("sellToken", endpoint.TokenIn)
	params.Add("buyToken", endpoint.TokenOut)
	params.Add("sellAmount", endpoint.SwapAmount)
	params.Add("includedSources", "BalancerV3")

	return fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil
}
