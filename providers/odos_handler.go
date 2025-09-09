package providers

import (
	"encoding/json"
	"fmt"

	"go-monitoring/internal/api"
	"go-monitoring/internal/collector"
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

// OdosHandler implements the ResponseHandler interface for Odos
type OdosHandler struct{}

// HandleResponse processes the Odos API response
func (h *OdosHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Check status code
	if response.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	// Validate the response
	valid, err := h.validateOdosResponse(response.Body)
	if err != nil {
		return fmt.Errorf("response validation failed: %v", err)
	}

	if !valid {
		return fmt.Errorf("response validation failed")
	}

	// Extract and store the return amount
	var odosResponse OdosQuoteResponse
	if err := json.Unmarshal(response.Body, &odosResponse); err == nil && len(odosResponse.OutAmounts) > 0 {
		endpoint.ReturnAmount = odosResponse.OutAmounts[0]
	}

	return nil
}

// GetIgnoreList returns the list of DEXs to ignore for Odos
func (h *OdosHandler) GetIgnoreList(network string) (string, error) {
	return "", nil
}

// OdosURLBuilder implements the URLBuilder interface for Odos
type OdosURLBuilder struct{}

// BuildURL constructs the URL for Odos API requests
func (b *OdosURLBuilder) BuildURL(endpoint *collector.Endpoint, options api.RequestOptions) (string, error) {
	return "https://api.odos.xyz/sor/quote/v2", nil
}

// OdosRequestBodyBuilder implements the RequestBodyBuilder interface for Odos
type OdosRequestBodyBuilder struct{}

// BuildRequestBody constructs the JSON request body for Odos API requests
func (b *OdosRequestBodyBuilder) BuildRequestBody(endpoint *collector.Endpoint, options api.RequestOptions) ([]byte, error) {
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
		UserAddr: "0x47E2D28169738039755586743E2dfCF3bd643f86",
	}

	// Only add source whitelist if we're filtering for Balancer sources only
	if options.IsBalancerSourceOnly {
		requestBody.SourceWhitelist = []string{"Balancer V3 Gyro", "Balancer V3 Stable", "Balancer V3 Weighted", "Balancer V3 StableSurge", "Balancer V3 reCLAMM"}
	}

	return json.Marshal(requestBody)
}

// validateOdosResponse checks if the API response meets the monitoring requirements
func (h *OdosHandler) validateOdosResponse(body []byte) (bool, error) {
	// First try to parse as error response
	var errorResponse OdosErrorResponse
	if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.ErrorCode != 0 {
		return false, fmt.Errorf("odos API error: %s (code: %d)", h.getOdosErrorMessage(errorResponse.ErrorCode), errorResponse.ErrorCode)
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

	// Check if the first outValue is greater than 0
	if response.OutValues[0] <= 0 {
		return false, fmt.Errorf("outValues is not greater than 0: %f", response.OutValues[0])
	}

	return true, nil
}

// getOdosErrorMessage returns a human-readable error message based on the error code
func (h *OdosHandler) getOdosErrorMessage(code int) string {
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
		case 3150:
			return "Simulation internal error"
		case 3151:
			return "Simulation connection error"
		case 3152:
			return "Simulation timeout"
		case 3160:
			return "Quote internal error"
		case 3161:
			return "Quote connection error"
		case 3162:
			return "Quote timeout"
		case 3170:
			return "Swap internal error"
		case 3171:
			return "Swap connection error"
		case 3172:
			return "Swap timeout"
		case 3180:
			return "Transaction internal error"
		case 3181:
			return "Transaction connection error"
		case 3182:
			return "Transaction timeout"
		case 3190:
			return "User internal error"
		case 3191:
			return "User connection error"
		case 3192:
			return "User timeout"
		default:
			return "Unknown internal service error"
		}
	case code >= 4000 && code < 5000:
		switch code {
		case 4000:
			return "Bad request"
		case 4001:
			return "Invalid chain ID"
		case 4002:
			return "Invalid token address"
		case 4003:
			return "Invalid amount"
		case 4004:
			return "Invalid user address"
		case 4005:
			return "Invalid source whitelist"
		case 4006:
			return "Invalid destination whitelist"
		case 4007:
			return "Invalid source blacklist"
		case 4008:
			return "Invalid destination blacklist"
		case 4009:
			return "Invalid gas price"
		case 4010:
			return "Invalid gas limit"
		case 4011:
			return "Invalid slippage tolerance"
		case 4012:
			return "Invalid deadline"
		case 4013:
			return "Invalid permit"
		case 4014:
			return "Invalid signature"
		case 4015:
			return "Invalid transaction"
		case 4016:
			return "Invalid quote"
		case 4017:
			return "Invalid swap"
		case 4018:
			return "Invalid user"
		case 4019:
			return "Invalid configuration"
		case 4020:
			return "Invalid algorithm"
		case 4021:
			return "Invalid pricing"
		case 4022:
			return "Invalid gas"
		case 4023:
			return "Invalid simulation"
		case 4024:
			return "Invalid transaction assembly"
		case 4025:
			return "Invalid chain data"
		default:
			return "Unknown bad request error"
		}
	case code >= 5000 && code < 6000:
		switch code {
		case 5000:
			return "Internal server error"
		case 5001:
			return "Database error"
		case 5002:
			return "Cache error"
		case 5003:
			return "Queue error"
		case 5004:
			return "External service error"
		case 5005:
			return "Configuration error"
		case 5006:
			return "Algorithm error"
		case 5007:
			return "Pricing error"
		case 5008:
			return "Gas error"
		case 5009:
			return "Simulation error"
		case 5010:
			return "Transaction assembly error"
		case 5011:
			return "Chain data error"
		case 5012:
			return "User error"
		case 5013:
			return "Quote error"
		case 5014:
			return "Swap error"
		case 5015:
			return "Transaction error"
		default:
			return "Unknown internal server error"
		}
	default:
		return "Unknown error"
	}
}
