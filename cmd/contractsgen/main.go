package main

import (
	"log"
	"os"

	"orbyte/internal/platform/contractartifacts"
)

func main() {
	generated, err := contractartifacts.Generate()
	if err != nil {
		log.Printf("contract generation error: %v", err)
		os.Exit(1)
	}
	if err := contractartifacts.Write(".", generated); err != nil {
		log.Printf("contract write error: %v", err)
		os.Exit(1)
	}
}
