package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}
}

// getIgnoreList returns the list of DEXs to ignore based on the network
func get1nchIgnoreList(network string) (string, error) {
	switch network {
	case "100": // Gnosis
		return "&excludedProtocols=GNOSIS_CURVE,GNOSIS_SUSHI,GNOSIS_HONEYSWAP,GNOSIS_LEVINSWAP,GNOSIS_ONE_INCH_LIMIT_ORDER_V2,GNOSIS_ONE_INCH_LIMIT_ORDER_V3,GNOSIS_ONE_INCH_LIMIT_ORDER_V4,GNOSIS_SWAPR,GNOSIS_ELK,GNOSIS_SYMMETRIC,GNOSIS_BAOFINANCE,GNOSIS_CURVE_V2_TWOCRYPTO_META,GNOSIS_BALANCER_V2,GNOSIS_SUSHI_V3,GNOSIS_AAVE_V3,GNOSIS_BGD_AAVE_STATIC,GNOSIS_REALTRMM,GNOSIS_SDAI,GNOSIS_UNISWAP_V3", nil
	case "42161": // Arbitrum
		return "&excludedProtocols=ARBITRUM_BALANCER_V2,ARBITRUM_ONE_INCH_LIMIT_ORDER,ARBITRUM_ONE_INCH_LIMIT_ORDER_V2,ARBITRUM_ONE_INCH_LIMIT_ORDER_V3,ARBITRUM_ONE_INCH_LIMIT_ORDER_V4,ARBITRUM_DODO,ARBITRUM_DODO_V2,ARBITRUM_SUSHISWAP,ARBITRUM_DXSWAP,ARBITRUM_UNISWAP_V3,ARBITRUM_CURVE,ARBITRUM_CURVE_V2,ARBITRUM_GMX,ARBITRUM_SYNAPSE,ARBITRUM_PMM5,ARBITRUM_SADDLE,ARBITRUM_KYBERSWAP_ELASTIC,ARBITRUM_KYBER_DMM_STATIC,ARBITRUM_AAVE_V3,ARBITRUM_ELK,ARBITRUM_WOOFI_V2,ARBITRUM_CAMELOT,ARBITRUM_TRADERJOE,ARBITRUM_TRADERJOE_V2,ARBITRUM_TRADERJOE_V2_2,ARBITRUM_SWAPFISH,ARBITRUM_PMM6,ARBITRUM_ZYBER,ARBITRUM_ZYBER_STABLE,ARBITRUM_SOLIDLIZARD,ARBITRUM_ZYBER_V3,ARBITRUM_MYCELIUM,ARBITRUM_TRIDENT,ARBITRUM_SHELL_OCEAN,ARBITRUM_RAMSES,ARBITRUM_TRADERJOE_V2_1,ARBITRUM_PMM8,ARBITRUM_NOMISWAPEPCS,ARBITRUM_CAMELOT_V3,ARBITRUM_WOMBATSWAP,ARBITRUM_PMM3,ARBITRUM_CHRONOS,ARBITRUM_LIGHTER,ARBITRUM_ARBIDEX,ARBITRUM_ARBIDEX_V3,ARBSWAP,ARBSWAP_STABLE,ARBITRUM_SUSHISWAP_V3,ARBITRUM_RAMSES_V2,ARBITRUM_LEVEL_FINANCE,ARBITRUM_CHRONOS_V3,ARBITRUM_PANCAKESWAP_V3,ARBITRUM_PMM11,ARBITRUM_DODO_V3,ARBITRUM_SMARDEX,ARBITRUM_PMM14,ARBITRUM_INTEGRAL,ARBITRUM_PMM2,ARBITRUM_PMM16,ARBITRUM_DFX_FINANCE_V3,ARBITRUM_CURVE_STABLE_NG,ARBITRUM_PMM17,ARBITRUM_PMM13,ARBITRUM_VIRTUSWAP,ARBITRUM_CURVE_V2_TRICRYPTO_NG,ARBITRUM_CURVE_V2_TWOCRYPTO_NG,ARBITRUM_BGD_AAVE_STATIC,ARBITRUM_SOLIDLY_V3,ARBITRUM_ANGLE,ARBITRUM_MAVERICK_V2,ARBITRUM_PMM25,ARBITRUM_UNISWAP_V4,ARBITRUM_FLUID_DEX_T1,ARBITRUM_SPARK_PSM", nil
	case "8453": // Base
		return "&excludedProtocols=BASE_MAVERICK,BASE_ONE_INCH_LIMIT_ORDER_V3,BASE_ONE_INCH_LIMIT_ORDER_V4,BASE_UNISWAP_V3,BASE_BALANCER_V2,BASE_SUSHI_V3,BASE_SWAP,BASE_KOKONUT_SWAP,BASE_ROCKET_SWAP,BASE_SWAP_BASED,BASE_SYNTHSWAP,BASE_HORIZON_DEX,BASE_VELOCIMETER_V2,BASE_DACKIE_SWAP,BASE_ALIEN_BASE,BASE_WOOFI_V2,BASE_ZYBER_V3,BASE_PANCAKESWAP_V2,BASE_PANCAKESWAP_V3,BASE_AERODROME,BASE_BASESWAP_V3,BASE_CURVE,BASE_CURVE_V2_TRICRYPTO_NG,BASE_CURVE_V2_TWO_CRYPTO,BASE_SMARDEX,BASE_PMM11,BASE_PMM6,BASE_UNISWAP_V2,BASE_SUSHI_V2,BASE_AERODROME_V3,BASE_BGD_AAVE_STATIC,BASE_AAVE_V3,BASE_PMM14,BASE_PMM2,BASE_EQUALIZER,BASE_SOLIDLY_V3,BASE_PMM3,BASE_ANGLE,BASE_PMM19,BASE_WRAPPER_SUPER_OETH,BASE_MAVERICK_V2,BASE_SPARK_PSM,BASE_PMM25,BASE_UNISWAP_V4", nil
	case "1": // Ethereum Mainnet
		return "&excludedProtocols=UNISWAP_V1,UNISWAP_V2,SUSHI,MOONISWAP,BALANCER,COMPOUND,CURVE,CURVE_V2_SPELL_2_ASSET,CURVE_V2_SGT_2_ASSET,CURVE_V2_THRESHOLDNETWORK_2_ASSET,CHAI,OASIS,KYBER,AAVE,IEARN,BANCOR,PMM1,SWERVE,BLACKHOLESWAP,DODO,DODO_V2,VALUELIQUID,SHELL,DEFISWAP,SAKESWAP,LUASWAP,MINISWAP,MSTABLE,PMM2,AAVE_V2,ST_ETH,ONE_INCH_LP,ONE_INCH_LP_1_1,LINKSWAP,S_FINANCE,PSM,POWERINDEX,PMM3,XSIGMA,SMOOTHY_FINANCE,SADDLE,PMM4,KYBER_DMM,BALANCER_V2,UNISWAP_V3,SETH_WRAPPER,CURVE_V2,CURVE_V2_EURS_2_ASSET,CURVE_V2_ETH_CRV,CURVE_V2_ETH_CVX,CONVERGENCE_X,ONE_INCH_LIMIT_ORDER,ONE_INCH_LIMIT_ORDER_V2,ONE_INCH_LIMIT_ORDER_V3,ONE_INCH_LIMIT_ORDER_V4,DFX_FINANCE,FIXED_FEE_SWAP,DXSWAP,SHIBASWAP,UNIFI,PMMX,PMM5,PSM_PAX,PMM2MM1,WSTETH,DEFI_PLAZA,FIXED_FEE_SWAP_V3,SYNTHETIX_WRAPPER,SYNAPSE,CURVE_V2_YFI_2_ASSET,CURVE_V2_ETH_PAL,POOLTOGETHER,ETH_BANCOR_V3,PMM6,ELASTICSWAP,BALANCER_V2_WRAPPER,FRAXSWAP,RADIOSHACK,KYBERSWAP_ELASTIC,CURVE_V2_TWO_CRYPTO,PMM9,STABLE_PLAZA,PMM8,ZEROX_LIMIT_ORDER,CURVE_3CRV,KYBER_DMM_STATIC,ANGLE,ROCKET_POOL,ETHEREUM_ELK,ETHEREUM_PANCAKESWAP_V2,SYNTHETIX_ATOMIC_SIP288,PSM_GUSD,INTEGRAL,MAINNET_SOLIDLY,NOMISWAP_STABLE,CURVE_V2_TWOCRYPTO_META,MAVERICK_V1,VERSE,DFX_FINANCE_V3,ZK_BOB,PANCAKESWAP_V3,NOMISWAPEPCS,XFAI,PMM11,CURVE_V2_LLAMMA,CURVE_V2_TRICRYPTO_NG,CURVE_V2_TWOCRYPTO_NG,PMM8_2,SUSHISWAP_V3,SFRX_ETH,SDAI,ETHEREUM_WOMBATSWAP,PMM12,CARBON,COMPOUND_V3,DODO_V3,SMARDEX,TRADERJOE_V2_1,PMM14,PMM13,PMM15,SOLIDLY_V3,RAFT_PSM,PMM17,PMM18,CLAYSTACK,PMM16,CURVE_STABLE_NG,LIF3,BLUEPRINT,AAVE_V3,ORIGIN,BGD_AAVE_STATIC,SYNTHETIX_SUSD,ORIGIN_WOETH,PMM20,ETHENA,SFRAX,SDOLA,PMM22,POL_MIGRATOR,PMM23,LITEPSM_USDC,USDS_MIGRATOR,MAVERICK_V2,GHO_WRAPPER,CRVUSD_WRAPPER,USDE_WRAPPER,FLUID_DEX_T1,SCRVUSD,ORIGIN_ARMOETH,USDS_PSM_SAVINGS,PMM24,UNISWAP_V4", nil
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

// Function to check 1inch API status
func check1inchAPI(endpoint *Endpoint) {
	start := "https://proxy-app.1inch.io/v2.0/v1.5/chain/" 
	from := "/router/v6/quotes?fromTokenAddress="
	to := "&toTokenAddress="
	amount := "&amount="
    defaults := "&gasPrice=3856314&preset=maxReturnResult&isTableEnabled=true"
	ignoreList, err := get1nchIgnoreList(endpoint.Network)
	if err != nil {
		mu.Lock()
		endpoint.LastStatus = "error"
		endpoint.LastChecked = time.Now()
		endpoint.Message = fmt.Sprintf("Error getting ignore list: %v", err)
		mu.Unlock()
		fmt.Printf("%s[ERROR]%s %s: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}

	apiKey := os.Getenv("INCH_API_KEY")
	if apiKey == "" {
		mu.Lock()
		endpoint.LastStatus = "error"
		endpoint.LastChecked = time.Now()
		endpoint.Message = "INCH_API_KEY environment variable is not set"
		mu.Unlock()
		fmt.Printf("%s[ERROR]%s %s: INCH_API_KEY environment variable is not set\n", colorRed, colorReset, endpoint.Name)
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
	builder.WriteString(defaults)
	builder.WriteString(ignoreList)
	url := builder.String()
	
	client := http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		mu.Lock()
		endpoint.LastStatus = "down"
		endpoint.LastChecked = time.Now()
		endpoint.Message = fmt.Sprintf("Failed to create request: %v", err)
		mu.Unlock()
		fmt.Printf("%s[ERROR]%s %s: Failed to create request: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	resp, err := client.Do(req)

	mu.Lock()
	defer mu.Unlock()

	endpoint.LastChecked = time.Now()
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Request failed: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Request failed: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Failed to read response: %v", err)
		fmt.Printf("%s[ERROR]%s %s: Failed to read response: %v\n", colorRed, colorReset, endpoint.Name, err)
		return
	}

	// Validate the response
	valid, err := validate1inchResponse(body, endpoint.ExpectedPool)
	if err != nil {
		endpoint.LastStatus = "down"
		endpoint.Message = fmt.Sprintf("Response validation failed: %v", err)
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
		fmt.Printf("%s[FAILURE]%s %s: API is %s%s%s\n", colorRed, colorReset, endpoint.Name, colorRed, endpoint.LastStatus, colorReset)
	}
}

// validate1inchResponse checks if the API response meets the monitoring requirements
func validate1inchResponse(body []byte, expectedPool string) (bool, error) {
	var response struct {
		Results []struct {
			Protocol string `json:"protocol"`
			Routes   []struct {
				SubRoutes [][]struct {
					Market struct {
						ID string `json:"id"`
					} `json:"market"`
				} `json:"subRoutes"`
			} `json:"routes"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		prettyJSON, _ := json.MarshalIndent(response, "", "    ")
		fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", colorRed, colorReset, string(prettyJSON))
		return false, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if we have any results
	if len(response.Results) == 0 {
		prettyJSON, _ := json.MarshalIndent(response, "", "    ")
		fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", colorRed, colorReset, string(prettyJSON))
		return false, fmt.Errorf("no results found in response")
	}

	// Check all protocols are BASE_BALANCER_V3
	for _, result := range response.Results {
		if !strings.Contains(result.Protocol, "BALANCER_V3") {
			prettyJSON, _ := json.MarshalIndent(response, "", "    ")
			fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", colorRed, colorReset, string(prettyJSON))
			return false, fmt.Errorf("found protocol %s, expected protocol containing BALANCER_V3", result.Protocol)
		}

		// Check if any route contains the expected pool
		foundExpectedPool := false
		for _, route := range result.Routes {
			for _, subRoute := range route.SubRoutes {
				for _, hop := range subRoute {
					if hop.Market.ID == expectedPool {
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
			prettyJSON, _ := json.MarshalIndent(response, "", "    ")
			fmt.Printf("%s[ERROR]%s Failed response body:\n%s\n", colorRed, colorReset, string(prettyJSON))
			return false, fmt.Errorf("expected pool %s not found in any route", expectedPool)
		}
	}

	return true, nil
}