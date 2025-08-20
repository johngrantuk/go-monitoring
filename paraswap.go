package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go-monitoring/config"
	"go-monitoring/notifications"
)

// getIgnoreList returns the list of DEXs to ignore based on the network
func getIgnoreList(network string) (string, error) {
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

// ParaswapResponse represents the structure of the Paraswap API response
type ParaswapResponse struct {
	Error      string `json:"error,omitempty"`
	PriceRoute struct {
		BestRoute []struct {
			Swaps []struct {
				SwapExchanges []struct {
					Exchange      string   `json:"exchange"`
					PoolAddresses []string `json:"poolAddresses"`
				} `json:"swapExchanges"`
			} `json:"swaps"`
		} `json:"bestRoute"`
	} `json:"priceRoute"`
}

// Function to check Paraswap API status
func checkParaswapAPI(endpoint *Endpoint) {
	start := "https://api.paraswap.io/prices/?version=6.2"
	end := "&otherExchangePrices=true&partner=paraswap.io&userAddress=0x0000000000000000000000000000000000000000&ignoreBadUsdPrice=true"
	ignoreList, err := getIgnoreList(endpoint.Network)
	if err != nil {
		mu.Lock()
		endpoint.LastStatus = "error"
		endpoint.LastChecked = time.Now()
		endpoint.Message = fmt.Sprintf("Error getting ignore list: %v", err)
		mu.Unlock()
		fmt.Printf("%s[ERROR]%s %s: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		notifications.SendEmail(fmt.Sprintf("[%s] Error getting ignore list: %v", endpoint.Name, err))
		return
	}
	url := fmt.Sprintf("%s&srcToken=%s&destToken=%s&amount=%s&srcDecimals=%d&destDecimals=%d&side=SELL&excludeDEXS=%s&network=%s%s", start, endpoint.TokenIn, endpoint.TokenOut, endpoint.SwapAmount, endpoint.TokenInDecimals, endpoint.TokenOutDecimals, ignoreList, endpoint.Network, end)
	// fmt.Println(url)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			// For debugging, temporarily skip verification
			InsecureSkipVerify: true,
		},
	}
	client := http.Client{Timeout: 5 * time.Second, Transport: tr}
	resp, err := client.Get(url)

	mu.Lock()
	defer mu.Unlock()

	endpoint.LastChecked = time.Now()
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Request failed: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Request failed: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		notifications.SendEmail(fmt.Sprintf("[%s] Request failed: %v", endpoint.Name, err))
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to read response: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Failed to read response: %v\n", config.ColorRed, config.ColorReset, endpoint.Name, err)
		notifications.SendEmail(fmt.Sprintf("[%s] Failed to read response: %v", endpoint.Name, err))
		return
	}

	// Check for no routes error message
	if string(body) == `{"error":"No routes found with enough liquidity"}` {
		endpoint.LastStatus = "down"
		endpoint.Message = "No routes found with enough liquidity"
		fmt.Printf("%s[ERROR]%s %s: No routes found with enough liquidity\n", config.ColorRed, config.ColorReset, endpoint.Name)
		notifications.SendEmail(fmt.Sprintf("[%s] No routes found with enough liquidity", endpoint.Name))
		return
	}

	// Parse the response
	var paraswapResp ParaswapResponse
	if err := json.Unmarshal(body, &paraswapResp); err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to parse response: %v", err)
		prettyJSON, _ := json.MarshalIndent(paraswapResp, "", "    ")
		fmt.Printf("%s[ERROR]%s %s: Failed response body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, string(prettyJSON))
		notifications.SendEmail(fmt.Sprintf("[%s] Failed to parse response: %v\nResponse body:\n%s", endpoint.Name, err, string(prettyJSON)))
		return
	}

	// Check if all swaps use BalancerV3 and have the expected pool address
	allBalancerV3 := true
	expectedPool := endpoint.ExpectedPool
	hasExpectedPool := false

	for _, route := range paraswapResp.PriceRoute.BestRoute {
		for _, swap := range route.Swaps {
			for _, exchange := range swap.SwapExchanges {
				if exchange.Exchange != "BalancerV3" {
					allBalancerV3 = false
					endpoint.Message = fmt.Sprintf("Found exchange %s, expected BalancerV3", exchange.Exchange)
					prettyJSON, _ := json.MarshalIndent(paraswapResp, "", "    ")
					fmt.Printf("%s[ERROR]%s %s: Found exchange %s, expected BalancerV3\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, exchange.Exchange, string(prettyJSON))
					notifications.SendEmail(fmt.Sprintf("[%s] Found exchange %s, expected BalancerV3\nResponse body:\n%s", endpoint.Name, exchange.Exchange, string(prettyJSON)))
					break
				}
				// hasExpectedPool := false
				for _, pool := range exchange.PoolAddresses {
					if pool == expectedPool {
						hasExpectedPool = true
						break
					}
				}
				// if !hasExpectedPool {
				// 	allBalancerV3 = false
				// 	prettyJSON, _ := json.MarshalIndent(paraswapResp, "", "    ")
				// 	fmt.Printf("%s[ERROR]%s %s: Expected pool %s not found in any route\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, expectedPool, string(prettyJSON))
				// 	break
				// }
			}
			if !allBalancerV3 {
				break
			}
		}
		if !allBalancerV3 {
			break
		}
	}

	if !hasExpectedPool {
		endpoint.Message = fmt.Sprintf("Expected pool %s not found in any route", expectedPool)
		prettyJSON, _ := json.MarshalIndent(paraswapResp, "", "    ")
		fmt.Printf("%s[ERROR]%s %s: Expected pool %s not found in any route\nResponse body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, expectedPool, string(prettyJSON))
		notifications.SendEmail(fmt.Sprintf("[%s] Expected pool %s not found in any route\nResponse body:\n%s", endpoint.Name, expectedPool, string(prettyJSON)))
	}

	if resp.StatusCode == http.StatusOK && allBalancerV3 && hasExpectedPool {
		endpoint.LastStatus = "up"
		endpoint.Message = "OK"
		fmt.Printf("%s[SUCCESS]%s %s: API is %s%s%s\n", config.ColorGreen, config.ColorReset, endpoint.Name, config.ColorGreen, endpoint.LastStatus, config.ColorReset)
	} else {
		endpoint.LastStatus = "down"
		errorMsg := ""
		if paraswapResp.Error != "" {
			errorMsg = fmt.Sprintf(", Error: %s", paraswapResp.Error)
		}
		endpoint.Message = fmt.Sprintf("Status code: %d,\nAll BalancerV3: %v,\nHas Expected Pool: %v\n%s", resp.StatusCode, allBalancerV3, hasExpectedPool, errorMsg)
		fmt.Printf("%s[FAILURE]%s %s: API is %s%s%s %d %v %v%s Response body:\n%s\n", config.ColorRed, config.ColorReset, endpoint.Name, config.ColorRed, endpoint.LastStatus, config.ColorReset, resp.StatusCode, allBalancerV3, hasExpectedPool, errorMsg, string(body))
		notifications.SendEmail(fmt.Sprintf("[%s] API check failed - Status code: %d, All BalancerV3: %v, Has Expected Pool: %v%s\nResponse body:\n%s", endpoint.Name, resp.StatusCode, allBalancerV3, hasExpectedPool, errorMsg, string(body)))
	}

	// Debug 404 status code
	if resp.StatusCode == http.StatusNotFound {
		endpoint.Message = "404 Not Found" + string(body)
		fmt.Printf("%s[DEBUG]%s %s: 404 Not Found - Response body: %s\n", config.ColorYellow, config.ColorReset, endpoint.Name, string(body))
		notifications.SendEmail(fmt.Sprintf("[%s] 404 Not Found - Response body: %s", endpoint.Name, string(body)))
	}
}
