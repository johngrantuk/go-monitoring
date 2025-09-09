package api

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go-monitoring/config"
	"go-monitoring/internal/collector"
	"go-monitoring/notifications"
)

// RequestOptions contains configuration for API requests
type RequestOptions struct {
	IsBalancerSourceOnly bool
	CustomHeaders        map[string]string
}

// APIResponse represents a generic API response
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ResponseHandler defines how to process API responses
type ResponseHandler interface {
	HandleResponse(response *APIResponse, endpoint *collector.Endpoint) error
	GetIgnoreList(network string) (string, error)
}

// CustomResponseHandler allows for custom response handling without ignore list
type CustomResponseHandler interface {
	HandleResponse(response *APIResponse, endpoint *collector.Endpoint) error
}

// URLBuilder defines how to build URLs for different providers
type URLBuilder interface {
	BuildURL(endpoint *collector.Endpoint, options RequestOptions) (string, error)
}

// RequestBodyBuilder defines how to build JSON request bodies for POST requests
type RequestBodyBuilder interface {
	BuildRequestBody(endpoint *collector.Endpoint, options RequestOptions) ([]byte, error)
}

// APIClient handles HTTP requests and provides common functionality
type APIClient struct {
	client *http.Client
}

// NewAPIClient creates a new API client with default configuration
func NewAPIClient() *APIClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	return &APIClient{client: client}
}

// MakeRequest performs an HTTP request and handles common error scenarios
func (c *APIClient) MakeRequest(endpoint *collector.Endpoint, baseURL string, options RequestOptions) (*APIResponse, error) {
	return c.MakeGETRequest(endpoint, baseURL, options)
}

// MakeGETRequest performs a GET HTTP request
func (c *APIClient) MakeGETRequest(endpoint *collector.Endpoint, baseURL string, options RequestOptions) (*APIResponse, error) {
	// Update endpoint timestamp
	endpoint.LastChecked = time.Now()

	// Create HTTP request
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		c.handleError(endpoint, "error", fmt.Sprintf("Error creating request: %v", err))
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add custom headers
	for key, value := range options.CustomHeaders {
		req.Header.Add(key, value)
	}

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		c.handleError(endpoint, "down", fmt.Sprintf("Error sending request: %v", err))
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.handleError(endpoint, "down", fmt.Sprintf("Error reading response: %v", err))
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}, nil
}

// MakePOSTRequest performs a POST HTTP request with JSON body
func (c *APIClient) MakePOSTRequest(endpoint *collector.Endpoint, baseURL string, requestBody []byte, options RequestOptions) (*APIResponse, error) {
	// Update endpoint timestamp
	endpoint.LastChecked = time.Now()

	// Create HTTP request
	req, err := http.NewRequest("POST", baseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		c.handleError(endpoint, "error", fmt.Sprintf("Error creating request: %v", err))
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add custom headers
	for key, value := range options.CustomHeaders {
		req.Header.Add(key, value)
	}

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		c.handleError(endpoint, "down", fmt.Sprintf("Error sending request: %v", err))
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.handleError(endpoint, "down", fmt.Sprintf("Error reading response: %v", err))
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}, nil
}

// CheckAPI performs a complete API check using the provided handler and URL builder
func (c *APIClient) CheckAPI(endpoint *collector.Endpoint, handler ResponseHandler, urlBuilder URLBuilder, requestBodyBuilder RequestBodyBuilder, usePOST bool, options RequestOptions) {
	// Update endpoint timestamp
	endpoint.LastChecked = time.Now()

	var response *APIResponse

	if usePOST && requestBodyBuilder != nil {
		// Build the request body for POST request
		requestBody, err := requestBodyBuilder.BuildRequestBody(endpoint, options)
		if err != nil {
			c.handleError(endpoint, "error", fmt.Sprintf("Error building request body: %v", err))
			return
		}

		// Build the URL using the provider-specific builder
		fullURL, err := urlBuilder.BuildURL(endpoint, options)
		if err != nil {
			c.handleError(endpoint, "error", fmt.Sprintf("Error building URL: %v", err))
			return
		}
		fmt.Println("URL: ", fullURL)

		// Make the POST request
		response, err = c.MakePOSTRequest(endpoint, fullURL, requestBody, options)
		if err != nil {
			// Error already handled in MakePOSTRequest
			return
		}
	} else {
		// Build the URL using the provider-specific builder
		fullURL, err := urlBuilder.BuildURL(endpoint, options)
		if err != nil {
			c.handleError(endpoint, "error", fmt.Sprintf("Error building URL: %v", err))
			return
		}
		fmt.Println("URL: ", fullURL)

		// Make the GET request
		response, err = c.MakeGETRequest(endpoint, fullURL, options)
		if err != nil {
			// Error already handled in MakeGETRequest
			return
		}
	}

	// Handle the response using the provided handler
	if err := handler.HandleResponse(response, endpoint); err != nil {
		c.handleError(endpoint, "down", fmt.Sprintf("Error handling response: %v", err))
		return
	}

	// Success
	endpoint.LastStatus = "up"
	endpoint.Message = "Ok"
	fmt.Printf("%s[SUCCESS]%s %s: API is %s%s%s\n", config.ColorGreen, config.ColorReset, endpoint.Name, config.ColorGreen, endpoint.LastStatus, config.ColorReset)
}

// handleError updates endpoint status and sends notifications for errors
func (c *APIClient) handleError(endpoint *collector.Endpoint, status, message string) {
	endpoint.LastStatus = status
	endpoint.Message = message
	fmt.Printf("%s[ERROR]%s %s: %s\n", config.ColorRed, config.ColorReset, endpoint.Name, message)
	notifications.SendEmail(fmt.Sprintf("[%s] %s", endpoint.Name, message))
}

// ValidateAPIKey checks if a required API key is present
func (c *APIClient) ValidateAPIKey(envVar string, endpoint *collector.Endpoint) (string, error) {
	apiKey := os.Getenv(envVar)
	if apiKey == "" {
		c.handleError(endpoint, "error", fmt.Sprintf("%s environment variable not set", envVar))
		return "", fmt.Errorf("%s environment variable not set", envVar)
	}
	return apiKey, nil
}
