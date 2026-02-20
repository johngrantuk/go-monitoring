package providers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
)

// routerAddresses maps chain IDs to the Balancer v3 Router contract address.
// The Router exposes querySwapSingleTokenExactIn for off-chain price simulation.
var routerAddresses = map[string]string{
	"1":     "0xAE563E3f8219521950555F5962419C8919758Ea2", // Mainnet
	"42161": "0xEAedc32a51c510d35ebC11088fD5fF2b47aACF2E", // Arbitrum
	"10":    "0xe2fa4e1d17725e72dcdAfe943Ecf45dF4B9E285b", // Optimism
	"8453":  "0x3f170631ed9821Ca51A59D996aB095162438DC10", // Base
	"43114": "0xF39CA6ede9BF7820a952b52f3c94af526bAB9015", // Avalanche
	"100":   "0x4eff2d77D9fFbAeFB4b141A3e494c085b3FF4Cb5", // Gnosis
	"999":   "0xA8920455934Da4D853faac1f94Fe7bEf72943eF1", // HyperEVM
	"9745":  "0x9dA18982a33FD0c7051B19F0d7C76F2d5E7e017c", // Plasma
}

// batchRouterAddresses maps chain IDs to the Balancer v3 BatchRouter contract address.
// The BatchRouter exposes querySwapExactIn for multi-path swap queries.
var batchRouterAddresses = map[string]string{
	"1":     "0x136f1EFcC3f8f88516B9E94110D56FDBfB1778d1", // Mainnet
	"42161": "0xaD89051bEd8d96f045E8912aE1672c6C0bF8a85E", // Arbitrum
	"10":    "0xaD89051bEd8d96f045E8912aE1672c6C0bF8a85E", // Optimism
	"8453":  "0x85a80afee867aDf27B50BdB7b76DA70f1E853062", // Base
	"43114": "0xc9b36096f5201ea332Db35d6D195774ea0D5988f", // Avalanche
	"100":   "0xe2fa4e1d17725e72dcdAfe943Ecf45dF4B9E285b", // Gnosis
	"999":   "0x9dd5Db2d38b50bEF682cE532bCca5DfD203915E1", // HyperEVM
	"9745":  "0x85a80afee867aDf27B50BdB7b76DA70f1E853062", // Plasma
}

// Router ABI JSON for querySwapSingleTokenExactIn
const routerABI = `[
	{
		"inputs": [
			{"internalType": "address", "name": "pool", "type": "address"},
			{"internalType": "address", "name": "tokenIn", "type": "address"},
			{"internalType": "address", "name": "tokenOut", "type": "address"},
			{"internalType": "uint256", "name": "exactAmountIn", "type": "uint256"},
			{"internalType": "address", "name": "sender", "type": "address"},
			{"internalType": "bytes", "name": "userData", "type": "bytes"}
		],
		"name": "querySwapSingleTokenExactIn",
		"outputs": [
			{"internalType": "uint256", "name": "amountOut", "type": "uint256"}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

// BatchRouter ABI JSON for querySwapExactIn
const batchRouterABI = `[
	{
		"inputs": [
			{
				"components": [
					{"internalType": "address", "name": "tokenIn", "type": "address"},
					{
						"components": [
							{"internalType": "address", "name": "pool", "type": "address"},
							{"internalType": "address", "name": "tokenOut", "type": "address"},
							{"internalType": "bool", "name": "isBuffer", "type": "bool"}
						],
						"internalType": "struct BatchRouter.SwapPathStep[]",
						"name": "steps",
						"type": "tuple[]"
					},
					{"internalType": "uint256", "name": "exactAmountIn", "type": "uint256"},
					{"internalType": "uint256", "name": "minAmountOut", "type": "uint256"}
				],
				"internalType": "struct BatchRouter.SwapPathExactAmountIn[]",
				"name": "paths",
				"type": "tuple[]"
			},
			{"internalType": "address", "name": "sender", "type": "address"},
			{"internalType": "bytes", "name": "userData", "type": "bytes"}
		],
		"name": "querySwapExactIn",
		"outputs": [
			{"internalType": "uint256[]", "name": "pathAmountsOut", "type": "uint256[]"},
			{"internalType": "address[]", "name": "tokensOut", "type": "address[]"},
			{"internalType": "uint256[]", "name": "amountsOut", "type": "uint256[]"}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var (
	routerABIParsed      abi.ABI
	batchRouterABIParsed abi.ABI
	clients              = make(map[string]*ethclient.Client)
	clientsMu            sync.RWMutex
	initOnce             sync.Once
)

// initABIs initializes the parsed ABI instances
func initABIs() error {
	var err error
	routerABIParsed, err = abi.JSON(strings.NewReader(routerABI))
	if err != nil {
		return fmt.Errorf("failed to parse Router ABI: %w", err)
	}

	batchRouterABIParsed, err = abi.JSON(strings.NewReader(batchRouterABI))
	if err != nil {
		return fmt.Errorf("failed to parse BatchRouter ABI: %w", err)
	}

	return nil
}

// getClient returns an ethclient for the given RPC URL, reusing existing clients
func getClient(rpcURL string) (*ethclient.Client, error) {
	clientsMu.RLock()
	client, exists := clients[rpcURL]
	clientsMu.RUnlock()

	if exists {
		return client, nil
	}

	clientsMu.Lock()
	defer clientsMu.Unlock()

	// Double-check after acquiring write lock
	if client, exists := clients[rpcURL]; exists {
		return client, nil
	}
	// Create HTTP client with proper TLS configuration for fly.io
	// Explicitly load system certificate pool to ensure CA certificates are available
	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		// If system cert pool fails, create a new empty pool
		// This can happen in some container environments
		systemCertPool = x509.NewCertPool()
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: systemCertPool,
			},
		},
		Timeout: 30 * time.Second,
	}

	// Create RPC client with custom HTTP client
	rpcClient, err := rpc.DialHTTPWithClient(rpcURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	// Wrap RPC client with ethclient
	client = ethclient.NewClient(rpcClient)

	clients[rpcURL] = client
	return client, nil
}

// SwapPathStep represents a single step in a swap path
type SwapPathStep struct {
	Pool     common.Address
	TokenOut common.Address
	IsBuffer bool
}

// SwapPathExactAmountIn represents a swap path with exact input amount
type SwapPathExactAmountIn struct {
	TokenIn       common.Address
	Steps         []SwapPathStep
	ExactAmountIn *big.Int
	MinAmountOut  *big.Int
}

// QueryOnChainPrice performs an eth_call to query the on-chain swap price.
// For single-pool swaps, it uses Router.querySwapSingleTokenExactIn.
// For multi-path swaps, it uses BatchRouter.querySwapExactIn.
// Returns the amountOut as a raw integer string.
// Returns an error if the RPC URL is not configured or the call fails.
func QueryOnChainPrice(endpoint *collector.Endpoint) (string, error) {
	initOnce.Do(func() {
		if err := initABIs(); err != nil {
			panic(fmt.Sprintf("Failed to initialize ABIs: %v", err))
		}
	})

	rpcURL := config.GetRPCURL(endpoint.Network)
	if rpcURL == "" {
		return "", fmt.Errorf("no RPC URL configured for network %s", endpoint.Network)
	}

	// Check if we have path information
	if len(endpoint.SwapPathPools) == 0 {
		return "", fmt.Errorf("no path information available for endpoint %s", endpoint.Name)
	}

	fmt.Printf("[DEBUG] On-chain query for %s:\n", endpoint.Name)
	fmt.Printf("[DEBUG]   Network: %s\n", endpoint.Network)
	fmt.Printf("[DEBUG]   RPC URL: %s\n", rpcURL)
	fmt.Printf("[DEBUG]   Path pools: %v\n", endpoint.SwapPathPools)
	fmt.Printf("[DEBUG]   Path tokenOut: %v\n", endpoint.SwapPathTokenOut)
	fmt.Printf("[DEBUG]   Path isBuffer: %v\n", endpoint.SwapPathIsBuffer)
	fmt.Printf("[DEBUG]   TokenIn: %s\n", endpoint.TokenIn)
	fmt.Printf("[DEBUG]   TokenOut: %s\n", endpoint.TokenOut)
	fmt.Printf("[DEBUG]   SwapAmount: %s\n", endpoint.SwapAmount)

	// Determine if single-pool or multi-path swap
	if len(endpoint.SwapPathPools) == 1 {
		fmt.Printf("[DEBUG]   Detected: Single-pool swap, using Router\n")
		return querySinglePoolSwap(rpcURL, endpoint)
	}

	fmt.Printf("[DEBUG]   Detected: Multi-path swap (%d pools), using BatchRouter\n", len(endpoint.SwapPathPools))
	return queryMultiPathSwap(rpcURL, endpoint)
}

// querySinglePoolSwap performs a single-pool swap query using Router.querySwapSingleTokenExactIn
func querySinglePoolSwap(rpcURL string, endpoint *collector.Endpoint) (string, error) {
	routerAddr, ok := routerAddresses[endpoint.Network]
	if !ok {
		return "", fmt.Errorf("no Router address known for network %s", endpoint.Network)
	}

	pool := endpoint.SwapPathPools[0]
	senderAddr := common.HexToAddress("0x0000000000000000000000000000000000000000")

	fmt.Printf("[DEBUG]   Router address: %s\n", routerAddr)
	fmt.Printf("[DEBUG]   Pool: %s\n", pool)
	fmt.Printf("[DEBUG]   Sender: %s\n", senderAddr.Hex())

	// Convert addresses
	poolAddr := common.HexToAddress(pool)
	tokenInAddr := common.HexToAddress(endpoint.TokenIn)
	tokenOutAddr := common.HexToAddress(endpoint.TokenOut)

	// Convert swap amount
	amountInt, ok := new(big.Int).SetString(endpoint.SwapAmount, 10)
	if !ok {
		return "", fmt.Errorf("invalid swap amount: %s", endpoint.SwapAmount)
	}

	// Pack function call
	calldata, err := routerABIParsed.Pack("querySwapSingleTokenExactIn",
		poolAddr,
		tokenInAddr,
		tokenOutAddr,
		amountInt,
		senderAddr,
		[]byte{},
	)
	if err != nil {
		return "", fmt.Errorf("ABI encoding failed: %w", err)
	}

	fmt.Printf("[DEBUG]   Calldata length: %d bytes\n", len(calldata))
	fmt.Printf("[DEBUG]   Calldata: 0x%x\n", calldata)

	// Get client and make call
	client, err := getClient(rpcURL)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contractAddr := common.HexToAddress(routerAddr)
	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: calldata,
	}

	result, err := client.CallContract(ctx, msg, nil)
	if err != nil {
		fmt.Printf("[DEBUG]   RPC call failed: %v\n", err)
		// Try to extract revert reason if available
		if rpcErr, ok := err.(interface{ ErrorCode() int }); ok {
			fmt.Printf("[DEBUG]   RPC error code: %d\n", rpcErr.ErrorCode())
		}
		return "", fmt.Errorf("eth_call failed: %w", err)
	}

	fmt.Printf("[DEBUG]   RPC result: 0x%x\n", result)

	// Unpack result - returns a single uint256
	unpacked, err := routerABIParsed.Unpack("querySwapSingleTokenExactIn", result)
	if err != nil {
		return "", fmt.Errorf("ABI decoding failed: %w", err)
	}

	if len(unpacked) == 0 {
		return "", fmt.Errorf("empty result from unpack")
	}

	amountOut, ok := unpacked[0].(*big.Int)
	if !ok {
		return "", fmt.Errorf("unexpected return type: %T", unpacked[0])
	}

	fmt.Printf("[DEBUG]   Decoded amountOut: %s\n", amountOut.String())
	return amountOut.String(), nil
}

// queryMultiPathSwap performs a multi-path swap query using BatchRouter.querySwapExactIn
func queryMultiPathSwap(rpcURL string, endpoint *collector.Endpoint) (string, error) {
	batchRouterAddr, ok := batchRouterAddresses[endpoint.Network]
	if !ok || batchRouterAddr == "" {
		return "", fmt.Errorf("no BatchRouter address known for network %s", endpoint.Network)
	}

	fmt.Printf("[DEBUG]   BatchRouter address: %s\n", batchRouterAddr)

	// Validate path information
	if len(endpoint.SwapPathPools) != len(endpoint.SwapPathTokenOut) {
		return "", fmt.Errorf("path pools length (%d) does not match tokenOut length (%d)",
			len(endpoint.SwapPathPools), len(endpoint.SwapPathTokenOut))
	}
	if len(endpoint.SwapPathPools) != len(endpoint.SwapPathIsBuffer) {
		return "", fmt.Errorf("path pools length (%d) does not match isBuffer length (%d)",
			len(endpoint.SwapPathPools), len(endpoint.SwapPathIsBuffer))
	}

	// Build SwapPathStep array
	steps := make([]SwapPathStep, len(endpoint.SwapPathPools))
	for i := 0; i < len(endpoint.SwapPathPools); i++ {
		fmt.Printf("[DEBUG]     Step %d: pool=%s, tokenOut=%s, isBuffer=%v\n",
			i, endpoint.SwapPathPools[i], endpoint.SwapPathTokenOut[i], endpoint.SwapPathIsBuffer[i])

		steps[i] = SwapPathStep{
			Pool:     common.HexToAddress(endpoint.SwapPathPools[i]),
			TokenOut: common.HexToAddress(endpoint.SwapPathTokenOut[i]),
			IsBuffer: endpoint.SwapPathIsBuffer[i],
		}
	}

	// Convert swap amount
	amountInt, ok := new(big.Int).SetString(endpoint.SwapAmount, 10)
	if !ok {
		return "", fmt.Errorf("invalid swap amount: %s", endpoint.SwapAmount)
	}

	// Build SwapPathExactAmountIn struct
	path := SwapPathExactAmountIn{
		TokenIn:       common.HexToAddress(endpoint.TokenIn),
		Steps:         steps,
		ExactAmountIn: amountInt,
		MinAmountOut:  big.NewInt(0), // 0 for queries
	}

	// Pack function call
	calldata, err := batchRouterABIParsed.Pack("querySwapExactIn",
		[]SwapPathExactAmountIn{path},
		common.HexToAddress("0x0000000000000000000000000000000000000000"),
		[]byte{},
	)
	if err != nil {
		return "", fmt.Errorf("ABI encoding failed: %w", err)
	}

	fmt.Printf("[DEBUG]   Calldata length: %d bytes\n", len(calldata))
	fmt.Printf("[DEBUG]   Calldata: 0x%x\n", calldata)

	// Get client and make call
	client, err := getClient(rpcURL)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contractAddr := common.HexToAddress(batchRouterAddr)
	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: calldata,
	}

	result, err := client.CallContract(ctx, msg, nil)
	if err != nil {
		fmt.Printf("[DEBUG]   RPC call failed: %v\n", err)
		// Try to extract revert reason if available
		if rpcErr, ok := err.(interface{ ErrorCode() int }); ok {
			fmt.Printf("[DEBUG]   RPC error code: %d\n", rpcErr.ErrorCode())
		}
		return "", fmt.Errorf("eth_call failed: %w", err)
	}

	fmt.Printf("[DEBUG]   RPC result: 0x%x\n", result)

	// Unpack result - returns (uint256[] pathAmountsOut, address[] tokensOut, uint256[] amountsOut)
	unpacked, err := batchRouterABIParsed.Unpack("querySwapExactIn", result)
	if err != nil {
		return "", fmt.Errorf("ABI decoding failed: %w", err)
	}

	if len(unpacked) < 3 {
		return "", fmt.Errorf("unexpected number of return values: %d", len(unpacked))
	}

	// unpacked[0] = pathAmountsOut []*big.Int
	// unpacked[1] = tokensOut []common.Address
	// unpacked[2] = amountsOut []*big.Int
	amountsOut, ok := unpacked[2].([]*big.Int)
	if !ok {
		return "", fmt.Errorf("unexpected return type for amountsOut: %T", unpacked[2])
	}

	if len(amountsOut) == 0 {
		return "", fmt.Errorf("empty amountsOut array")
	}

	// Return the last amountOut (final output)
	amountOut := amountsOut[len(amountsOut)-1]
	fmt.Printf("[DEBUG]   Decoded amountOut: %s\n", amountOut.String())
	return amountOut.String(), nil
}
