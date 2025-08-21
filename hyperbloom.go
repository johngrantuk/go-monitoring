package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
	"go-monitoring/notifications"

	"github.com/joho/godotenv"
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}
}

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

// HyperBloomQuoteResponse represents the response structure from the HyperBloom quote endpoint
type HyperBloomQuoteResponse struct {
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

// Function to check HyperBloom API status
func checkHyperBloomAPI(endpoint *collector.Endpoint) {

	endpoint.LastChecked = time.Now()

	// Get API key from environment
	apiKey := os.Getenv("HYPERBLOOM_API_KEY")
	if apiKey == "" {
		endpoint.LastStatus = "down"
		endpoint.Message = "HYPERBLOOM_API_KEY not found in environment"
		notifications.SendEmail(fmt.Sprintf("[%s] HYPERBLOOM_API_KEY not found in environment", endpoint.Name))
		fmt.Printf("%s[ERROR]%s %s: HYPERBLOOM_API_KEY not found in environment\n", config.ColorRed, config.ColorReset, endpoint.Name)
		return
	}

	// Build the API URL
	apiURL := "https://api.hyperbloom.xyz/swap/v1/price"

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create GET request with query parameters
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to create request: %v", err)
		notifications.SendEmail(fmt.Sprintf("[%s] Failed to create request: %v", endpoint.Name, err))
		fmt.Printf("%s[ERROR]%s %s: Failed to create request: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		return
	}

	// Add query parameters using url.Values
	params := url.Values{}
	params.Add("sellToken", endpoint.TokenIn)
	params.Add("buyToken", endpoint.TokenOut)
	params.Add("sellAmount", endpoint.SwapAmount)
	params.Add("includedSources", "BalancerV3")
	req.URL.RawQuery = params.Encode()

	// Add API key header
	req.Header.Set("api-key", apiKey)

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Request failed: %v", err)
		notifications.SendEmail(fmt.Sprintf("[%s] Request failed: %v", endpoint.Name, err))
		fmt.Printf("%s[ERROR]%s %s: Request failed: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to read response: %v", err)
		notifications.SendEmail(fmt.Sprintf("[%s] Failed to read response: %v", endpoint.Name, err))
		fmt.Printf("%s[ERROR]%s %s: Failed to read response: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		return
	}

	// Validate the response
	valid, err := validateHyperBloomResponse(body, endpoint)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Response validation failed: %v", err)
		notifications.SendEmail(fmt.Sprintf("[%s] Response validation failed: %v\nResponse body:\n%s", endpoint.Name, err, string(body)))
		fmt.Printf("%s[ERROR]%s %s: Response validation failed: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		return
	}

	if resp.StatusCode == http.StatusOK && valid {
		endpoint.LastStatus = "up"
		endpoint.Message = "OK"
		fmt.Printf("%s[SUCCESS]%s %s: API is %s%s%s\n", config.ColorGreen, config.ColorReset, endpoint.Name, config.ColorGreen, endpoint.LastStatus, config.ColorReset)
	} else {
		endpoint.LastStatus = "down"
		if endpoint.Message == "" {
			endpoint.Message = fmt.Sprintf("Status code: %d, Valid: %v", resp.StatusCode, valid)
		}
		notifications.SendEmail(fmt.Sprintf("[%s] API check failed - Status code: %d, Valid: %v\nResponse body:\n%s", endpoint.Name, resp.StatusCode, valid, string(body)))
		fmt.Printf("%s[FAILURE]%s %s: API is %s%s%s\n", config.ColorRed, config.ColorReset, endpoint.Name, config.ColorRed, endpoint.LastStatus, config.ColorReset)
	}
}

// validateHyperBloomResponse checks if the API response meets the monitoring requirements
func validateHyperBloomResponse(body []byte, endpoint *collector.Endpoint) (bool, error) {
	// Try to parse as HyperBloom response
	var response HyperBloomQuoteResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return false, fmt.Errorf("failed to parse response: %v", err)
	}

	// fmt.Printf("Response: %+v\n", response)

	// Check if we have a valid buy amount
	if response.BuyAmount == "" {
		return false, fmt.Errorf("no buyAmount in response")
	}

	// Check if buyAmount is greater than 0
	if response.BuyAmount == "0" {
		return false, fmt.Errorf("buyAmount is 0")
	}

	// Check if we have a valid price
	if response.Price == "" {
		return false, fmt.Errorf("no price in response")
	}

	// Check if price is greater than 0
	if response.Price == "0" {
		return false, fmt.Errorf("price is 0")
	}

	// Validate that sources only contains BalancerV3
	if len(response.Sources) == 0 {
		return false, fmt.Errorf("no sources in response")
	}

	// Check that all sources with proportion > 0 are BalancerV3
	foundBalancerV3 := false
	for _, source := range response.Sources {
		if source.Proportion != "0" {
			if source.Name != "BalancerV3" {
				return false, fmt.Errorf("unexpected source found: %s with proportion %s. Expected only BalancerV3", source.Name, source.Proportion)
			}
			foundBalancerV3 = true
		}
	}

	if !foundBalancerV3 {
		return false, fmt.Errorf("no BalancerV3 source found with proportion > 0")
	}

	// Validate token addresses match
	if response.SellTokenAddress != endpoint.TokenIn {
		return false, fmt.Errorf("sellTokenAddress mismatch: expected %s, got %s", endpoint.TokenIn, response.SellTokenAddress)
	}

	if response.BuyTokenAddress != endpoint.TokenOut {
		return false, fmt.Errorf("buyTokenAddress mismatch: expected %s, got %s", endpoint.TokenOut, response.BuyTokenAddress)
	}

	return true, nil
}
