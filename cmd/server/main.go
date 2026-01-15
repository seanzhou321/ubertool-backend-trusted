package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "ubertool-backend-trusted/api/gen/v1"
	api "ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"
	"database/sql"
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

	// Initialize Services
	authSvc := service.NewAuthService(store.UserRepository, store.InvitationRepository, store.JoinRequestRepository, "secret")
	userSvc := service.NewUserService(store.UserRepository)
	orgSvc := service.NewOrganizationService(store.OrganizationRepository)
	toolSvc := service.NewToolService(store.ToolRepository)
	ledgerSvc := service.NewLedgerService(store.LedgerRepository)
	noteSvc := service.NewNotificationService(store.NotificationRepository)
	rentalSvc := service.NewRentalService(store.RentalRepository, store.ToolRepository, store.LedgerRepository, store.UserRepository)
	adminSvc := service.NewAdminService(store.JoinRequestRepository, store.UserRepository, store.LedgerRepository)

	// Initialize gRPC handlers
	authHandler := api.NewAuthHandler(authSvc)
	userHandler := api.NewUserHandler(userSvc)
	orgHandler := api.NewOrganizationHandler(orgSvc)
	toolHandler := api.NewToolHandler(toolSvc)
	rentalHandler := api.NewRentalHandler(rentalSvc)
	ledgerHandler := api.NewLedgerHandler(ledgerSvc)
	notificationHandler := api.NewNotificationHandler(noteSvc)
	adminHandler := api.NewAdminHandler(adminSvc)

	// Set up gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterAuthServiceServer(s, authHandler)
	pb.RegisterUserServiceServer(s, userHandler)
	pb.RegisterOrganizationServiceServer(s, orgHandler)
	pb.RegisterToolServiceServer(s, toolHandler)
	pb.RegisterRentalServiceServer(s, rentalHandler)
	pb.RegisterLedgerServiceServer(s, ledgerHandler)
	pb.RegisterNotificationServiceServer(s, notificationHandler)
	pb.RegisterAdminServiceServer(s, adminHandler)

	reflection.Register(s)

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
