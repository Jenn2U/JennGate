package main

import (
	"fmt"
	"log"

	"github.com/Jenn2U/JennGate/internal/config"
	"github.com/Jenn2U/JennGate/internal/daemon"
	"github.com/Jenn2U/JennGate/internal/db"
	"github.com/Jenn2U/JennGate/internal/handlers"
	"github.com/Jenn2U/JennGate/internal/migrations"
	"github.com/Jenn2U/JennGate/internal/services"
	"github.com/gin-gonic/gin"
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

	// Initialize recording service
	recordingSvc, err := services.NewRecordingService(database, cfg.RecordingDir)
	if err != nil {
		log.Fatal("Failed to initialize recording service:", err)
	}
	log.Println("Recording service initialized successfully")

	// Initialize policy service
	policySvc := services.NewPolicyService()
	log.Println("Policy service initialized successfully")

	// Verify CA public key is available
	pubKey := caService.GetPublicKey()
	if len(pubKey) == 0 {
		log.Fatal("CA public key is not available")
	}
	log.Printf("CA service ready with public key (%d bytes)", len(pubKey))

	// Initialize HTTP handlers and router
	h := handlers.NewHandlers(caService, sessionSvc, recordingSvc, policySvc, database)
	router := gin.Default()
	h.RegisterRoutes(router)

	// Initialize gRPC daemon server (Phase 3a: Stub implementation)
	daemonSvc := daemon.NewDaemonServer(sessionSvc, recordingSvc, database)
	go func() {
		daemonPort := 9090 // TODO: Make configurable from JENNGATE_DAEMON_PORT (Phase 3b)
		log.Printf("Starting gRPC daemon server on port %d", daemonPort)
		if err := daemon.StartDaemonServer(daemonPort, daemonSvc); err != nil {
			log.Printf("Daemon server error: %v", err)
		}
	}()

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	log.Printf("Starting JennGate on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
