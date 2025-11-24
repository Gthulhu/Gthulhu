package main

import (
	"context"
	"log"
	"os"

	"github.com/Gthulhu/Gthulhu/gtp5g_operator/pkg/api"
)

func main() {
	log.Println("=== Testing API Client ===")

	// Set up environment
	apiEndpoint := os.Getenv("API_ENDPOINT")
	if apiEndpoint == "" {
		apiEndpoint = "http://localhost:8080"
	}
	
	publicKeyPath := os.Getenv("PUBLIC_KEY_PATH")
	if publicKeyPath == "" {
		publicKeyPath = "/tmp/jwt_public_key.pem"
	}
	
	log.Printf("API Endpoint: %s", apiEndpoint)
	log.Printf("Public Key: %s", publicKeyPath)

	// Create API client
	client := api.NewClient(apiEndpoint, publicKeyPath)
	ctx := context.Background()

	// Test data: simulate some nr-gnb PIDs
	testPIDs := map[int]bool{
		12345: true,
		67890: true,
		99999: true,
	}

	log.Printf("Sending strategies for %d PIDs...", len(testPIDs))
	
	// Send strategies
	err := client.SendStrategies(ctx, testPIDs, true, 20000000)
	if err != nil {
		log.Fatalf("❌ Failed to send strategies: %v", err)
	}

	log.Println("✅ Strategies sent successfully!")
	
	// Test empty PID list
	log.Println("\nTesting empty PID list...")
	emptyPIDs := map[int]bool{}
	err = client.SendStrategies(ctx, emptyPIDs, true, 20000000)
	if err != nil {
		log.Fatalf("❌ Failed to send empty strategies: %v", err)
	}
	
	log.Println("✅ Empty strategies sent successfully!")
	log.Println("\n=== API Client Test Passed ===")
}
