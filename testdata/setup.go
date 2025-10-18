package main

import (
	"fmt"
	"log"
	"os"

	"github.com/fedutinova/smartheart/testdata"
)

func main() {
	// Create test data directory
	testDir := "./testdata/images"

	fmt.Println("Setting up test data...")

	// Create test images
	if err := testdata.SaveTestImagesToFile(testDir); err != nil {
		log.Fatalf("Failed to save test images: %v", err)
	}

	fmt.Printf("Test images saved to: %s\n", testDir)

	// Print test image information
	info := testdata.GetTestImageInfo()
	fmt.Println("\nTest image information:")
	for name, details := range info {
		fmt.Printf("  %s: %+v\n", name, details)
	}

	// Create additional test directories
	dirs := []string{
		"./uploads",
		"./logs",
		"./temp",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Warning: Failed to create directory %s: %v", dir, err)
		} else {
			fmt.Printf("Created directory: %s\n", dir)
		}
	}

	// Create .env.example if it doesn't exist
	envExample := ".env.example"
	if _, err := os.Stat(envExample); os.IsNotExist(err) {
		envContent := `# SmartHeart Configuration

# Server
HTTP_ADDR=:8080

# Database
DATABASE_URL=postgres://user:password@localhost:5432/smartheart?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379

# JWT
JWT_SECRET=development-secret-change-in-production
JWT_ISSUER=smartheart
JWT_TTL_ACCESS=15m
JWT_TTL_REFRESH=168h

# Storage
STORAGE_MODE=local
LOCAL_STORAGE_DIR=./uploads
LOCAL_STORAGE_URL=http://localhost:8080/files

# S3 (if using S3 storage)
S3_BUCKET=smartheart-files
S3_ENDPOINT=http://localhost:4566
S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
S3_FORCE_PATH_STYLE=true

# OpenAI
OPENAI_API_KEY=your_openai_api_key_here

# Queue
QUEUE_WORKERS=4
QUEUE_BUFFER=1024
JOB_MAX_DURATION=30s

# Development
DEBUG=true
LOG_LEVEL=debug
`

		if err := os.WriteFile(envExample, []byte(envContent), 0644); err != nil {
			log.Printf("Warning: Failed to create .env.example: %v", err)
		} else {
			fmt.Printf("Created: %s\n", envExample)
		}
	}

	fmt.Println("\nTest data setup complete!")

	// Print test examples
	testdata.PrintTestExamples()

	fmt.Println("\nNext steps:")
	fmt.Println("1. Copy .env.example to .env and configure your settings")
	fmt.Println("2. Run 'make docker-compose-up' to start dependencies")
	fmt.Println("3. Run 'make test' to run all tests")
	fmt.Println("4. Run 'make run' to start the application")
	fmt.Println("5. Run 'go test ./...' to run specific tests")
}
