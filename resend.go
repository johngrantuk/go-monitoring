package main

import (
	"fmt"
	"github.com/resend/resend-go/v2"
	"os"
)

func sendEmail(message string) {
	// Get API key from environment variable
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		fmt.Printf("%s[ERROR]%s: RESEND_API_KEY environment variable not set\n", colorRed, colorReset)
		return
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
