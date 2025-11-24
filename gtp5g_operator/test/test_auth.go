package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Gthulhu/Gthulhu/gtp5g_operator/pkg/auth"
)

func main() {
	log.Println("=== Testing JWT Authentication ===")

	// Set public key path
	publicKeyPath := os.Getenv("PUBLIC_KEY_PATH")
	if publicKeyPath == "" {
		publicKeyPath = "../api/config/jwt_public_key.pem"
	}
	
	// Check if public key exists
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		log.Fatalf("Public key not found: %s", publicKeyPath)
	}
	
	log.Printf("Public key path: %s", publicKeyPath)

	// Create auth client
	apiEndpoint := os.Getenv("API_ENDPOINT")
	if apiEndpoint == "" {
		apiEndpoint = "http://localhost:8080"
	}
	
	client := auth.NewClient(apiEndpoint, publicKeyPath)
	log.Printf("API endpoint: %s", apiEndpoint)

	// Get token
	log.Println("Requesting JWT token...")
	token, err := client.GetToken()
	if err != nil {
		log.Fatalf("Failed to get token: %v", err)
	}

	log.Println("✅ JWT token obtained successfully!")
	fmt.Printf("Token (first 50 chars): %s...\n", token[:min(50, len(token))])
	fmt.Printf("Token length: %d\n", len(token))

	// Test token refresh (get again)
	log.Println("\nTesting token cache...")
	token2, err := client.GetToken()
	if err != nil {
		log.Fatalf("Failed to get cached token: %v", err)
	}

	if token == token2 {
		log.Println("✅ Token cache working correctly!")
	} else {
		log.Println("⚠️  Token changed (cache may not be working)")
	}

	log.Println("\n=== Authentication Test Passed ===")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
