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

// ZeroXResponse represents the structure of the 0x API response
type ZeroXResponse struct {
	BuyAmount string `json:"buyAmount,omitempty"`
	Route     struct {
		Fills []struct {
			Source string `json:"source"`
		} `json:"fills"`
		Tokens []struct {
			Address string `json:"address"`
			Symbol  string `json:"symbol"`
		} `json:"tokens"`
	} `json:"route"`
}

// ZeroXHandler implements the ResponseHandler interface for 0x API
type ZeroXHandler struct{}

// ZeroXURLBuilder implements the URLBuilder interface for 0x API
type ZeroXURLBuilder struct{}

// NewZeroXHandler creates a new 0x response handler
func NewZeroXHandler() *ZeroXHandler {
	return &ZeroXHandler{}
}

// HandleResponse processes the 0x API response and validates it according to business rules
func (h *ZeroXHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result ZeroXResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check if fills or tokens are null
	if result.Route.Fills == nil || result.Route.Tokens == nil {
		h.handleError(endpoint, "down", "No Routes Found", string(response.Body))
		return fmt.Errorf("response contains null fills or tokens")
	}

	// Check if all fills are from Balancer_V3
	allBalancerV3 := true
	for _, fill := range result.Route.Fills {
		if fill.Source != "Balancer_V3" {
			allBalancerV3 = false
			endpoint.Message = fmt.Sprintf("Found source %s, expected Balancer_V3", fill.Source)
			prettyJSON, _ := json.MarshalIndent(result, "", "    ")
			h.handleError(endpoint, "down", fmt.Sprintf("Found source %s, expected Balancer_V3", fill.Source), string(prettyJSON))
			return fmt.Errorf("found source %s, expected Balancer_V3", fill.Source)
		}
	}

	if !allBalancerV3 {
		endpoint.LastStatus = "down"
		return fmt.Errorf("not all fills are from Balancer_V3")
	}

	// Check number of hops
	expectedTokens := endpoint.ExpectedNoHops + 1 // Number of tokens = number of hops + 1 (start and end tokens)
	if len(result.Route.Tokens) != expectedTokens {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Expected %d tokens (hops + 2), got %d", expectedTokens, len(result.Route.Tokens))
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", fmt.Sprintf("Expected %d tokens (hops + 2), got %d", expectedTokens, len(result.Route.Tokens)), string(prettyJSON))
		return fmt.Errorf("expected %d tokens, got %d", expectedTokens, len(result.Route.Tokens))
	}

	// Store the return amount if available
	if result.BuyAmount != "" {
		endpoint.ReturnAmount = result.BuyAmount
	}

	return nil
}

// HandleResponseForMarketPrice processes the 0x API response for market price (all sources)
func (h *ZeroXHandler) HandleResponseForMarketPrice(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result ZeroXResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// For market price, we don't validate sources or hops - just extract the amount
	if result.BuyAmount != "" {
		endpoint.MarketPrice = result.BuyAmount
	}

	return nil
}

// GetIgnoreList returns the list of DEXs to ignore based on the network
func (h *ZeroXHandler) GetIgnoreList(network string) (string, error) {
	switch network {
	case "42161": // Arbitrum
		return "Bebop,Fluid,Hydrex,Blackhole,Blackhole_CL,Lithos,QuickSwap_V4,ArbSwap,DeltaSwap,Swaap_V2,SpartaDex,0x_RFQ,Angle,Balancer_V2,Camelot_V2,Camelot_V3,Curve,DODO_V2,Fluid,GMX_V1,Integral,MIMSwap,Maverick_V2,PancakeSwap_V2,PancakeSwap_V3,Ramses,Ramses_V2,Solidly_V3,SushiSwap,Swapr,Synapse,TraderJoe_V2.1,TraderJoe_V2.2,Uniswap_V2,Uniswap_V3,Uniswap_V4,WOOFi_V2,Wrapped_USDM", nil
	case "8453": // Base
		return "Bebop,Fluid,Hydrex,Blackhole,Blackhole_CL,Lithos,QuickSwap_V4,0x_RFQ,Aerodrome_V2,Aerodrome_V3,AlienBase_Stable,AlienBase_V2,AlienBase_V3,Angle,Balancer_V2,BaseSwap,BaseX,Clober_V2,Curve,DackieSwap_V2,DackieSwap_V3,DeltaSwap,Equalizer,Infusion,IziSwap,Kim_V4,Kinetix,Maverick,Maverick_V2,Morphex,Overnight,PancakeSwap_V2,PancakeSwap_V3,Pinto,RocketSwap,SharkSwap_V2,SoSwap,Solidly_V3,Spark_PSM,SushiSwap,SushiSwap_V3,Swaap_V2,SwapBased_V3,Synapse,Synthswap_V2,Synthswap_V3,Thick,Treble,Treble_V2,Uniswap_V2,Uniswap_V3,Uniswap_V4,WOOFi_V2,Wrapped_BLT,Wrapped_USDM", nil
	case "1": // Ethereum Mainnet
		return "Bebop,Fluid,Hydrex,Blackhole,Blackhole_CL,Lithos,QuickSwap_V4,0x_RFQ,Ambient,Angle,Balancer_V1,Balancer_V2,Bancor_V3,Curve,DODO_V1,DODO_V2,DeFi_Swap,Ekubo,Fluid,Fraxswap_V2,Integral,Lido,Maker_PSM,Maverick,Maverick_V2,Origin,PancakeSwap_V2,PancakeSwap_V3,Polygon_Migration,RingSwap,RocketPool,ShibaSwap,Sky_Migration,Solidly_V3,Spark,Stepn,SushiSwap,SushiSwap_V3,Swaap_V2,Synapse,Uniswap_V2,Uniswap_V3,Uniswap_V4,Wrapped_USDM,Yearn,Yearn_V3", nil
	case "43114": // Avalanche
		return "Bebop,Fluid,Hydrex,Blackhole,Blackhole_CL,Lithos,QuickSwap_V4,GMX_V1,TraderJoe_V1,Pangolin,DODO_V2,TraderJoe_V2.1,Pharaoh_CL,TraderJoe_V2.2,0x_RFQ,Aerodrome_V2,Aerodrome_V3,AlienBase_Stable,AlienBase_V2,AlienBase_V3,Angle,Balancer_V2,BaseSwap,BaseX,Clober_V2,Curve,DackieSwap_V2,DackieSwap_V3,DeltaSwap,Equalizer,Infusion,IziSwap,Kim_V4,Kinetix,Maverick,Maverick_V2,Morphex,Overnight,PancakeSwap_V2,PancakeSwap_V3,Pinto,RocketSwap,SharkSwap_V2,SoSwap,Solidly_V3,Spark_PSM,SushiSwap,SushiSwap_V3,Swaap_V2,SwapBased_V3,Synapse,Synthswap_V2,Synthswap_V3,Thick,Treble,Treble_V2,Uniswap_V2,Uniswap_V3,Uniswap_V4,WOOFi_V2,Wrapped_BLT,Wrapped_USDM", nil
	case "9745": // Plasma
		return "Bebop,Fluid,Hydrex,Blackhole,Blackhole_CL,Lithos,QuickSwap_V4,0x_RFQ,Ambient,Angle,Balancer_V1,Balancer_V2,Bancor_V3,Curve,DODO_V1,DODO_V2,DeFi_Swap,Ekubo,Fluid,Fraxswap_V2,Integral,Lido,Maker_PSM,Maverick,Maverick_V2,Origin,PancakeSwap_V2,PancakeSwap_V3,Polygon_Migration,RingSwap,RocketPool,ShibaSwap,Sky_Migration,Solidly_V3,Spark,Stepn,SushiSwap,SushiSwap_V3,Swaap_V2,Synapse,Uniswap_V2,Uniswap_V3,Uniswap_V4,Wrapped_USDM,Yearn,Yearn_V3", nil
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

// handleError updates endpoint status and sends notifications for 0x-specific errors
func (h *ZeroXHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewZeroXURLBuilder creates a new 0x URL builder
func NewZeroXURLBuilder() *ZeroXURLBuilder {
	return &ZeroXURLBuilder{}
}

// BuildURL builds the complete URL for 0x API requests
func (b *ZeroXURLBuilder) BuildURL(endpoint *collector.Endpoint, options api.RequestOptions) (string, error) {
	baseURL := "https://api.0x.org/swap/permit2/price"

	// Build parameters
	params := url.Values{}
	params.Add("chainId", endpoint.Network)
	params.Add("sellToken", endpoint.TokenIn)
	params.Add("buyToken", endpoint.TokenOut)
	params.Add("sellAmount", endpoint.SwapAmount)

	// Only add excludedSources if we're filtering for Balancer sources only
	if options.IsBalancerSourceOnly {
		// Create handler to get ignore list
		handler := &ZeroXHandler{}
		ignoreList, err := handler.GetIgnoreList(endpoint.Network)
		if err != nil {
			return "", fmt.Errorf("error getting ignore list: %v", err)
		}
		if ignoreList != "" {
			params.Add("excludedSources", ignoreList)
		}
	}

	return fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil
}
