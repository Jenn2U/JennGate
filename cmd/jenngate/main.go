package main

import (
	"fmt"
	"log"

	"github.com/Jenn2U/JennGate/internal/config"
	"github.com/Jenn2U/JennGate/internal/db"
	"github.com/Jenn2U/JennGate/internal/migrations"
	"github.com/Jenn2U/JennGate/internal/services"
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

	// Run pending migrations
	connString := "postgresql://" + cfg.DBUser + ":" + cfg.DBPassword + "@" +
		cfg.DBHost + ":" + fmt.Sprint(cfg.DBPort) + "/" + cfg.DBName +
		"?sslmode=" + cfg.SSLMode
	if err := migrations.RunMigrations(connString); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	log.Println("Migrations completed successfully")

	// Initialize services
	caService, err := services.NewCAService(database)
	if err != nil {
		log.Fatal("Failed to initialize CA service:", err)
	}
	log.Println("CA service initialized successfully")

	sessionSvc := services.NewSessionService(database)
	log.Printf("Session service initialized successfully (db: %v)", sessionSvc != nil)

	// TODO: Initialize recordings service

	// Verify CA public key is available
	pubKey := caService.GetPublicKey()
	if len(pubKey) == 0 {
		log.Fatal("CA public key is not available")
	}
	log.Printf("CA service ready with public key (%d bytes)", len(pubKey))
}
