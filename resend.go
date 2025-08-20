package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"go-monitoring/config"

	"github.com/resend/resend-go/v2"
)

func sendEmail(message string) {
	// Check if email sending is enabled
	if !enableEmailSending {
		fmt.Printf("%s[INFO]%s: Email sending is disabled\n", config.ColorYellow, config.ColorReset)
		return
	}

	// Get API key from environment variable
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		fmt.Printf("%s[ERROR]%s: RESEND_API_KEY environment variable not set\n", config.ColorRed, config.ColorReset)
		return
	}

	// Set global HTTP transport to skip certificate verification
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	client := resend.NewClient(apiKey)

	params := &resend.SendEmailRequest{
		From:    "onboarding@resend.dev",
		To:      []string{"john@balancerlabs.dev"},
		Subject: "Aggregator Monitor",
		Html:    "<p>" + message + "</p>",
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		fmt.Println("Error sending email:", err)
	} else {
		fmt.Println("Email sent successfully:", sent)
	}
}
