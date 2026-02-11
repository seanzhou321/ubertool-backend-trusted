package jobs

import (
	"database/sql"

	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"
)

// JobRunner coordinates all scheduled jobs
type JobRunner struct {
	db       *sql.DB
	store    *postgres.Store
	services *Services
	config   *config.Config
}

// Services holds all service dependencies needed by jobs
type Services struct {
	Email  service.EmailService
	Rental service.RentalService
	Ledger service.LedgerService
	Org    service.OrganizationService
	User   service.UserService
}

// NewJobRunner creates a new job runner with all dependencies
func NewJobRunner(db *sql.DB, store *postgres.Store, services *Services, cfg *config.Config) *JobRunner {
	return &JobRunner{
		db:       db,
		store:    store,
		services: services,
		config:   cfg,
	}
}

// runWithRecovery wraps job execution with panic recovery
func (jr *JobRunner) runWithRecovery(jobName string, jobFunc func()) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Job panicked", "job", jobName, "panic", r)
		}
	}()

	logger.Info("Starting job", "job", jobName)
	jobFunc()
	logger.Info("Job completed", "job", jobName)
}

// RunAllNightlyJobs runs all nightly jobs (for manual execution)
func (jr *JobRunner) RunAllNightlyJobs() {
	jr.MarkOverdueRentals()
	jr.SendOverdueReminders()
	jr.SendBillReminders()
}

// RunAllMonthlyJobs runs all monthly jobs (for manual execution)
func (jr *JobRunner) RunAllMonthlyJobs() {
	jr.ResolveDisputedBills()
	jr.TakeBalanceSnapshots()
	jr.PerformBillSplitting()
}
