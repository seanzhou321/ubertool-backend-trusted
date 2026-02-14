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
	cfg := s.jobs.Config().Scheduler

	// Nightly jobs
	// Mark overdue rentals
	_, err := s.cron.AddFunc(cfg.MarkOverdueRentals, s.jobs.MarkOverdueRentals)
	if err != nil {
		logger.Error("Failed to register MarkOverdueRentals job", "error", err)
	}

	// Send overdue reminders
	_, err = s.cron.AddFunc(cfg.SendOverdueReminders, s.jobs.SendOverdueReminders)
	if err != nil {
		logger.Error("Failed to register SendOverdueReminders job", "error", err)
	}

	// Send bill reminders
	_, err = s.cron.AddFunc(cfg.SendBillReminders, s.jobs.SendBillReminders)
	if err != nil {
		logger.Error("Failed to register SendBillReminders job", "error", err)
	}

	// Check overdue bills daily
	_, err = s.cron.AddFunc(cfg.CheckOverdueBills, s.jobs.CheckOverdueBills)
	if err != nil {
		logger.Error("Failed to register CheckOverdueBills job", "error", err)
	}

	// Monthly jobs
	// Resolve disputed bills
	_, err = s.cron.AddFunc(cfg.ResolveDisputedBills, s.jobs.ResolveDisputedBills)
	if err != nil {
		logger.Error("Failed to register ResolveDisputedBills job", "error", err)
	}

	// Take balance snapshots
	_, err = s.cron.AddFunc(cfg.TakeBalanceSnapshots, s.jobs.TakeBalanceSnapshots)
	if err != nil {
		logger.Error("Failed to register TakeBalanceSnapshots job", "error", err)
	}

	// Perform bill splitting
	_, err = s.cron.AddFunc(cfg.PerformBillSplitting, s.jobs.PerformBillSplitting)
	if err != nil {
		logger.Error("Failed to register PerformBillSplitting job", "error", err)
	}

	// Send bill splitting notices
	_, err = s.cron.AddFunc(cfg.SendBillNotices, s.jobs.SendBillSplittingNotices)
	if err != nil {
		logger.Error("Failed to register SendBillSplittingNotices job", "error", err)
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
