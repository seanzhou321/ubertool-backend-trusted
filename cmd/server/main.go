package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "ubertool-backend-trusted/api/gen/v1"
	api "ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/api/grpc/interceptor"
	httpapi "ubertool-backend-trusted/internal/api/http"
	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/security"
	"ubertool-backend-trusted/internal/service"
	"ubertool-backend-trusted/internal/storage"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config/config.dev.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting Ubertool Trusted Backend...")
	log.Printf("Server: %s", cfg.GetServerAddress())
	log.Printf("Database: %s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)
	log.Printf("SMTP: %s:%d", cfg.SMTP.Host, cfg.SMTP.Port)

	// Initialize Database
	db, err := sql.Open("postgres", cfg.GetDatabaseConnectionString())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Database connection established")

	// Initialize Repositories
	store := postgres.NewStore(db)

	// Initialize Security
	tokenManager := security.NewTokenManager(cfg.JWT.Secret)
	authInterceptor := interceptor.NewAuthInterceptor(tokenManager)

	// Initialize Storage Service
	var storageService storage.StorageInterface
	if cfg.Storage.Type == "" || cfg.Storage.Type == "mock" {
		log.Println("Using mock storage (local filesystem)")
		mockStorage, err := storage.NewMockStorageService(cfg.Storage.BaseURL, cfg.Storage.UploadDir)
		if err != nil {
			log.Fatalf("Failed to initialize mock storage: %v", err)
		}
		storageService = mockStorage
	} else {
		log.Fatalf("Storage type '%s' not yet implemented", cfg.Storage.Type)
	}

	// Initialize Image Storage Service
	imageSvc := service.NewImageStorageService(
		store.ToolRepository,
		store.UserRepository,
		store.OrganizationRepository,
		storageService,
	)

	// Initialize Email Service
	emailSvc := service.NewEmailService(
		cfg.SMTP.Host,
		fmt.Sprintf("%d", cfg.SMTP.Port),
		cfg.SMTP.User,
		cfg.SMTP.Password,
		cfg.SMTP.From,
	)

	// Initialize Services
	authSvc := service.NewAuthService(
		store.UserRepository,
		store.InvitationRepository,
		store.JoinRequestRepository,
		store.OrganizationRepository,
		store.NotificationRepository,
		emailSvc,
		cfg.JWT.Secret,
	)
	userSvc := service.NewUserService(store.UserRepository, store.OrganizationRepository)
	orgSvc := service.NewOrganizationService(store.OrganizationRepository, store.UserRepository, store.InvitationRepository, store.NotificationRepository)
	toolSvc := service.NewToolService(store.ToolRepository, store.UserRepository)
	ledgerSvc := service.NewLedgerService(store.LedgerRepository)
	noteSvc := service.NewNotificationService(store.NotificationRepository)
	rentalSvc := service.NewRentalService(
		store.RentalRepository,
		store.ToolRepository,
		store.LedgerRepository,
		store.UserRepository,
		emailSvc,
		store.NotificationRepository,
	)
	adminSvc := service.NewAdminService(
		store.JoinRequestRepository,
		store.UserRepository,
		store.LedgerRepository,
		store.OrganizationRepository,
		store.InvitationRepository,
		emailSvc,
	)

	// Initialize gRPC handlers
	authHandler := api.NewAuthHandler(authSvc)
	userHandler := api.NewUserHandler(userSvc)
	orgHandler := api.NewOrganizationHandler(orgSvc)
	toolHandler := api.NewToolHandler(toolSvc)
	rentalHandler := api.NewRentalHandler(rentalSvc)
	ledgerHandler := api.NewLedgerHandler(ledgerSvc)
	notificationHandler := api.NewNotificationHandler(noteSvc)
	adminHandler := api.NewAdminHandler(adminSvc)
	imageHandler := api.NewImageStorageHandler(imageSvc)

	// Set up gRPC server
	lis, err := net.Listen("tcp", cfg.GetServerAddress())
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor.Unary()),
	)

	// Register services
	pb.RegisterAuthServiceServer(s, authHandler)
	pb.RegisterUserServiceServer(s, userHandler)
	pb.RegisterOrganizationServiceServer(s, orgHandler)
	pb.RegisterToolServiceServer(s, toolHandler)
	pb.RegisterRentalServiceServer(s, rentalHandler)
	pb.RegisterLedgerServiceServer(s, ledgerHandler)
	pb.RegisterNotificationServiceServer(s, notificationHandler)
	pb.RegisterAdminServiceServer(s, adminHandler)
	pb.RegisterImageStorageServiceServer(s, imageHandler)

	// Register reflection service for grpcurl
	reflection.Register(s)

	// Set up HTTP server for mock storage endpoints (if using mock storage)
	if cfg.Storage.Type == "" || cfg.Storage.Type == "mock" {
		mockStorage := storageService.(*storage.MockStorageService)
		router := mux.NewRouter()
		httpapi.RegisterMockStorageRoutes(router, mockStorage)

		// Start HTTP server in a goroutine
		httpPort := cfg.Server.Port + 1 // Use next port for HTTP
		httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, httpPort)
		go func() {
			log.Printf("HTTP server for mock storage listening on %s", httpAddr)
			if err := http.ListenAndServe(httpAddr, router); err != nil {
				log.Printf("HTTP server error: %v", err)
			}
		}()
	}

	log.Printf("gRPC server listening on %s", cfg.GetServerAddress())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
