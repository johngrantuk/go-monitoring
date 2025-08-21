package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/collector"

	"github.com/joho/godotenv"
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}
}

// getIgnoreList returns the list of DEXs to ignore based on the network
func getBalancerName(network string) (string, error) {
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

// Function to check 1inch API status
func check1inchAPI(endpoint *collector.Endpoint) {

	// Check if this is a GyroE endpoint
	if strings.Contains(endpoint.Name, "GyroE") {
		endpoint.LastStatus = "info"
		endpoint.Message = "1inch GyroE integration WIP"
		fmt.Printf("%s[INFO]%s %s: API is %s%s%s\n", config.ColorYellow, config.ColorReset, endpoint.Name, config.ColorOrange, endpoint.LastStatus, config.ColorReset)
		return
	}

	// Check if this is a GyroE endpoint
	if strings.Contains(endpoint.Name, "Quant") {
		endpoint.LastStatus = "info"
		endpoint.Message = "1inch QuantAMM integration WIP"
		fmt.Printf("%s[INFO]%s %s: API is %s%s%s\n", config.ColorYellow, config.ColorReset, endpoint.Name, config.ColorOrange, endpoint.LastStatus, config.ColorReset)
		return
	}

	// Check if this is a reCLAMM endpoint
	if strings.Contains(endpoint.Name, "reCLAMM") {
		endpoint.LastStatus = "info"
		endpoint.Message = "1inch reCLAMM integration WIP"
		fmt.Printf("%s[INFO]%s %s: API is %s%s%s\n", config.ColorYellow, config.ColorReset, endpoint.Name, config.ColorOrange, endpoint.LastStatus, config.ColorReset)
		return
	}

	// Check if this is an Avalanche endpoint
	if endpoint.Network == "43114" {
		endpoint.LastStatus = "info"
		endpoint.Message = "1inch network support WIP"
		fmt.Printf("%s[INFO]%s %s: API is %s%s%s\n", config.ColorYellow, config.ColorReset, endpoint.Name, config.ColorOrange, endpoint.LastStatus, config.ColorReset)
		return
	}

	start := "https://api.1inch.dev/swap/v6.0/"
	from := "/quote?src="
	to := "&dst="
	amount := "&amount="
	balancerName, err := getBalancerName(endpoint.Network)
	if err != nil {
		endpoint.LastStatus = "error"
		endpoint.LastChecked = time.Now()
		endpoint.Message = fmt.Sprintf("Error getting 1inch balancer name: %v", err)
		fmt.Printf("%s[ERROR]%s %s: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		return
	}

	apiKey := os.Getenv("INCH_API_KEY")
	if apiKey == "" {
		endpoint.LastStatus = "error"
		endpoint.LastChecked = time.Now()
		endpoint.Message = "INCH_API_KEY environment variable is not set"
		fmt.Printf("%s[ERROR]%s %s: INCH_API_KEY environment variable is not set\n", config.ColorRed, config.ColorReset, endpoint.Name)
		return
	}

	var builder strings.Builder
	builder.WriteString(start)
	builder.WriteString(endpoint.Network)
	builder.WriteString(from)
	builder.WriteString(endpoint.TokenIn)
	builder.WriteString(to)
	builder.WriteString(endpoint.TokenOut)
	builder.WriteString(amount)
	builder.WriteString(endpoint.SwapAmount)
	builder.WriteString("&includeProtocols=true&protocols=")
	builder.WriteString(balancerName)
	url := builder.String()

	// fmt.Println(url)

	client := http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.LastChecked = time.Now()
		endpoint.Message = fmt.Sprintf("Failed to create request: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Failed to create request: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)

	endpoint.LastChecked = time.Now()
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Request failed: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Request failed: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to read response: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Failed to read response: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		return
	}

	// fmt.Println(string(body))

	// Validate the response
	valid, err := validate1inchResponse(body, endpoint.ExpectedPool)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Response validation failed: %v", err)
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
		fmt.Printf("%s[FAILURE]%s %s: API is %s%s%s\n", config.ColorRed, config.ColorReset, endpoint.Name, config.ColorRed, endpoint.LastStatus, config.ColorReset)
	}
}

// validate1inchResponse checks if the API response meets the monitoring requirements
func validate1inchResponse(body []byte, expectedPool string) (bool, error) {
	// First try to parse as error response
	var errorResponse struct {
		Error       string `json:"error"`
		Description string `json:"description"`
		StatusCode  int    `json:"statusCode"`
		Meta        []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"meta"`
		RequestID string `json:"requestId"`
	}

	if err := json.Unmarshal(body, &errorResponse); err == nil {
		// If we successfully parsed an error response
		if errorResponse.Description == "insufficient liquidity" {
			prettyJSON, _ := json.MarshalIndent(errorResponse, "", "    ")
			fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", config.ColorRed, config.ColorReset, string(prettyJSON))
			return false, fmt.Errorf("insufficient liquidity")
		}
	}

	// If not an error response, try to parse as success response
	var response struct {
		DstAmount string `json:"dstAmount"`
		Protocols [][][]struct {
			Name             string `json:"name"`
			Part             int    `json:"part"`
			FromTokenAddress string `json:"fromTokenAddress"`
			ToTokenAddress   string `json:"toTokenAddress"`
		} `json:"protocols"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		prettyJSON, _ := json.MarshalIndent(response, "", "    ")
		fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", config.ColorRed, config.ColorReset, string(prettyJSON))
		return false, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if protocols is null or empty
	if response.Protocols == nil {
		// prettyJSON, _ := json.MarshalIndent(response, "", "    ")
		// fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", config.ColorRed, config.ColorReset, string(prettyJSON))
		return false, fmt.Errorf("1inch network support WIP")
	}

	// Check if we have any protocols
	if len(response.Protocols) == 0 || len(response.Protocols[0]) == 0 || len(response.Protocols[0][0]) == 0 {
		prettyJSON, _ := json.MarshalIndent(response, "", "    ")
		fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", config.ColorRed, config.ColorReset, string(prettyJSON))
		return false, fmt.Errorf("no protocols found in response")
	}

	// Check all protocols are Balancer V3
	totalPart := 0
	for _, protocol := range response.Protocols[0][0] {
		if !strings.Contains(protocol.Name, "BALANCER_V3") {
			prettyJSON, _ := json.MarshalIndent(response, "", "    ")
			fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", config.ColorRed, config.ColorReset, string(prettyJSON))
			return false, fmt.Errorf("found protocol %s, expected protocol containing BALANCER_V3", protocol.Name)
		}
		totalPart += protocol.Part
	}

	// Verify that parts sum up to 100
	if totalPart != 100 {
		prettyJSON, _ := json.MarshalIndent(response, "", "    ")
		fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", config.ColorRed, config.ColorReset, string(prettyJSON))
		return false, fmt.Errorf("protocol parts sum to %d, expected 100", totalPart)
	}

	return true, nil
}
