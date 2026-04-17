package main

import (
	"log"

	"github.com/Jenn2U/JennGate/internal/config"
	"github.com/Jenn2U/JennGate/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}
	log.Printf("Loaded config: %v", cfg)

	// Initialize database
	database, err := db.InitDB(cfg)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer func() {
		if err := db.Close(database); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	log.Println("Database initialized successfully")
}
