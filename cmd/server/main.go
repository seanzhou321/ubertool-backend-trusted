package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	pb "ubertool-backend-trusted/api/gen/v1"
	api "ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/api/grpc/interceptor"
	httpapi "ubertool-backend-trusted/internal/api/http"
	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/security"
	"ubertool-backend-trusted/internal/service"
	"ubertool-backend-trusted/internal/storage"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Initialize(cfg.Log.Level, cfg.Log.Format)
	logger.Info("Starting Ubertool Trusted Backend...", "log_level", cfg.Log.Level, "log_format", cfg.Log.Format)
	logger.Info("Server configuration", "address", cfg.GetServerAddress())
	logger.Info("Database configuration", "host", cfg.Database.Host, "port", cfg.Database.Port, "database", cfg.Database.Database, "user", cfg.Database.User)
	logger.Info("SMTP configuration", "host", cfg.SMTP.Host, "port", cfg.SMTP.Port)

	// Initialize Database
	logger.Debug("Connecting to database...", "connection_string", fmt.Sprintf("%s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Database))
	db, err := sql.Open("postgres", cfg.GetDatabaseConnectionString())
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Error("Failed to ping database", "error", err)
		log.Fatalf("Failed to ping database: %v", err)
	}
	logger.Info("Database connection established")

	// Initialize Repositories
	store := postgres.NewStore(db)

	// Initialize Notification + Push services (created first so other services can depend on noteSvc)
	fcmClient, fcmErr := config.InitFirebase(cfg.FirebaseKeyPath)
	switch {
	case cfg.FirebaseKeyPath == "":
		logger.Info("FCM disabled (firebase_key_path not configured)")
	case fcmErr != nil:
		logger.Warn("FCM client unavailable — push notifications disabled", "error", fcmErr)
		fcmClient = nil
	default:
		logger.Info("Firebase Messaging Service connection established")
	}
	noteSvc := service.NewNotificationService(store.NotificationRepository, store.FcmTokenRepository)
	pushSvc := service.NewPushNotificationService(fcmClient, store.FcmTokenRepository)
	noteSvc.SetPushService(pushSvc)

	// Initialize Security
	tokenManager := security.NewTokenManager(cfg.JWT.Secret)
	authInterceptor := interceptor.NewAuthInterceptor(tokenManager)

	// Initialize Storage Service
	var storageService storage.StorageInterface
	if cfg.Storage.Type == "s3" {
		logger.Info("Using S3 storage", "bucket", cfg.Storage.S3Bucket, "region", cfg.Storage.S3Region)
		s3Storage, err := storage.NewS3StorageService(context.Background(), cfg.Storage.S3Bucket, cfg.Storage.S3Region)
		if err != nil {
			logger.Error("Failed to initialize S3 storage", "error", err)
			log.Fatalf("Failed to initialize S3 storage: %v", err)
		}
		storageService = s3Storage
	} else {
		logger.Info("Using mock storage (local filesystem)", "upload_dir", cfg.Storage.UploadDir)
		mockStorage, err := storage.NewMockStorageService(cfg.Storage.BaseURL, cfg.Storage.UploadDir)
		if err != nil {
			logger.Error("Failed to initialize mock storage", "error", err)
			log.Fatalf("Failed to initialize mock storage: %v", err)
		}
		storageService = mockStorage
	}

	// Initialize Image Storage Service
	imageSvc := service.NewImageStorageService(
		store.ToolRepository,
		store.UserRepository,
		store.OrganizationRepository,
		storageService,
	)

	// Initialize Email Service (wrapped in async worker pool so sends never block gRPC handlers)
	var baseEmailSvc service.EmailService
	if cfg.EmailProvider == "ses" {
		logger.Info("Using AWS SES email provider", "region", cfg.SES.Region, "from", cfg.SES.From)
		sesEmailSvc, err := service.NewSESEmailService(cfg.SES.Region, cfg.SES.From)
		if err != nil {
			logger.Error("Failed to initialize SES email service", "error", err)
			log.Fatalf("Failed to initialize SES email service: %v", err)
		}
		baseEmailSvc = sesEmailSvc
	} else {
		logger.Info("Using SMTP email provider", "host", cfg.SMTP.Host)
		baseEmailSvc = service.NewEmailService(
			cfg.SMTP.Host,
			fmt.Sprintf("%d", cfg.SMTP.Port),
			cfg.SMTP.User,
			cfg.SMTP.Password,
			cfg.SMTP.From,
		)
	}
	emailSvc := service.NewAsyncEmailService(baseEmailSvc)

	// Initialize Services
	authSvc := service.NewAuthService(
		store.UserRepository,
		store.InvitationRepository,
		store.JoinRequestRepository,
		store.OrganizationRepository,
		noteSvc,
		emailSvc,
		cfg.JWT.Secret,
		store.FcmTokenRepository,
		store.PendingCredentialsRepository,
		store.LegalConsentRepository,
		cfg.TwoFA,
	)
	userSvc := service.NewUserService(store.UserRepository, store.OrganizationRepository)
	orgSvc := service.NewOrganizationService(store.OrganizationRepository, store.UserRepository, store.InvitationRepository, noteSvc, emailSvc, pushSvc)
	toolSvc := service.NewToolService(store.ToolRepository, store.UserRepository, store.OrganizationRepository)
	ledgerSvc := service.NewLedgerService(store.LedgerRepository)
	rentalSvc := service.NewRentalService(
		store.RentalRepository,
		store.ToolRepository,
		store.LedgerRepository,
		store.UserRepository,
		store.OrganizationRepository,
		emailSvc,
		noteSvc,
	)
	adminSvc := service.NewAdminService(
		store.JoinRequestRepository,
		store.UserRepository,
		store.LedgerRepository,
		store.OrganizationRepository,
		store.InvitationRepository,
		emailSvc,
	)
	billSplitSvc := service.NewBillSplitService(
		store.BillRepository,
		store.UserRepository,
		store.OrganizationRepository,
		noteSvc,
		emailSvc,
	)

	// Initialize gRPC handlers
	authHandler := api.NewAuthHandler(authSvc)
	userHandler := api.NewUserHandler(userSvc)
	orgHandler := api.NewOrganizationHandler(orgSvc, cfg.Features.AllowAPIOrganizationCreation)
	toolHandler := api.NewToolHandler(toolSvc)
	rentalHandler := api.NewRentalHandler(rentalSvc, userSvc, toolSvc, orgSvc)
	ledgerHandler := api.NewLedgerHandler(ledgerSvc)
	notificationHandler := api.NewNotificationHandler(noteSvc)
	adminHandler := api.NewAdminHandler(adminSvc)
	imageHandler := api.NewImageStorageHandler(imageSvc)
	billSplitHandler := api.NewBillSplitHandler(billSplitSvc, userSvc)

	// Set up gRPC server
	lis, err := net.Listen("tcp", cfg.GetServerAddress())
	if err != nil {
		logger.Error("Failed to listen", "error", err, "address", cfg.GetServerAddress())
		log.Fatalf("Failed to listen: %v", err)
	}

	if cfg.RateLimit.Disabled {
		logger.Warn("Login/Verify2FA rate limiting is DISABLED via config — must not be used in production")
	}
	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			interceptor.NewRateLimitInterceptor(security.NewIPRateLimiter(), cfg.RateLimit.Disabled).Unary(),
			authInterceptor.Unary(),
		),
	}
	if cfg.TLS.Enabled {
		creds, err := credentials.NewServerTLSFromFile(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			logger.Error("Failed to load TLS credentials", "error", err)
			log.Fatalf("Failed to load TLS credentials: %v", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
		logger.Info("TLS enabled", "cert_file", cfg.TLS.CertFile)
	}
	s := grpc.NewServer(serverOpts...)
	pb.RegisterAuthServiceServer(s, authHandler)
	pb.RegisterUserServiceServer(s, userHandler)
	pb.RegisterOrganizationServiceServer(s, orgHandler)
	pb.RegisterToolServiceServer(s, toolHandler)
	pb.RegisterRentalServiceServer(s, rentalHandler)
	pb.RegisterLedgerServiceServer(s, ledgerHandler)
	pb.RegisterNotificationServiceServer(s, notificationHandler)
	pb.RegisterAdminServiceServer(s, adminHandler)
	pb.RegisterImageStorageServiceServer(s, imageHandler)
	pb.RegisterBillSplitServiceServer(s, billSplitHandler)

	// Register reflection service (disabled in production via server.grpc_reflection: false)
	if cfg.Server.GRPCReflection {
		reflection.Register(s)
		logger.Info("gRPC reflection enabled")
	} else {
		logger.Info("gRPC reflection disabled")
	}

	// Set up HTTP server for mock storage endpoints (only when using mock storage)
	if cfg.Storage.Type != "s3" {
		mockStorage := storageService.(*storage.MockStorageService)
		router := mux.NewRouter()
		httpapi.RegisterMockStorageRoutes(router, mockStorage)

		httpPort := cfg.Server.Port + 1
		httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, httpPort)
		go func() {
			logger.Info("HTTP server for mock storage listening", "address", httpAddr)
			if err := http.ListenAndServe(httpAddr, router); err != nil {
				logger.Error("HTTP server error", "error", err)
			}
		}()
	}

	logger.Info("gRPC server listening", "address", cfg.GetServerAddress())

	// Run gRPC server in background; block until OS signal.
	serveErr := make(chan error, 1)
	go func() {
		if err := s.Serve(lis); err != nil {
			serveErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serveErr:
		logger.Error("gRPC server exited unexpectedly", "error", err)
		log.Fatalf("gRPC server error: %v", err)
	case sig := <-quit:
		logger.Info("Shutdown signal received", "signal", sig)
	}

	// Stop accepting new RPCs; wait for in-flight handlers to complete.
	s.GracefulStop()
	logger.Info("gRPC server stopped")

	// Drain any in-flight FCM send goroutines (including sleeping retries).
	// Allow up to 15 seconds before giving up so critical pushes are delivered.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer drainCancel()
	if err := pushSvc.Shutdown(drainCtx); err != nil {
		logger.Warn("FCM drain timed out; some in-flight pushes may be lost", "error", err)
	} else {
		logger.Info("FCM goroutines drained")
	}

	// Drain in-flight async email sends.
	if err := emailSvc.Shutdown(drainCtx); err != nil {
		logger.Warn("Email drain timed out; some in-flight emails may be lost", "error", err)
	} else {
		logger.Info("Email goroutines drained")
	}
}
