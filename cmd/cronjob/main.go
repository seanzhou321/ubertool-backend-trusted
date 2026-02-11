package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"

	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/jobs"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/scheduler"
	"ubertool-backend-trusted/internal/service"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config/config.dev.yaml", "Path to configuration file")
	runOnce := flag.String("run-once", "", "Run a specific job once and exit (e.g., 'mark-overdue-rentals', 'all-nightly', 'all-monthly')")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Initialize(cfg.Log.Level, cfg.Log.Format)
	logger.Info("Starting Ubertool Cronjob Runner...", "log_level", cfg.Log.Level)

	// Initialize Database
	logger.Info("Connecting to database...", "host", cfg.Database.Host, "port", cfg.Database.Port)
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

	// Initialize Services
	emailService := service.NewEmailService(
		cfg.SMTP.Host,
		fmt.Sprintf("%d", cfg.SMTP.Port),
		cfg.SMTP.User,
		cfg.SMTP.Password,
		cfg.SMTP.From,
	)

	rentalService := service.NewRentalService(
		store.RentalRepository,
		store.ToolRepository,
		store.LedgerRepository,
		store.UserRepository,
		emailService,
		store.NotificationRepository,
	)

	ledgerService := service.NewLedgerService(
		store.LedgerRepository,
	)

	orgService := service.NewOrganizationService(
		store.OrganizationRepository,
		store.UserRepository,
		store.InvitationRepository,
		store.NotificationRepository,
	)

	userService := service.NewUserService(
		store.UserRepository,
		store.OrganizationRepository,
	)

	jobServices := &jobs.Services{
		Email:  emailService,
		Rental: rentalService,
		Ledger: ledgerService,
		Org:    orgService,
		User:   userService,
	}

	// Initialize Job Runner
	jobRunner := jobs.NewJobRunner(db, store, jobServices, cfg)

	// Check if running a single job
	if *runOnce != "" {
		logger.Info("Running job once", "job", *runOnce)
		runJobOnce(jobRunner, *runOnce)
		logger.Info("Job execution completed", "job", *runOnce)
		return
	}

	// Initialize Scheduler
	cronScheduler := scheduler.NewScheduler(jobRunner)

	// Start scheduler
	cronScheduler.Start()
	logger.Info("Cronjob scheduler is running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	logger.Info("Shutting down cronjob scheduler...")
	cronScheduler.Stop()
	logger.Info("Cronjob scheduler stopped. Goodbye!")
}

// runJobOnce runs a specific job once and exits
func runJobOnce(jobRunner *jobs.JobRunner, jobName string) {
	switch jobName {
	case "mark-overdue-rentals":
		jobRunner.MarkOverdueRentals()
	case "send-overdue-reminders":
		jobRunner.SendOverdueReminders()
	case "send-bill-reminders":
		jobRunner.SendBillReminders()
	case "check-overdue-bills":
		jobRunner.CheckOverdueBills()
	case "resolve-disputed-bills":
		jobRunner.ResolveDisputedBills()
	case "take-balance-snapshots":
		jobRunner.TakeBalanceSnapshots()
	case "perform-bill-splitting":
		jobRunner.PerformBillSplitting()
	case "all-nightly":
		jobRunner.RunAllNightlyJobs()
	case "all-monthly":
		jobRunner.RunAllMonthlyJobs()
	default:
		logger.Error("Unknown job name", "job", jobName)
		fmt.Printf("Available jobs:\n")
		fmt.Printf("  - mark-overdue-rentals\n")
		fmt.Printf("  - send-overdue-reminders\n")
		fmt.Printf("  - send-bill-reminders\n")
		fmt.Printf("  - check-overdue-bills\n")
		fmt.Printf("  - resolve-disputed-bills\n")
		fmt.Printf("  - take-balance-snapshots\n")
		fmt.Printf("  - perform-bill-splitting\n")
		fmt.Printf("  - all-nightly\n")
		fmt.Printf("  - all-monthly\n")
		os.Exit(1)
	}
}
