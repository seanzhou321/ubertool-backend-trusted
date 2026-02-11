package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"
	"ubertool-backend-trusted/internal/jobs"
	"ubertool-backend-trusted/internal/logger"
)

// Scheduler manages cron job scheduling
type Scheduler struct {
	cron *cron.Cron
	jobs *jobs.JobRunner
}

// NewScheduler creates a new scheduler with the provided job runner
func NewScheduler(jobRunner *jobs.JobRunner) *Scheduler {
	// Create cron with UTC timezone and seconds precision
	c := cron.New(
		cron.WithLocation(time.UTC),
		cron.WithSeconds(),
	)

	s := &Scheduler{
		cron: c,
		jobs: jobRunner,
	}

	s.registerJobs()
	return s
}

// registerJobs registers all scheduled jobs with the cron scheduler
func (s *Scheduler) registerJobs() {
	// Nightly jobs (2 AM UTC)
	// Mark overdue rentals
	_, err := s.cron.AddFunc("0 0 2 * * *", s.jobs.MarkOverdueRentals)
	if err != nil {
		logger.Error("Failed to register MarkOverdueRentals job", "error", err)
	}

	// Send overdue reminders (3 AM UTC)
	_, err = s.cron.AddFunc("0 0 3 * * *", s.jobs.SendOverdueReminders)
	if err != nil {
		logger.Error("Failed to register SendOverdueReminders job", "error", err)
	}

	// Send bill reminders (4 AM UTC)
	_, err = s.cron.AddFunc("0 0 4 * * *", s.jobs.SendBillReminders)
	if err != nil {
		logger.Error("Failed to register SendBillReminders job", "error", err)
	}

	// Check overdue bills daily (10th of each month at 5 AM UTC)
	_, err = s.cron.AddFunc("0 0 5 10 * *", s.jobs.CheckOverdueBills)
	if err != nil {
		logger.Error("Failed to register CheckOverdueBills job", "error", err)
	}

	// Monthly jobs - Run on last day of month
	// Resolve disputed bills (11 PM UTC on last day of month)
	_, err = s.cron.AddFunc("0 0 23 L * *", s.jobs.ResolveDisputedBills)
	if err != nil {
		logger.Error("Failed to register ResolveDisputedBills job", "error", err)
	}

	// Take balance snapshots (11:30 PM UTC on last day of month)
	_, err = s.cron.AddFunc("0 30 23 L * *", s.jobs.TakeBalanceSnapshots)
	if err != nil {
		logger.Error("Failed to register TakeBalanceSnapshots job", "error", err)
	}

	// Perform bill splitting (12:00 AM UTC on 1st of month)
	_, err = s.cron.AddFunc("0 0 0 1 * *", s.jobs.PerformBillSplitting)
	if err != nil {
		logger.Error("Failed to register PerformBillSplitting job", "error", err)
	}

	logger.Info("All cron jobs registered successfully")
}

// Start begins the cron scheduler
func (s *Scheduler) Start() {
	logger.Info("Starting cron scheduler...")
	s.cron.Start()
	logger.Info("Cron scheduler started successfully")
}

// Stop gracefully stops the cron scheduler
func (s *Scheduler) Stop() {
	logger.Info("Stopping cron scheduler...")
	ctx := s.cron.Stop()
	<-ctx.Done()
	logger.Info("Cron scheduler stopped")
}

// IsRunning returns true if the scheduler is running
func (s *Scheduler) IsRunning() bool {
	return len(s.cron.Entries()) > 0
}
