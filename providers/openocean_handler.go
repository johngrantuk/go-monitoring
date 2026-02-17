package providers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/api"
	"go-monitoring/internal/collector"
	"go-monitoring/notifications"
)

// OpenOceanDexInfo represents a single DEX entry from the /dexList endpoint
type OpenOceanDexInfo struct {
	Index int    `json:"index"`
	Code  string `json:"code"`
	Name  string `json:"name"`
}

// OpenOceanDexListResponse represents the response from the /dexList endpoint
type OpenOceanDexListResponse struct {
	Code int                `json:"code"`
	Data []OpenOceanDexInfo `json:"data"`
}

// OpenOceanGasPriceResponse represents the response from the /gasPrice endpoint
type OpenOceanGasPriceResponse struct {
	Code int `json:"code"`
	Data struct {
		Standard interface{} `json:"standard"`
	} `json:"data"`
}

// OpenOceanRouteDex represents a DEX in a route's subRoute
type OpenOceanRouteDex struct {
	Dex        string  `json:"dex"`
	ID         string  `json:"id"`
	Parts      int     `json:"parts"`
	Percentage float64 `json:"percentage"`
	Fee        float64 `json:"fee"`
}

// OpenOceanSubRoute represents a sub-route in the path
type OpenOceanSubRoute struct {
	From  string              `json:"from"`
	To    string              `json:"to"`
	Parts int                 `json:"parts"`
	Dexes []OpenOceanRouteDex `json:"dexes"`
}

// OpenOceanRoute represents a route in the path
type OpenOceanRoute struct {
	Parts      int                 `json:"parts"`
	Percentage float64             `json:"percentage"`
	SubRoutes  []OpenOceanSubRoute `json:"subRoutes"`
}

// OpenOceanPath represents the path structure in the response
type OpenOceanPath struct {
	From   string           `json:"from"`
	To     string           `json:"to"`
	Parts  int              `json:"parts"`
	Routes []OpenOceanRoute `json:"routes"`
}

// OpenOceanToken represents token info in the response
type OpenOceanToken struct {
	Address  string `json:"address"`
	Decimals int    `json:"decimals"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	USD      string `json:"usd"`
	Volume   float64 `json:"volume"`
}

// OpenOceanQuoteDex represents a DEX quote in the response
type OpenOceanQuoteDex struct {
	DexIndex   int    `json:"dexIndex"`
	DexCode    string `json:"dexCode"`
	SwapAmount string `json:"swapAmount"`
}

// OpenOceanResponse represents the structure of the OpenOcean API quote response
type OpenOceanResponse struct {
	Code int `json:"code"`
	Data struct {
		InToken      OpenOceanToken      `json:"inToken"`
		OutToken     OpenOceanToken      `json:"outToken"`
		InAmount     string              `json:"inAmount"`
		OutAmount    string              `json:"outAmount"`
		EstimatedGas string              `json:"estimatedGas"`
		Dexes        []OpenOceanQuoteDex `json:"dexes"`
		Path         OpenOceanPath       `json:"path"`
		PriceImpact  string              `json:"price_impact"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

// OpenOceanHandler implements the ResponseHandler interface for OpenOcean API
type OpenOceanHandler struct{}

// OpenOceanURLBuilder implements the URLBuilder interface for OpenOcean API
type OpenOceanURLBuilder struct{}

// NewOpenOceanHandler creates a new OpenOcean response handler
func NewOpenOceanHandler() *OpenOceanHandler {
	return &OpenOceanHandler{}
}

// HandleResponse processes the OpenOcean API response and validates it according to business rules
func (h *OpenOceanHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result OpenOceanResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check if the API returned an error
	if result.Code != 200 {
		h.handleError(endpoint, "down", fmt.Sprintf("OpenOcean API error (code %d): %s", result.Code, result.Error), string(response.Body))
		return fmt.Errorf("OpenOcean API error (code %d): %s", result.Code, result.Error)
	}

	// Check if we have an outAmount
	if result.Data.OutAmount == "" || result.Data.OutAmount == "0" {
		h.handleError(endpoint, "down", "No outAmount or outAmount is 0", string(response.Body))
		return fmt.Errorf("no outAmount or outAmount is 0")
	}

	// Validate that routes exist
	if len(result.Data.Path.Routes) == 0 {
		h.handleError(endpoint, "down", "No routes found in response", string(response.Body))
		return fmt.Errorf("no routes found in response")
	}

	// Validate all DEXs in route are BalancerV3
	for _, route := range result.Data.Path.Routes {
		for _, subRoute := range route.SubRoutes {
			for _, dex := range subRoute.Dexes {
				if !strings.Contains(dex.Dex, "BalancerV3") {
					prettyJSON, _ := json.MarshalIndent(result, "", "    ")
					h.handleError(endpoint, "down", fmt.Sprintf("Found DEX %s, expected BalancerV3", dex.Dex), string(prettyJSON))
					return fmt.Errorf("found DEX %s, expected BalancerV3", dex.Dex)
				}
			}
		}
	}

	// Validate that the expected pool is found in the route
	foundExpectedPool := false
	for _, route := range result.Data.Path.Routes {
		for _, subRoute := range route.SubRoutes {
			for _, dex := range subRoute.Dexes {
				if strings.EqualFold(dex.ID, endpoint.ExpectedPool) {
					foundExpectedPool = true
					break
				}
			}
			if foundExpectedPool {
				break
			}
		}
		if foundExpectedPool {
			break
		}
	}

	if !foundExpectedPool {
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", fmt.Sprintf("Expected pool %s not found in route", endpoint.ExpectedPool), string(prettyJSON))
		return fmt.Errorf("expected pool %s not found in route", endpoint.ExpectedPool)
	}

	// Store the return amount
	endpoint.ReturnAmount = result.Data.OutAmount

	return nil
}

// HandleResponseForMarketPrice processes the OpenOcean API response for market price (all sources)
func (h *OpenOceanHandler) HandleResponseForMarketPrice(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result OpenOceanResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// For market price, we don't validate sources or pools - just extract the amount
	if result.Data.OutAmount != "" {
		endpoint.MarketPrice = result.Data.OutAmount
	}

	return nil
}

// GetIgnoreList returns an empty string since OpenOcean uses enabledDexIds instead
func (h *OpenOceanHandler) GetIgnoreList(network string) (string, error) {
	return "", nil
}

// handleError updates endpoint status and sends notifications for OpenOcean-specific errors
func (h *OpenOceanHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewOpenOceanURLBuilder creates a new OpenOcean URL builder
func NewOpenOceanURLBuilder() *OpenOceanURLBuilder {
	return &OpenOceanURLBuilder{}
}

// BuildURL builds the complete URL for OpenOcean API requests
func (b *OpenOceanURLBuilder) BuildURL(endpoint *collector.Endpoint, options api.RequestOptions) (string, error) {
	// Get chain name for the API endpoint
	chainName := b.getChainName(endpoint.Network)

	// Fetch gas price from OpenOcean's gasPrice endpoint, fall back to default if it fails
	gasPrice, err := b.getGasPrice(chainName)
	if err != nil {
		gasPrice = b.getDefaultGasPrice(chainName)
		fmt.Printf("%s[WARNING]%s OpenOcean: Gas price API failed for chain %s (%v), using fallback: %s\n", config.ColorYellow, config.ColorReset, chainName, err, gasPrice)
	}

	// Build the base API URL
	baseURL := fmt.Sprintf("https://open-api.openocean.finance/v4/%s/quote", chainName)

	// Build parameters
	params := url.Values{}
	params.Add("inTokenAddress", endpoint.TokenIn)
	params.Add("outTokenAddress", endpoint.TokenOut)
	params.Add("amountDecimals", endpoint.SwapAmount)
	params.Add("gasPriceDecimals", gasPrice)
	params.Add("slippage", "1")

	// Only add DEX filtering if we're filtering for Balancer sources only
	if options.IsBalancerSourceOnly {
		enabledDexIds, err := b.getBalancerDexIndices(chainName)
		if err != nil {
			fmt.Printf("%s[WARNING]%s OpenOcean: Failed to fetch Balancer DEX indices for chain %s: %v\n", config.ColorYellow, config.ColorReset, chainName, err)
		} else if enabledDexIds != "" {
			params.Add("enabledDexIds", enabledDexIds)
		}
	}

	return fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil
}

// getChainName maps chain IDs to OpenOcean chain names
func (b *OpenOceanURLBuilder) getChainName(chainID string) string {
	switch chainID {
	case "1":
		return "eth"
	case "56":
		return "bsc"
	case "42161":
		return "arbitrum"
	case "137":
		return "polygon"
	case "10":
		return "optimism"
	case "43114":
		return "avax"
	case "8453":
		return "base"
	case "100":
		return "xdai"
	case "250":
		return "fantom"
	case "324":
		return "zksync"
	case "59144":
		return "linea"
	case "534352":
		return "scroll"
	default:
		return chainID
	}
}

// getDefaultGasPrice returns a hardcoded fallback gas price (in wei) for each chain
func (b *OpenOceanURLBuilder) getDefaultGasPrice(chainName string) string {
	switch chainName {
	case "eth":
		return "30000000000" // 30 gwei
	case "bsc":
		return "3000000000" // 3 gwei
	case "arbitrum":
		return "100000000" // 0.1 gwei
	case "polygon":
		return "30000000000" // 30 gwei
	case "optimism":
		return "1000000" // 0.001 gwei
	case "avax":
		return "25000000000" // 25 gwei
	case "base":
		return "1000000" // 0.001 gwei
	case "gnosis":
		return "2000000000" // 2 gwei
	case "fantom":
		return "50000000000" // 50 gwei
	case "zksync":
		return "250000000" // 0.25 gwei
	case "linea":
		return "50000000" // 0.05 gwei
	case "scroll":
		return "100000000" // 0.1 gwei
	default:
		return "30000000000" // 30 gwei as a safe default
	}
}

// getGasPrice fetches the current gas price from OpenOcean's gasPrice endpoint
func (b *OpenOceanURLBuilder) getGasPrice(chainName string) (string, error) {
	gasURL := fmt.Sprintf("https://open-api.openocean.finance/v4/%s/gasPrice", chainName)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(gasURL)
	if err != nil {
		return "", fmt.Errorf("error fetching gas price: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading gas price response: %v", err)
	}

	var gasResponse OpenOceanGasPriceResponse
	if err := json.Unmarshal(body, &gasResponse); err != nil {
		return "", fmt.Errorf("error parsing gas price response: %v", err)
	}

	if gasResponse.Code != 200 {
		return "", fmt.Errorf("gas price API returned code %d", gasResponse.Code)
	}

	// The standard field can be either a number (non-EVM style) or an object (EVM style with legacyGasPrice)
	switch v := gasResponse.Data.Standard.(type) {
	case float64:
		return fmt.Sprintf("%.0f", v), nil
	case map[string]interface{}:
		if legacyGasPrice, ok := v["legacyGasPrice"]; ok {
			if price, ok := legacyGasPrice.(float64); ok {
				return fmt.Sprintf("%.0f", price), nil
			}
		}
		return "", fmt.Errorf("could not extract legacyGasPrice from standard gas price object")
	default:
		return "", fmt.Errorf("unexpected gas price format: %T", v)
	}
}

// getBalancerDexIndices fetches the DEX list from OpenOcean and returns BalancerV3 DEX indices
func (b *OpenOceanURLBuilder) getBalancerDexIndices(chainName string) (string, error) {
	dexURL := fmt.Sprintf("https://open-api.openocean.finance/v4/%s/dexList", chainName)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(dexURL)
	if err != nil {
		return "", fmt.Errorf("error fetching DEX list: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading DEX list response: %v", err)
	}

	var dexListResponse OpenOceanDexListResponse
	if err := json.Unmarshal(body, &dexListResponse); err != nil {
		return "", fmt.Errorf("error parsing DEX list response: %v", err)
	}

	if dexListResponse.Code != 200 {
		return "", fmt.Errorf("DEX list API returned code %d", dexListResponse.Code)
	}

	// Find all Balancer-related DEXs and log them
	var allBalancerDexes []string
	var v3Indices []string

	for _, dex := range dexListResponse.Data {
		if strings.Contains(strings.ToLower(dex.Code), "balancer") || strings.Contains(strings.ToLower(dex.Name), "balancer") {
			allBalancerDexes = append(allBalancerDexes, fmt.Sprintf("index=%d %s", dex.Index, dex.Code))

			// Only include BalancerV3 DEXs for filtering
			if strings.Contains(dex.Code, "BalancerV3") {
				v3Indices = append(v3Indices, fmt.Sprintf("%d", dex.Index))
			}
		}
	}

	// Log all Balancer-related DEXs for visibility
	if len(allBalancerDexes) > 0 {
		fmt.Printf("%s[INFO]%s OpenOcean Balancer DEXs on chain %s: %s\n", config.ColorCyan, config.ColorReset, chainName, strings.Join(allBalancerDexes, ", "))
	} else {
		fmt.Printf("%s[WARNING]%s OpenOcean: No Balancer DEXs found on chain %s\n", config.ColorYellow, config.ColorReset, chainName)
	}

	// Log the filtered V3 indices
	if len(v3Indices) > 0 {
		fmt.Printf("%s[INFO]%s OpenOcean: Using BalancerV3 DEX indices for chain %s: %s\n", config.ColorCyan, config.ColorReset, chainName, strings.Join(v3Indices, ","))
	}

	return strings.Join(v3Indices, ","), nil
}
