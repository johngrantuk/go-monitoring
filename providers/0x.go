package providers

import (
	"crypto/tls"
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
)

// 0xResponse represents the structure of the 0x API response
type ZeroXResponse struct {
	Route struct {
		Fills []struct {
			Source string `json:"source"`
		} `json:"fills"`
		Tokens []struct {
			Address string `json:"address"`
			Symbol  string `json:"symbol"`
		} `json:"tokens"`
	} `json:"route"`
}

// getIgnoreList returns the list of DEXs to ignore based on the network
func get0xIgnoreList(network string) (string, error) {
	switch network {
	case "42161": // Arbitrum
		return "ArbSwap,DeltaSwap,Swaap_V2,SpartaDex,0x_RFQ,Angle,Balancer_V2,Camelot_V2,Camelot_V3,Curve,DODO_V2,Fluid,GMX_V1,Integral,MIMSwap,Maverick_V2,PancakeSwap_V2,PancakeSwap_V3,Ramses,Ramses_V2,Solidly_V3,SushiSwap,Swapr,Synapse,TraderJoe_V2.1,TraderJoe_V2.2,Uniswap_V2,Uniswap_V3,Uniswap_V4,WOOFi_V2,Wrapped_USDM", nil
	case "8453": // Base
		return "0x_RFQ,Aerodrome_V2,Aerodrome_V3,AlienBase_Stable,AlienBase_V2,AlienBase_V3,Angle,Balancer_V2,BaseSwap,BaseX,Clober_V2,Curve,DackieSwap_V2,DackieSwap_V3,DeltaSwap,Equalizer,Infusion,IziSwap,Kim_V4,Kinetix,Maverick,Maverick_V2,Morphex,Overnight,PancakeSwap_V2,PancakeSwap_V3,Pinto,RocketSwap,SharkSwap_V2,SoSwap,Solidly_V3,Spark_PSM,SushiSwap,SushiSwap_V3,Swaap_V2,SwapBased_V3,Synapse,Synthswap_V2,Synthswap_V3,Thick,Treble,Treble_V2,Uniswap_V2,Uniswap_V3,Uniswap_V4,WOOFi_V2,Wrapped_BLT,Wrapped_USDM", nil
	case "1": // Ethereum Mainnet
		return "0x_RFQ,Ambient,Angle,Balancer_V1,Balancer_V2,Bancor_V3,Curve,DODO_V1,DODO_V2,DeFi_Swap,Ekubo,Fluid,Fraxswap_V2,Integral,Lido,Maker_PSM,Maverick,Maverick_V2,Origin,PancakeSwap_V2,PancakeSwap_V3,Polygon_Migration,RingSwap,RocketPool,ShibaSwap,Sky_Migration,Solidly_V3,Spark,Stepn,SushiSwap,SushiSwap_V3,Swaap_V2,Synapse,Uniswap_V2,Uniswap_V3,Uniswap_V4,Wrapped_USDM,Yearn,Yearn_V3", nil
	case "43114": // Avalanche
		return "GMX_V1,TraderJoe_V1,Pangolin,DODO_V2,TraderJoe_V2.1,Pharaoh_CL,TraderJoe_V2.2,0x_RFQ,Aerodrome_V2,Aerodrome_V3,AlienBase_Stable,AlienBase_V2,AlienBase_V3,Angle,Balancer_V2,BaseSwap,BaseX,Clober_V2,Curve,DackieSwap_V2,DackieSwap_V3,DeltaSwap,Equalizer,Infusion,IziSwap,Kim_V4,Kinetix,Maverick,Maverick_V2,Morphex,Overnight,PancakeSwap_V2,PancakeSwap_V3,Pinto,RocketSwap,SharkSwap_V2,SoSwap,Solidly_V3,Spark_PSM,SushiSwap,SushiSwap_V3,Swaap_V2,SwapBased_V3,Synapse,Synthswap_V2,Synthswap_V3,Thick,Treble,Treble_V2,Uniswap_V2,Uniswap_V3,Uniswap_V4,WOOFi_V2,Wrapped_BLT,Wrapped_USDM", nil
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

// Check0xAPI checks the 0x API status
func Check0xAPI(endpoint *collector.Endpoint) {
	endpoint.LastChecked = time.Now()

	// Get ignore list for the network
	ignoreList, err := get0xIgnoreList(endpoint.Network)
	if err != nil {
		endpoint.LastStatus = "error"
		endpoint.Message = fmt.Sprintf("Error getting ignore list: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Error getting ignore list: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		notifications.SendEmail(fmt.Sprintf("[%s] Error getting ignore list: %v", endpoint.Name, err))
		return
	}

	// Create URL parameters
	params := url.Values{}
	params.Add("chainId", endpoint.Network)
	params.Add("sellToken", endpoint.TokenIn)
	params.Add("buyToken", endpoint.TokenOut)
	params.Add("sellAmount", endpoint.SwapAmount)
	params.Add("excludedSources", ignoreList)

	// Construct the full URL
	baseURL := "https://api.0x.org/swap/permit2/price"
	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// Create a new HTTP request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Error creating request: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Error creating request: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		notifications.SendEmail(fmt.Sprintf("[%s] Error creating request: %v", endpoint.Name, err))
		return
	}

	// Get API key from environment variable
	apiKey := os.Getenv("ZEROX_API_KEY")
	if apiKey == "" {
		endpoint.LastStatus = "error"
		endpoint.Message = "ZEROX_API_KEY environment variable not set"
		fmt.Printf("%s[ERROR]%s %s: ZEROX_API_KEY environment variable not set\n", config.ColorRed, config.ColorReset, endpoint.Name)
		notifications.SendEmail(fmt.Sprintf("[%s] ZEROX_API_KEY environment variable not set", endpoint.Name))
		return
	}

	// Add headers
	req.Header.Add("0x-api-key", apiKey)
	req.Header.Add("0x-version", "v2")

	// Create an HTTP client and send the request
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Error sending request: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Error sending request: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		notifications.SendEmail(fmt.Sprintf("[%s] Error sending request: %v", endpoint.Name, err))
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Error reading response: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Error reading response: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		notifications.SendEmail(fmt.Sprintf("[%s] Error reading response: %v", endpoint.Name, err))
		return
	}
	// fmt.Printf("%s[DEBUG]%s %s: Response body:\n%s\n", config.ColorYellow, config.ColorReset, endpoint.Name, string(body))

	// Parse the JSON response
	var result ZeroXResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Error parsing JSON: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Error parsing JSON: %v\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, err, string(body))
		notifications.SendEmail(fmt.Sprintf("[%s] Error parsing JSON: %v\nResponse body:\n%s", endpoint.Name, err, string(body)))
		return
	}

	// Check if fills or tokens are null
	if result.Route.Fills == nil || result.Route.Tokens == nil {
		endpoint.LastStatus = "down"
		endpoint.Message = "No Routes Found"
		fmt.Printf("%s[ERROR]%s %s: Response contains null fills or tokens\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, string(body))
		notifications.SendEmail(fmt.Sprintf("[%s] Response contains null fills or tokens\nResponse body:\n%s", endpoint.Name, string(body)))
		return
	}

	// Check if all fills are from Balancer_V3
	allBalancerV3 := true
	for _, fill := range result.Route.Fills {
		if fill.Source != "Balancer_V3" {
			allBalancerV3 = false
			endpoint.Message = fmt.Sprintf("Found source %s, expected Balancer_V3", fill.Source)
			prettyJSON, _ := json.MarshalIndent(result, "", "    ")
			fmt.Printf("%s[ERROR]%s %s: Found source %s, expected Balancer_V3\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, fill.Source, string(prettyJSON))
			notifications.SendEmail(fmt.Sprintf("[%s] Found source %s, expected Balancer_V3\nResponse body:\n%s", endpoint.Name, fill.Source, string(prettyJSON)))
			break
		}
	}

	if !allBalancerV3 {
		endpoint.LastStatus = "down"
		return
	}

	// Check number of hops
	expectedTokens := endpoint.ExpectedNoHops + 1 // Number of tokens = number of hops + 1 (start and end tokens)
	if len(result.Route.Tokens) != expectedTokens {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Expected %d tokens (hops + 2), got %d", expectedTokens, len(result.Route.Tokens))
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		fmt.Printf("%s[ERROR]%s %s: Expected %d tokens (hops + 2), got %d\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, expectedTokens, len(result.Route.Tokens), string(prettyJSON))
		notifications.SendEmail(fmt.Sprintf("[%s] Expected %d tokens (hops + 2), got %d\nResponse body:\n%s", endpoint.Name, expectedTokens, len(result.Route.Tokens), string(prettyJSON)))
		return
	}

	endpoint.LastStatus = "up"
	endpoint.Message = "Ok"
	fmt.Printf("%s[SUCCESS]%s %s: API is %s%s%s\n", config.ColorGreen, config.ColorReset, endpoint.Name, config.ColorGreen, endpoint.LastStatus, config.ColorReset)
}
