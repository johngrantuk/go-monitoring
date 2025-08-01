package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OdosQuoteRequest represents the request body for the Odos quote endpoint
type OdosQuoteRequest struct {
	ChainID     string `json:"chainId"`
	InputTokens []struct {
		Amount       string `json:"amount"`
		TokenAddress string `json:"tokenAddress"`
	} `json:"inputTokens"`
	OutputTokens []struct {
		Proportion   int    `json:"proportion"`
		TokenAddress string `json:"tokenAddress"`
	} `json:"outputTokens"`
	SourceWhitelist []string `json:"sourceWhitelist"`
	UserAddr        string   `json:"userAddr"`
}

// OdosQuoteResponse represents the response structure from the Odos quote endpoint
type OdosQuoteResponse struct {
	InTokens    []string  `json:"inTokens"`
	OutTokens   []string  `json:"outTokens"`
	InAmounts   []string  `json:"inAmounts"`
	OutAmounts  []string  `json:"outAmounts"`
	InValues    []float64 `json:"inValues"`
	OutValues   []float64 `json:"outValues"`
	NetOutValue float64   `json:"netOutValue"`
}

// OdosErrorResponse represents the error response structure from the Odos API
type OdosErrorResponse struct {
	Detail    string `json:"detail"`
	TraceID   string `json:"traceId"`
	ErrorCode int    `json:"errorCode"`
}

// getOdosErrorMessage returns a human-readable error message based on the error code
func getOdosErrorMessage(code int) string {
	switch {
	case code >= 1000 && code < 2000:
		return "General API Error"
	case code >= 2000 && code < 3000:
		switch code {
		case 2000:
			return "No viable path found"
		case 2400:
			return "Algorithm validation error"
		case 2997:
			return "Algorithm connection error"
		case 2998:
			return "Algorithm timeout"
		case 2999:
			return "Algorithm internal error"
		default:
			return "Unknown algorithm error"
		}
	case code >= 3000 && code < 4000:
		switch code {
		case 3000:
			return "Internal service error"
		case 3100:
			return "Configuration internal error"
		case 3101:
			return "Configuration connection error"
		case 3102:
			return "Configuration timeout"
		case 3110:
			return "Transaction assembly internal error"
		case 3111:
			return "Transaction assembly connection error"
		case 3112:
			return "Transaction assembly timeout"
		case 3120:
			return "Chain data internal error"
		case 3121:
			return "Chain data connection error"
		case 3122:
			return "Chain data timeout"
		case 3130:
			return "Pricing internal error"
		case 3131:
			return "Pricing connection error"
		case 3132:
			return "Pricing timeout"
		case 3140:
			return "Gas internal error"
		case 3141:
			return "Gas connection error"
		case 3142:
			return "Gas timeout"
		case 3143:
			return "Gas unavailable"
		default:
			return "Unknown internal service error"
		}
	case code >= 4000 && code < 5000:
		switch code {
		case 4000:
			return "Invalid request"
		case 4001:
			return "Invalid chain ID"
		case 4002:
			return "Invalid input tokens"
		case 4003:
			return "Invalid output tokens"
		case 4004:
			return "Invalid user address"
		case 4005:
			return "Blocked user address"
		case 4006:
			return "Too slippery"
		case 4007:
			return "Same input and output"
		case 4008:
			return "Multi-zap output"
		case 4009:
			return "Invalid token count"
		case 4010:
			return "Invalid token address"
		case 4011:
			return "Non-integer token amount"
		case 4012:
			return "Negative token amount"
		case 4013:
			return "Same input and output tokens"
		case 4014:
			return "Token blacklisted"
		case 4015:
			return "Invalid token proportions"
		case 4016:
			return "Token routing unavailable"
		case 4017:
			return "Invalid referral code"
		case 4018:
			return "Invalid token amount"
		case 4019:
			return "Non-string token amount"
		default:
			return "Unknown validation error"
		}
	case code >= 5000:
		switch code {
		case 5000:
			return "Internal error"
		case 5001:
			return "Swap unavailable"
		case 5002:
			return "Price check failure"
		case 5003:
			return "Default gas failure"
		default:
			return "Unknown internal error"
		}
	default:
		return "Unknown error"
	}
}

// Function to check Odos API status
func checkOdosAPI(endpoint *Endpoint) {
	mu.Lock()
	defer mu.Unlock()

	// Check if this is a Quant endpoint
	if strings.Contains(endpoint.Name, "Quant") {
		endpoint.LastStatus = "info"
		endpoint.Message = "Odos QuantAMM integration WIP"
		fmt.Printf("%s[INFO]%s %s: API is %s%s%s\n", colorYellow, colorReset, endpoint.Name, colorOrange, endpoint.LastStatus, colorReset)
		return
	}
	endpoint.LastChecked = time.Now()

	// Prepare the request body
	requestBody := OdosQuoteRequest{
		ChainID: endpoint.Network,
		InputTokens: []struct {
			Amount       string `json:"amount"`
			TokenAddress string `json:"tokenAddress"`
		}{
			{
				Amount:       endpoint.SwapAmount,
				TokenAddress: endpoint.TokenIn,
			},
		},
		OutputTokens: []struct {
			Proportion   int    `json:"proportion"`
			TokenAddress string `json:"tokenAddress"`
		}{
			{
				Proportion:   1,
				TokenAddress: endpoint.TokenOut,
			},
		},
		SourceWhitelist: []string{"Balancer V3 Gyro", "Balancer V3 Stable", "Balancer V3 Weighted", "Balancer V3 StableSurge", "Balancer V3 reCLAMM"},
		UserAddr:        "0x47E2D28169738039755586743E2dfCF3bd643f86",
	}

	// fmt.Println(requestBody)

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to marshal request body: %v", err)
		sendEmail(fmt.Sprintf("[%s] Failed to marshal request body: %v", endpoint.Name, err))
		fmt.Printf("%s[ERROR]%s %s: Failed to marshal request body: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create POST request
	req, err := http.NewRequest("POST", "https://api.odos.xyz/sor/quote/v2", bytes.NewBuffer(jsonBody))
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to create request: %v", err)
		sendEmail(fmt.Sprintf("[%s] Failed to create request: %v", endpoint.Name, err))
		fmt.Printf("%s[ERROR]%s %s: Failed to create request: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Request failed: %v", err)
		sendEmail(fmt.Sprintf("[%s] Request failed: %v", endpoint.Name, err))
		fmt.Printf("%s[ERROR]%s %s: Request failed: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to read response: %v", err)
		sendEmail(fmt.Sprintf("[%s] Failed to read response: %v", endpoint.Name, err))
		fmt.Printf("%s[ERROR]%s %s: Failed to read response: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}

	// fmt.Println(string(body))

	// Validate the response
	valid, err := validateOdosResponse(body)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Response validation failed: %v", err)
		sendEmail(fmt.Sprintf("[%s] Response validation failed: %v\nResponse body:\n%s", endpoint.Name, err, string(body)))
		fmt.Printf("%s[ERROR]%s %s: Response validation failed: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}

	if resp.StatusCode == http.StatusOK && valid {
		endpoint.LastStatus = "up"
		endpoint.Message = "OK"
		fmt.Printf("%s[SUCCESS]%s %s: API is %s%s%s\n", colorGreen, colorReset, endpoint.Name, colorGreen, endpoint.LastStatus, colorReset)
	} else {
		endpoint.LastStatus = "down"
		if endpoint.Message == "" {
			endpoint.Message = fmt.Sprintf("Status code: %d, Valid: %v", resp.StatusCode, valid)
		}
		sendEmail(fmt.Sprintf("[%s] API check failed - Status code: %d, Valid: %v\nResponse body:\n%s", endpoint.Name, resp.StatusCode, valid, string(body)))
		fmt.Printf("%s[FAILURE]%s %s: API is %s%s%s\n", colorRed, colorReset, endpoint.Name, colorRed, endpoint.LastStatus, colorReset)
	}
}

// validateOdosResponse checks if the API response meets the monitoring requirements
func validateOdosResponse(body []byte) (bool, error) {
	// First try to parse as error response
	var errorResponse OdosErrorResponse
	if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.ErrorCode != 0 {
		return false, fmt.Errorf("odos API error: %s (code: %d)", getOdosErrorMessage(errorResponse.ErrorCode), errorResponse.ErrorCode)
	}

	// If not an error response, try to parse as success response
	var response OdosQuoteResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return false, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if we have any outValues
	if len(response.OutValues) == 0 {
		return false, fmt.Errorf("no outValues in response")
	}

	// Check if outValues is greater than 0
	if response.OutValues[0] <= 0 {
		return false, fmt.Errorf("outValues is not greater than 0: %f", response.OutValues[0])
	}

	return true, nil
}
