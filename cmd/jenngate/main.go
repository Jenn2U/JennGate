package main

import (
	"log"

	"github.com/Jenn2U/JennGate/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}
	log.Printf("Loaded config: %v", cfg)
}
