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

// ParaswapResponse represents the structure of the Paraswap API response
type ParaswapResponse struct {
	Error      string `json:"error,omitempty"`
	PriceRoute struct {
		DestAmount string `json:"destAmount,omitempty"`
		BestRoute  []struct {
			Swaps []struct {
				SwapExchanges []struct {
					Exchange      string   `json:"exchange"`
					PoolAddresses []string `json:"poolAddresses"`
				} `json:"swapExchanges"`
			} `json:"swaps"`
		} `json:"bestRoute"`
	} `json:"priceRoute"`
}

// ParaswapHandler implements the ResponseHandler interface for Paraswap API
type ParaswapHandler struct{}

// ParaswapURLBuilder implements the URLBuilder interface for Paraswap API
type ParaswapURLBuilder struct{}

// NewParaswapHandler creates a new Paraswap response handler
func NewParaswapHandler() *ParaswapHandler {
	return &ParaswapHandler{}
}

// HandleResponse processes the Paraswap API response and validates it according to business rules
func (h *ParaswapHandler) HandleResponse(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Check for no routes error message
	if string(response.Body) == `{"error":"No routes found with enough liquidity"}` {
		h.handleError(endpoint, "down", "No routes found with enough liquidity", string(response.Body))
		return fmt.Errorf("no routes found with enough liquidity")
	}

	// Parse the JSON response
	var result ParaswapResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		h.handleError(endpoint, "down", fmt.Sprintf("Error parsing JSON: %v", err), string(response.Body))
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// Check if there's an error in the response
	if result.Error != "" {
		h.handleError(endpoint, "down", fmt.Sprintf("API error: %s", result.Error), string(response.Body))
		return fmt.Errorf("API error: %s", result.Error)
	}

	// Check if priceRoute exists and has bestRoute
	if len(result.PriceRoute.BestRoute) == 0 {
		h.handleError(endpoint, "down", "No best route found", string(response.Body))
		return fmt.Errorf("no best route found")
	}

	// Check if the route uses the expected pool (Balancer V3)
	foundBalancerV3 := false
	for _, route := range result.PriceRoute.BestRoute {
		for _, swap := range route.Swaps {
			for _, exchange := range swap.SwapExchanges {
				if exchange.Exchange == "BalancerV3" {
					foundBalancerV3 = true
					break
				}
			}
		}
	}

	if !foundBalancerV3 {
		endpoint.Message = "Route does not use Balancer V3"
		prettyJSON, _ := json.MarshalIndent(result, "", "    ")
		h.handleError(endpoint, "down", "Route does not use Balancer V3", string(prettyJSON))
		return fmt.Errorf("route does not use Balancer V3")
	}

	// Store the return amount if available
	if result.PriceRoute.DestAmount != "" {
		endpoint.ReturnAmount = result.PriceRoute.DestAmount
	}

	return nil
}

// HandleResponseForMarketPrice processes the Paraswap API response for market price (all sources)
func (h *ParaswapHandler) HandleResponseForMarketPrice(response *api.APIResponse, endpoint *collector.Endpoint) error {
	// Parse the JSON response
	var result ParaswapResponse
	err := json.Unmarshal(response.Body, &result)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	// For market price, we don't validate exchanges - just extract the amount
	if result.PriceRoute.DestAmount != "" {
		endpoint.MarketPrice = result.PriceRoute.DestAmount
	}

	return nil
}

// GetIgnoreList returns the list of DEXs to ignore based on the network
func (h *ParaswapHandler) GetIgnoreList(network string) (string, error) {
	switch network {
	case "100": // Gnosis
		return "WooFiV2,AaveV3,AaveV3Stata,AaveV3StataV2,BalancerV2,CurveV1,CurveV1StableNg,CurveV2,HoneySwap,OkuTradeV3,sDAI,SushiSwap,SwaprV2,SwaprV3,Wxdai", nil
	case "42161": // Arbitrum
		return "WooFiV2,AaveV3,AaveV3Stata,AaveV3StataV2,AngleStakedStableEUR,AngleStakedStableUSD,AngleTransmuter,AugustusRFQ,BalancerV2,Bebop,Cables,Camelot,CamelotV3,Chronos,ChronosV3,CurveV1,CurveV1Factory,CurveV1StableNg,CurveV2,Dexalot,DODOV1,DODOV2,FluidDex,GMX,Hashflow,MaverickV2,PancakeSwapV2,PancakeswapV3,ParaSwapLimitOrders,Ramses,RamsesV2,SolidlyV3,SparkPsm,SushiSwap,SushiSwapV3,SwaapV2,Synapse,TraderJoeV2.1,TraderJoeV2.2,UniswapV2,UniswapV3,UniswapV4,Weth,Wombat,WooFiV2,wUSDM,Zyberswap,ZyberSwapV3", nil
	case "8453": // Base
		return "WooFiV2,AaveV3,AaveV3Stata,AaveV3StataV2,Aerodrome,AerodromeSlipstream,Alien,AlienBaseV3,AngleStakedStableUSD,AngleTransmuter,BalancerV2,BaseSwap,BaseswapV3,Bebop,CurveV1Factory,CurveV1StableNg,DackieSwap,DackieSwapV3,Dexalot,Equalizer,Hashflow,Infusion,MaverickV1,MaverickV2,PancakeswapV3,RocketSwap,SharkSwap,SolidlyV3,SoSwap,SparkPsm,SushiSwapV3,SwaapV2,SwapBased,SwapBasedV3,UniswapV2,UniswapV3,,UniswapV4,Velocimeter,Weth,Wombat,WooFiV2,wUSDM", nil
	case "1": // Ethereum Mainnet
		return "RingV2,WooFiV2,AaveGsm,AaveV2,AaveV3,AaveV3Stata,AaveV3StataV2,AngleStakedStableEUR,AngleStakedStableUSD,AngleTransmuter,AugustusRFQ,BalancerV1,BalancerV2,Bancor,Bebop,Compound,ConcentratorArusd,CurveV1,CurveV1Factory,CurveV1StableNg,CurveV2,DaiUsds,DefiSwap,DODOV1,DODOV2,Ekubo,EtherFi,FluidDex,FxProtocolRusd,Hashflow,IdleDao,KyberDmm,Lido,LinkSwap,LitePsm,MakerPsm,MaverickV1,MaverickV2,MkrSky,MWrappedM,OSwap,PancakeSwapV2,PancakeswapV3,ParaSwapLimitOrders,PolygonMigrator,ShibaSwap,Smoothy,SolidlyV2,SolidlyV3,Spark,Stader,StkGHO,sUSDS,SushiSwap,SushiSwapV3,SwaapV2,Swell,Swerve,Synapse,Synthetix,TraderJoeV2.1,UniswapV2,UniswapV3,UniswapV4,UsualBond,UsualMUsd0,UsualMWrappedM,UsualPP,Verse,Weth,Wombat,WrappedMM,wstETH,wUSDL,wUSDM", nil
	case "43114": // Avalanche
		return "Baguette,ArenaDexV2,ElkFinance,PharaohV1,LydiaFinance,CanarySwap,PangolinV3,PangolinSwap,WooFiV2,GMX,TraderJoe,TraderJoeV2.2,Dexalot,PharaohV2,AaveGsm,AaveV2,AaveV3,AaveV3Stata,AaveV3StataV2,AngleStakedStableEUR,AngleStakedStableUSD,AngleTransmuter,AugustusRFQ,BalancerV1,BalancerV2,Bancor,Bebop,Compound,ConcentratorArusd,CurveV1,CurveV1Factory,CurveV1StableNg,CurveV2,DaiUsds,DefiSwap,DODOV1,DODOV2,Ekubo,EtherFi,FluidDex,FxProtocolRusd,Hashflow,IdleDao,KyberDmm,Lido,LinkSwap,LitePsm,MakerPsm,MaverickV1,MaverickV2,MkrSky,MWrappedM,OSwap,PancakeSwapV2,PancakeswapV3,ParaSwapLimitOrders,PolygonMigrator,ShibaSwap,Smoothy,SolidlyV2,SolidlyV3,Spark,Stader,StkGHO,sUSDS,SushiSwap,SushiSwapV3,SwaapV2,Swell,Swerve,Synapse,Synthetix,TraderJoeV2.1,UniswapV2,UniswapV3,UniswapV4,UsualBond,UsualMUsd0,UsualMWrappedM,UsualPP,Verse,Weth,Wombat,WrappedMM,wstETH,wUSDL,wUSDM", nil
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

// handleError updates endpoint status and sends notifications for Paraswap-specific errors
func (h *ParaswapHandler) handleError(endpoint *collector.Endpoint, status, message, responseBody string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, message, responseBody)
	notifications.SendEmail(fmt.Sprintf("[%s] %s\nResponse body:\n%s", endpoint.Name, message, responseBody))
}

// NewParaswapURLBuilder creates a new Paraswap URL builder
func NewParaswapURLBuilder() *ParaswapURLBuilder {
	return &ParaswapURLBuilder{}
}

// BuildURL builds the complete URL for Paraswap API requests
func (b *ParaswapURLBuilder) BuildURL(endpoint *collector.Endpoint, options api.RequestOptions) (string, error) {
	baseURL := "https://api.paraswap.io/prices/"

	// Build parameters
	params := url.Values{}
	params.Add("version", "6.2")
	params.Add("srcToken", endpoint.TokenIn)
	params.Add("destToken", endpoint.TokenOut)
	params.Add("amount", endpoint.SwapAmount)
	params.Add("srcDecimals", fmt.Sprintf("%d", endpoint.TokenInDecimals))
	params.Add("destDecimals", fmt.Sprintf("%d", endpoint.TokenOutDecimals))
	params.Add("side", "SELL")
	params.Add("network", endpoint.Network)
	params.Add("otherExchangePrices", "true")
	params.Add("partner", "paraswap.io")
	params.Add("userAddress", "0x0000000000000000000000000000000000000000")
	params.Add("ignoreBadUsdPrice", "true")

	// Only add excludeDEXS if we're filtering for Balancer sources only
	if options.IsBalancerSourceOnly {
		// Create handler to get ignore list
		handler := &ParaswapHandler{}
		ignoreList, err := handler.GetIgnoreList(endpoint.Network)
		if err != nil {
			return "", fmt.Errorf("error getting ignore list: %v", err)
		}
		if ignoreList != "" {
			params.Add("excludeDEXS", ignoreList)
		}
	}

	return fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil
}
