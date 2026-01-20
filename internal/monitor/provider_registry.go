package monitor

import (
	"fmt"
	"strings"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/api"
	"go-monitoring/internal/collector"
	"go-monitoring/providers"
)

// ProviderConfig holds the configuration for a provider
type ProviderConfig struct {
	Handler            api.ResponseHandler
	URLBuilder         api.URLBuilder
	RequestBodyBuilder api.RequestBodyBuilder
	BaseURL            string
	APIKeyEnvVar       string
	CustomHeaders      map[string]string
	UsePOST            bool // Whether to use POST request instead of GET
}

// CheckOptions provides optional configuration for provider checks
type CheckOptions struct {
	IsBalancerSourceOnly *bool // Optional override for Balancer source only usage
}

// ProviderRegistry manages all registered providers
type ProviderRegistry struct {
	providers map[string]ProviderConfig
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]ProviderConfig),
	}
}

// RegisterProvider registers a provider with the new generic client
func (r *ProviderRegistry) RegisterProvider(name string, config ProviderConfig) {
	r.providers[name] = config
}

// CheckProvider checks a provider with custom options
func (r *ProviderRegistry) CheckProvider(endpoint *collector.Endpoint, options *CheckOptions) {
	// Check if provider uses new generic client
	if providerConfig, exists := r.providers[endpoint.RouteSolver]; exists {
		// If no specific options provided, make both calls (Balancer-only and market price)
		if options == nil {
			// First call: Balancer source only (existing behavior)
			fmt.Printf("%s[BALANCER CHECK]%s %s: Checking Balancer-only sources\n", config.ColorBlue, config.ColorReset, endpoint.Name)
			balancerOptions := &CheckOptions{IsBalancerSourceOnly: &[]bool{true}[0]}
			r.checkWithGenericClient(endpoint, providerConfig, balancerOptions)

			// Add delay between calls to avoid rate limiting
			fmt.Printf("%s[DELAY]%s %s: Waiting 2 seconds before market price check\n", config.ColorYellow, config.ColorReset, endpoint.Name)
			time.Sleep(2 * time.Second)

			// Second call: Market price (all sources)
			fmt.Printf("%s[MARKET PRICE CHECK]%s %s: Checking all sources for market price\n", config.ColorCyan, config.ColorReset, endpoint.Name)
			marketOptions := &CheckOptions{IsBalancerSourceOnly: &[]bool{false}[0]}
			r.checkWithGenericClientForMarketPrice(endpoint, providerConfig, marketOptions)
		} else {
			// Use provided options (for manual checks)
			r.checkWithGenericClient(endpoint, providerConfig, options)
		}
		return
	}

	// Provider not found
	endpoint.LastChecked = time.Now()
	endpoint.LastStatus = "unsupported"
	fmt.Printf("Unsupported route solver '%s' for endpoint %s\n", endpoint.RouteSolver, endpoint.Name)
}

// checkWithGenericClient checks a provider using the new generic client
func (r *ProviderRegistry) checkWithGenericClient(endpoint *collector.Endpoint, config ProviderConfig, checkOptions *CheckOptions) {
	// Check for WIP cases before making any requests
	if r.isWIPCase(endpoint) {
		r.handleWIPCase(endpoint)
		return
	}

	client := api.NewAPIClient()

	// Validate API key if required
	var apiKey string
	var err error
	if config.APIKeyEnvVar != "" {
		apiKey, err = client.ValidateAPIKey(config.APIKeyEnvVar, endpoint)
		if err != nil {
			return // Error already handled by ValidateAPIKey
		}
	}

	// Prepare headers
	headers := make(map[string]string)
	for key, value := range config.CustomHeaders {
		headers[key] = value
	}
	if apiKey != "" {
		// Add API key to headers (provider-specific)
		switch endpoint.RouteSolver {
		case "0x":
			headers["0x-api-key"] = apiKey
			headers["0x-version"] = "v2"
		case "1inch":
			headers["Authorization"] = fmt.Sprintf("Bearer %s", apiKey)
			headers["Content-Type"] = "application/json"
		case "hyperbloom":
			headers["api-key"] = apiKey
		case "barter":
			headers["Authorization"] = fmt.Sprintf("Bearer %s", apiKey)
		}
	}

	// Use options if provided, otherwise default to true
	isBalancerSourceOnly := true // Default behavior - most providers should use Balancer sources only
	if checkOptions != nil && checkOptions.IsBalancerSourceOnly != nil {
		isBalancerSourceOnly = *checkOptions.IsBalancerSourceOnly
	}
	// Configure request options
	requestOptions := api.RequestOptions{
		IsBalancerSourceOnly: isBalancerSourceOnly,
		CustomHeaders:        headers,
	}

	client.CheckAPI(endpoint, config.Handler, config.URLBuilder, config.RequestBodyBuilder, config.UsePOST, requestOptions)
}

// checkWithGenericClientForMarketPrice checks a provider for market price (all sources)
func (r *ProviderRegistry) checkWithGenericClientForMarketPrice(endpoint *collector.Endpoint, config ProviderConfig, checkOptions *CheckOptions) {
	// Check for WIP cases before making any requests
	if r.isWIPCase(endpoint) {
		// For WIP cases, don't make market price calls
		return
	}

	client := api.NewAPIClient()

	// Validate API key if required
	var apiKey string
	var err error
	if config.APIKeyEnvVar != "" {
		apiKey, err = client.ValidateAPIKey(config.APIKeyEnvVar, endpoint)
		if err != nil {
			return // Error already handled by ValidateAPIKey
		}
	}

	// Prepare headers
	headers := make(map[string]string)
	for key, value := range config.CustomHeaders {
		headers[key] = value
	}
	if apiKey != "" {
		// Add API key to headers (provider-specific)
		switch endpoint.RouteSolver {
		case "0x":
			headers["0x-api-key"] = apiKey
			headers["0x-version"] = "v2"
		case "1inch":
			headers["Authorization"] = fmt.Sprintf("Bearer %s", apiKey)
			headers["Content-Type"] = "application/json"
		case "hyperbloom":
			headers["api-key"] = apiKey
		case "barter":
			headers["Authorization"] = fmt.Sprintf("Bearer %s", apiKey)
		}
	}

	// Use options if provided, otherwise default to false for market price
	isBalancerSourceOnly := false // Default behavior for market price - use all sources
	if checkOptions != nil && checkOptions.IsBalancerSourceOnly != nil {
		isBalancerSourceOnly = *checkOptions.IsBalancerSourceOnly
	}
	// Configure request options
	requestOptions := api.RequestOptions{
		IsBalancerSourceOnly: isBalancerSourceOnly,
		CustomHeaders:        headers,
	}

	// Create a temporary endpoint copy for market price check to avoid overwriting the main endpoint data
	tempEndpoint := *endpoint
	client.CheckAPIForMarketPrice(&tempEndpoint, config.Handler, config.URLBuilder, config.RequestBodyBuilder, config.UsePOST, requestOptions)

	// Store the market price result in the original endpoint
	endpoint.MarketPrice = tempEndpoint.MarketPrice
}

// isWIPCase checks if the endpoint is a WIP case that should be handled specially
func (r *ProviderRegistry) isWIPCase(endpoint *collector.Endpoint) bool {
	switch endpoint.RouteSolver {
	case "1inch":
		return strings.Contains(endpoint.Name, "GyroE") ||
			strings.Contains(endpoint.Name, "Quant") ||
			endpoint.Network == "43114"
	case "odos":
		return strings.Contains(endpoint.Name, "Quant")
	default:
		return false
	}
}

// handleWIPCase handles WIP cases by setting appropriate status and message
func (r *ProviderRegistry) handleWIPCase(endpoint *collector.Endpoint) {
	endpoint.LastChecked = time.Now()

	var message string
	switch endpoint.RouteSolver {
	case "1inch":
		if strings.Contains(endpoint.Name, "GyroE") {
			message = "1inch GyroE integration WIP"
		} else if strings.Contains(endpoint.Name, "Quant") {
			message = "1inch QuantAMM integration WIP"
		} else if endpoint.Network == "43114" {
			message = "1inch network support WIP"
		}
	case "odos":
		message = "Odos QuantAMM integration WIP"
	}

	endpoint.LastStatus = "info"
	endpoint.Message = message
	fmt.Printf("%s[INFO]%s %s: API is %s%s%s\n", config.ColorYellow, config.ColorReset, endpoint.Name, config.ColorOrange, endpoint.LastStatus, config.ColorReset)
}

// Global registry instance
var GlobalRegistry *ProviderRegistry

// InitializeRegistry initializes the global provider registry
func InitializeRegistry() {
	GlobalRegistry = NewProviderRegistry()

	// Register providers using the new generic client
	GlobalRegistry.RegisterProvider("0x", ProviderConfig{
		Handler:      providers.NewZeroXHandler(),
		URLBuilder:   providers.NewZeroXURLBuilder(),
		APIKeyEnvVar: "ZEROX_API_KEY",
	})

	GlobalRegistry.RegisterProvider("paraswap", ProviderConfig{
		Handler:    providers.NewParaswapHandler(),
		URLBuilder: providers.NewParaswapURLBuilder(),
		CustomHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	})

	GlobalRegistry.RegisterProvider("1inch", ProviderConfig{
		Handler:      providers.NewOneInchHandler(),
		URLBuilder:   providers.NewOneInchURLBuilder(),
		APIKeyEnvVar: "INCH_API_KEY",
		CustomHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	})

	GlobalRegistry.RegisterProvider("hyperbloom", ProviderConfig{
		Handler:      providers.NewHyperBloomHandler(),
		URLBuilder:   providers.NewHyperBloomURLBuilder(),
		APIKeyEnvVar: "HYPERBLOOM_API_KEY",
	})

	GlobalRegistry.RegisterProvider("kyberswap", ProviderConfig{
		Handler:    providers.NewKyberSwapHandler(),
		URLBuilder: providers.NewKyberSwapURLBuilder(),
		CustomHeaders: map[string]string{
			"x-client-id": "BalancerTest",
		},
	})

	GlobalRegistry.RegisterProvider("odos", ProviderConfig{
		Handler:            &providers.OdosHandler{},
		URLBuilder:         &providers.OdosURLBuilder{},
		RequestBodyBuilder: &providers.OdosRequestBodyBuilder{},
		UsePOST:            true,
		CustomHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	})

	GlobalRegistry.RegisterProvider("balancer_sor", ProviderConfig{
		Handler:            providers.NewBalancerSORHandler(),
		URLBuilder:         providers.NewBalancerSORURLBuilder(),
		RequestBodyBuilder: providers.NewBalancerSORRequestBodyBuilder(),
		UsePOST:            true,
		CustomHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	})

	GlobalRegistry.RegisterProvider("barter", ProviderConfig{
		Handler:            providers.NewBarterHandler(),
		URLBuilder:         providers.NewBarterURLBuilder(),
		RequestBodyBuilder: providers.NewBarterRequestBodyBuilder(),
		UsePOST:            true,
		APIKeyEnvVar:       "BARTER_API_KEY",
		CustomHeaders: map[string]string{
			"Content-Type": "application/json",
			"X-Request-Id": "123", // Default request ID, can be made dynamic if needed
		},
	})
}
