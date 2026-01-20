package main

import (
	"database/sql"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "ubertool-backend-trusted/api/gen/v1"
	api "ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/api/grpc/interceptor"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/security"
	"ubertool-backend-trusted/internal/service"

	_ "github.com/lib/pq"
)

func main() {
	// Initialize Database
	db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/ubertool?sslmode=disable")
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Repositories
	store := postgres.NewStore(db)

	jwtSecret := "secret"

	// Initialize Security
	tokenManager := security.NewTokenManager(jwtSecret)
	authInterceptor := interceptor.NewAuthInterceptor(tokenManager)

	// Initialize Services
	authSvc := service.NewAuthService(store.UserRepository, store.InvitationRepository, store.JoinRequestRepository, jwtSecret)
	userSvc := service.NewUserService(store.UserRepository)
	orgSvc := service.NewOrganizationService(store.OrganizationRepository)
	toolSvc := service.NewToolService(store.ToolRepository)
	ledgerSvc := service.NewLedgerService(store.LedgerRepository)
	noteSvc := service.NewNotificationService(store.NotificationRepository)
	rentalSvc := service.NewRentalService(store.RentalRepository, store.ToolRepository, store.LedgerRepository, store.UserRepository)
	
	// Create uploads directory if not exists
	uploadDir := "./uploads"
	imageSvc := service.NewImageStorageService(store.ToolRepository, uploadDir)

	// Gmail Configuration from Environment Variables
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"
	smtpUser := "your-email@gmail.com" // Replace or use env
	smtpPass := "your-app-password"    // Replace or use env
	smtpFrom := "your-email@gmail.com" // Replace or use env

	emailSvc := service.NewEmailService(smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom)
	adminSvc := service.NewAdminService(store.JoinRequestRepository, store.UserRepository, store.LedgerRepository, store.OrganizationRepository, store.InvitationRepository, emailSvc)

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
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor.Unary()),
	)
	pb.RegisterAuthServiceServer(s, authHandler)
	pb.RegisterUserServiceServer(s, userHandler)
	pb.RegisterOrganizationServiceServer(s, orgHandler)
	pb.RegisterToolServiceServer(s, toolHandler)
	pb.RegisterRentalServiceServer(s, rentalHandler)
	pb.RegisterLedgerServiceServer(s, ledgerHandler)
	pb.RegisterNotificationServiceServer(s, notificationHandler)
	pb.RegisterAdminServiceServer(s, adminHandler)
	pb.RegisterImageStorageServiceServer(s, imageHandler)

	reflection.Register(s)

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
