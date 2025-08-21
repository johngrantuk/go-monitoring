package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
	"go-monitoring/notifications"
)

// KyberSwapQuoteRequest represents the request parameters for the KyberSwap quote endpoint
type KyberSwapQuoteRequest struct {
	TokenIn  string `json:"tokenIn"`
	TokenOut string `json:"tokenOut"`
	AmountIn string `json:"amountIn"`
}

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

// KyberSwapQuoteResponse represents the response structure from the KyberSwap quote endpoint
type KyberSwapQuoteResponse struct {
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

// getChainName maps chain ID to KyberSwap chain name
func getChainName(chainID string) string {
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

// Function to check KyberSwap API status
func checkKyberSwapAPI(endpoint *collector.Endpoint) {

	// Check if this is a Quant endpoint
	if strings.Contains(endpoint.Name, "reCLAMM") {
		endpoint.LastStatus = "info"
		endpoint.Message = "KyberSwap reCLAMM integration WIP"
		fmt.Printf("%s[INFO]%s %s: API is %s%s%s\n", config.ColorYellow, config.ColorReset, endpoint.Name, config.ColorOrange, endpoint.LastStatus, config.ColorReset)
		return
	}

	endpoint.LastChecked = time.Now()

	// Get chain name for the API endpoint
	chainName := getChainName(endpoint.Network)

	// Build the API URL with query parameters
	apiURL := fmt.Sprintf("https://aggregator-api.kyberswap.com/%s/api/v1/routes", chainName)

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

	// Determine included sources based on endpoint name
	var includedSources string
	switch {
	case strings.Contains(endpoint.Name, "Quant"):
		includedSources = "balancer-v3-quantamm"
	case strings.Contains(endpoint.Name, "Stable"):
		includedSources = "balancer-v3-stable"
	case strings.Contains(endpoint.Name, "Gyro"):
		includedSources = "balancer-v3-eclp"
	default:
		endpoint.LastStatus = "down"
		endpoint.Message = "unsupported pool type"
		fmt.Printf("%s[ERROR]%s %s: %s\n", config.ColorRed, config.ColorReset, endpoint.Name, endpoint.Message)
		return
	}

	// Add query parameters using url.Values
	params := url.Values{}
	params.Add("tokenIn", endpoint.TokenIn)
	params.Add("tokenOut", endpoint.TokenOut)
	params.Add("amountIn", endpoint.SwapAmount)
	params.Add("includedSources", includedSources)
	req.URL.RawQuery = params.Encode()

	// Add client ID header for better rate limiting and service quality
	req.Header.Set("x-client-id", "BalancerTest")

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
	valid, err := validateKyberSwapResponse(body, endpoint)
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

// validateKyberSwapResponse checks if the API response meets the monitoring requirements
func validateKyberSwapResponse(body []byte, endpoint *collector.Endpoint) (bool, error) {
	// Try to parse as KyberSwap response (both success and error use same structure)
	var response KyberSwapQuoteResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return false, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if it's an error response (code != 0)
	if response.Code != 0 {
		return false, fmt.Errorf("kyberswap API error: %s (code: %d, requestId: %s)", response.Message, response.Code, response.RequestID)
	}

	// Check if we have route summary data
	if response.Data.RouteSummary.AmountOut == "" {
		return false, fmt.Errorf("no amountOut in route summary")
	}

	// Check if amountOut is greater than 0
	if response.Data.RouteSummary.AmountOut == "0" {
		return false, fmt.Errorf("amountOut is 0")
	}

	// Check if we have a route ID (indicates successful route calculation)
	if response.Data.RouteSummary.RouteID == "" {
		return false, fmt.Errorf("no route ID in response")
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
		return false, fmt.Errorf("unsupported pool type for validation")
	}

	// Check if route contains the expected pool and only the expected source type
	foundExpectedPool := false
	foundExpectedSource := false
	var foundExchanges []string

	for _, routeStep := range response.Data.RouteSummary.Route {
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
		return false, fmt.Errorf("expected pool %s not found in route", endpoint.ExpectedPool)
	}

	// Validate that expected source type was found
	if !foundExpectedSource {
		return false, fmt.Errorf("expected source %s not found in route. Found exchanges: %v", expectedSource, foundExchanges)
	}

	// Validate that only the expected source type is found
	for _, exchange := range foundExchanges {
		if exchange != expectedSource {
			return false, fmt.Errorf("unexpected source found in route: %s. Expected: %s, All exchanges: %v", exchange, expectedSource, foundExchanges)
		}
	}

	return true, nil
}
