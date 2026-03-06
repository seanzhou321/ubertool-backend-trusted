package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"firebase.google.com/go/v4/messaging"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository"
)

const (
	fcmSendTimeout = 10 * time.Second

	// fcmWorkerCount is the number of long-running batch-sender goroutines.
	// Workers are shared across all SendToUser calls so the goroutine count
	// stays bounded regardless of notification volume.
	fcmWorkerCount = 5

	// fcmJobQueueSize is the capacity of the buffered job channel. Jobs are
	// dropped (with a warning) only when this many are already waiting, which
	// protects against unbounded memory growth under extreme load.
	fcmJobQueueSize = 256

	// fcmBatchSize is the maximum number of messages per SendEach call.
	// Firebase enforces a hard limit of 500 per batch.
	fcmBatchSize = 500

	// fcmBatchWindow is how long a worker waits to accumulate a full batch
	// before flushing whatever it has collected so far.
	fcmBatchWindow = 100 * time.Millisecond
)

// fcmRetryDelays defines the back-off intervals between successive send attempts.
// The first element is the delay before the 1st retry, and so on.
var fcmRetryDelays = []time.Duration{
	1 * time.Minute,
	2 * time.Minute,
	4 * time.Minute,
}

// FCMSender abstracts single-message sends. Used in test mode so that existing
// unit-test mocks (which only implement Send) do not need to change.
type FCMSender interface {
	Send(ctx context.Context, msg *messaging.Message) (string, error)
}

// FCMBatchSender abstracts batch sends. Used by the production worker pool.
// *messaging.Client satisfies both FCMSender and FCMBatchSender.
type FCMBatchSender interface {
	SendEach(ctx context.Context, msgs []*messaging.Message) (*messaging.BatchResponse, error)
}

// fcmJob is a single unit of work queued for delivery.
type fcmJob struct {
	token   domain.FcmToken
	msg     *messaging.Message
	attempt int // 0 = first attempt; incremented on each transient-retry enqueue
}

type pushNotificationService struct {
	// fcmClient is the single-message sender used in test mode.
	fcmClient FCMSender
	// fcmBatchClient is used by the production worker pool's SendEach path.
	// nil in test mode.
	fcmBatchClient   FCMBatchSender
	fcmRepo          repository.FcmTokenRepository
	retryDelays      []time.Duration
	isUnregisteredFn func(error) bool

	// jobs is the bounded work queue; nil in test mode (runs synchronously).
	jobs         chan fcmJob
	jobsMu       sync.Mutex // guards jobsClosed + close(jobs)
	jobsClosed   bool
	workerCtx    context.Context    // cancelled on Shutdown to interrupt sleeping retries
	workerCancel context.CancelFunc // nil in test mode
	wg           sync.WaitGroup     // tracks worker goroutines
}

// NewPushNotificationService creates a PushNotificationService backed by Firebase
// Messaging. When fcmClient is nil (e.g. dev without credentials) all sends are
// no-ops. A fixed-size worker pool is started immediately and drained by Shutdown.
func NewPushNotificationService(fcmClient *messaging.Client, fcmRepo repository.FcmTokenRepository) PushNotificationService {
	// Assign to interfaces only when non-nil; a typed nil stored in an
	// interface is != nil and would bypass the nil-guard inside SendToUser.
	var sender FCMSender
	var batchSender FCMBatchSender
	if fcmClient != nil {
		sender = fcmClient
		batchSender = fcmClient
	}
	svc := newPushSvc(sender, fcmRepo, fcmRetryDelays)
	svc.fcmBatchClient = batchSender

	ctx, cancel := context.WithCancel(context.Background())
	svc.workerCtx = ctx
	svc.workerCancel = cancel
	svc.jobs = make(chan fcmJob, fcmJobQueueSize)

	for i := 0; i < fcmWorkerCount; i++ {
		svc.wg.Add(1)
		go svc.runWorker()
	}
	return svc
}

// NewPushNotificationServiceForTest creates a PushNotificationService for unit
// tests. No worker pool or batch client is created; SendToUser runs sendOne
// synchronously on the caller goroutine so assertions are race-free. Existing
// MockFCMSender mocks (which only implement Send) work without modification.
func NewPushNotificationServiceForTest(sender FCMSender, fcmRepo repository.FcmTokenRepository, retryDelays []time.Duration, isUnregisteredFn func(error) bool) PushNotificationService {
	svc := newPushSvc(sender, fcmRepo, retryDelays)
	if isUnregisteredFn != nil {
		svc.isUnregisteredFn = isUnregisteredFn
	}
	svc.workerCtx = context.Background()
	return svc
}

func newPushSvc(sender FCMSender, fcmRepo repository.FcmTokenRepository, delays []time.Duration) *pushNotificationService {
	return &pushNotificationService{
		fcmClient:        sender,
		fcmRepo:          fcmRepo,
		retryDelays:      delays,
		isUnregisteredFn: messaging.IsUnregistered,
	}
}

// ---------------------------------------------------------------------------
// Worker pool
// ---------------------------------------------------------------------------

// runWorker continuously collects micro-batches from the job queue and sends
// them via SendEach. Exits when the jobs channel is closed and drained.
func (s *pushNotificationService) runWorker() {
	defer s.wg.Done()
	for {
		batch := s.collectBatch()
		if batch == nil {
			return // channel closed and empty; worker exits cleanly
		}
		s.sendBatch(s.workerCtx, batch)
	}
}

// collectBatch blocks until at least one job is available, then accumulates up
// to fcmBatchSize additional jobs within fcmBatchWindow before returning. Returns
// nil when the jobs channel has been closed and is empty.
func (s *pushNotificationService) collectBatch() []fcmJob {
	// Block until the first job arrives or the channel is closed.
	job, ok := <-s.jobs
	if !ok {
		return nil
	}
	batch := make([]fcmJob, 1, fcmBatchSize)
	batch[0] = job

	timer := time.NewTimer(fcmBatchWindow)
	defer timer.Stop()

	for len(batch) < fcmBatchSize {
		select {
		case job, ok = <-s.jobs:
			if !ok {
				return batch // channel closed; flush what we have
			}
			batch = append(batch, job)
		case <-timer.C:
			return batch // window elapsed; flush the partial batch
		}
	}
	return batch
}

// sendBatch calls SendEach for all messages in the batch and dispatches each
// per-message result: logs successes, marks obsolete tokens, and schedules
// back-off retries for transient failures.
func (s *pushNotificationService) sendBatch(ctx context.Context, batch []fcmJob) {
	msgs := make([]*messaging.Message, len(batch))
	for i, job := range batch {
		msgs[i] = job.msg
	}

	sendCtx, cancel := context.WithTimeout(context.Background(), fcmSendTimeout)
	resp, err := s.fcmBatchClient.SendEach(sendCtx, msgs)
	cancel()

	if err != nil {
		// Whole-batch failure (network / auth error); treat all as transient.
		logger.Warn("FCM SendEach failed entirely", "batchSize", len(batch), "error", err)
		for _, job := range batch {
			s.scheduleRetry(ctx, job, err)
		}
		return
	}

	logger.Debug("FCM SendEach complete",
		"batchSize", len(batch),
		"successCount", resp.SuccessCount,
		"failureCount", resp.FailureCount)

	for i, sr := range resp.Responses {
		job := batch[i]
		if sr.Success {
			logger.Info("FCM push sent successfully",
				"userID", job.token.UserID,
				"attempt", job.attempt+1,
				"tokenPrefix", tokenPrefix(job.token.Token),
				"messageID", sr.MessageID)
			continue
		}
		sendErr := sr.Error

		// Permanent: unregistered token.
		if s.isUnregisteredFn(sendErr) {
			logger.Info("FCM token unregistered, marking obsolete",
				"token_prefix", tokenPrefix(job.token.Token))
			if obsErr := s.fcmRepo.MarkObsolete(context.Background(), job.token.Token); obsErr != nil {
				logger.Error("Failed to mark FCM token obsolete", "error", obsErr)
			}
			continue
		}

		// Permanent: syntactically invalid token.
		if messaging.IsInvalidArgument(sendErr) {
			logger.Info("FCM token invalid argument, marking obsolete",
				"token_prefix", tokenPrefix(job.token.Token), "error", sendErr)
			if obsErr := s.fcmRepo.MarkObsolete(context.Background(), job.token.Token); obsErr != nil {
				logger.Error("Failed to mark FCM token obsolete", "error", obsErr)
			}
			continue
		}

		// Transient failure: schedule a back-off retry.
		s.scheduleRetry(ctx, job, sendErr)
	}
}

// scheduleRetry re-enqueues job after the appropriate back-off delay if retries
// remain. A lightweight goroutine handles only the sleep — no I/O inside, so
// the count is bounded by fcmJobQueueSize at any point in time.
func (s *pushNotificationService) scheduleRetry(ctx context.Context, job fcmJob, lastErr error) {
	if job.attempt >= len(s.retryDelays) {
		logger.Error("FCM send failed after all retries",
			"userID", job.token.UserID, "totalAttempts", job.attempt+1, "error", lastErr)
		return
	}
	delay := s.retryDelays[job.attempt]
	logger.Warn("FCM send attempt failed, scheduling retry",
		"userID", job.token.UserID, "attempt", job.attempt+1, "nextDelay", delay, "error", lastErr)

	retryJob := fcmJob{token: job.token, msg: job.msg, attempt: job.attempt + 1}
	go func() {
		select {
		case <-time.After(delay):
			if !s.tryEnqueue(retryJob) {
				logger.Warn("FCM retry dropped: queue closed or full",
					"userID", retryJob.token.UserID, "attempt", retryJob.attempt)
			}
		case <-ctx.Done():
			logger.Warn("FCM retry cancelled during back-off",
				"userID", retryJob.token.UserID, "attempt", retryJob.attempt)
		}
	}()
}

// tryEnqueue safely adds a job to the queue. The mutex ensures that this send
// and closeJobQueue never race. Returns false if the queue is closed or full.
func (s *pushNotificationService) tryEnqueue(job fcmJob) bool {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	if s.jobsClosed {
		return false
	}
	select {
	case s.jobs <- job:
		return true
	default:
		logger.Warn("FCM job queue full, dropping push notification",
			"userID", job.token.UserID, "queueCapacity", fcmJobQueueSize)
		return false
	}
}

// closeJobQueue closes the jobs channel exactly once (idempotent).
func (s *pushNotificationService) closeJobQueue() {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	if !s.jobsClosed {
		s.jobsClosed = true
		close(s.jobs)
	}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// SendToUser looks up all ACTIVE FCM tokens for userID and enqueues a push job
// for each device. The call never blocks: if the worker-pool queue is full the
// job is dropped with a warning rather than stalling the RPC handler. The
// notificationID is embedded in the data payload so the client can call
// ReportMessageEvent to report delivery/click events back to the server.
func (s *pushNotificationService) SendToUser(ctx context.Context, userID int32, title, body string, notificationID int64, data map[string]string) error {
	if s.fcmClient == nil {
		logger.Debug("FCM client not configured, skipping push", "userID", userID, "notificationID", notificationID)
		return nil
	}

	tokens, err := s.fcmRepo.GetActiveByUserID(ctx, userID)
	if err != nil {
		logger.Error("Failed to get FCM tokens for user", "userID", userID, "error", err)
		return err
	}
	if len(tokens) == 0 {
		logger.Debug("No active FCM tokens for user, skipping push", "userID", userID, "notificationID", notificationID)
		return nil
	}
	logger.Debug("Enqueueing push notification", "userID", userID, "tokenCount", len(tokens), "notificationID", notificationID)

	fcmPriority := fcmPriorityForChannel(data["channel_id"])
	for _, t := range tokens {
		payload := buildPayload(data, notificationID, title, body)
		msg := &messaging.Message{
			Token: t.Token,
			Data:  payload,
			Android: &messaging.AndroidConfig{
				// "high" wakes the device from Doze mode for time-sensitive events.
				Priority: fcmPriority,
			},
		}
		job := fcmJob{token: t, msg: msg, attempt: 0}
		if s.jobs != nil {
			if !s.tryEnqueue(job) {
				logger.Warn("FCM job dropped: queue closed or full",
					"userID", userID, "notificationID", notificationID)
			}
		} else {
			// Test mode: run synchronously so test assertions are race-free.
			s.sendOne(s.workerCtx, job)
		}
	}
	return nil
}

// Shutdown cancels the worker context (waking sleeping retry goroutines), closes
// the job queue (so workers drain remaining items then exit), and waits for all
// workers to finish. Returns ctx.Err() if the drain deadline is exceeded.
func (s *pushNotificationService) Shutdown(ctx context.Context) error {
	if s.workerCancel != nil {
		s.workerCancel()
	}
	if s.jobs != nil {
		s.closeJobQueue()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ---------------------------------------------------------------------------
// Test-mode synchronous send path
// ---------------------------------------------------------------------------

// sendOne is the synchronous send path used exclusively in test mode (jobs == nil).
// It retries on transient failures using context-interruptible back-off sleeps.
func (s *pushNotificationService) sendOne(ctx context.Context, job fcmJob) {
	t, msg := job.token, job.msg
	var lastErr error
	for attempt := 0; attempt <= len(s.retryDelays); attempt++ {
		if attempt > 0 {
			delay := s.retryDelays[attempt-1]
			logger.Debug("Retrying FCM send", "userID", t.UserID, "attempt", attempt, "delay", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				logger.Warn("FCM send cancelled during retry back-off",
					"userID", t.UserID, "attempt", attempt, "error", ctx.Err())
				return
			}
		}

		sendCtx, cancel := context.WithTimeout(context.Background(), fcmSendTimeout)
		var msgID string
		msgID, lastErr = s.fcmClient.Send(sendCtx, msg)
		cancel()

		if lastErr == nil {
			logger.Info("FCM push sent successfully",
				"userID", t.UserID, "attempt", attempt+1,
				"tokenPrefix", tokenPrefix(t.Token), "messageID", msgID)
			return
		}

		if s.isUnregisteredFn(lastErr) {
			logger.Info("FCM token unregistered, marking obsolete", "token_prefix", tokenPrefix(t.Token))
			if obsErr := s.fcmRepo.MarkObsolete(context.Background(), t.Token); obsErr != nil {
				logger.Error("Failed to mark FCM token obsolete", "error", obsErr)
			}
			return
		}

		if messaging.IsInvalidArgument(lastErr) {
			logger.Info("FCM token invalid argument, marking obsolete",
				"token_prefix", tokenPrefix(t.Token), "error", lastErr)
			if obsErr := s.fcmRepo.MarkObsolete(context.Background(), t.Token); obsErr != nil {
				logger.Error("Failed to mark FCM token obsolete", "error", obsErr)
			}
			return
		}

		logger.Warn("FCM send attempt failed", "userID", t.UserID, "attempt", attempt+1, "error", lastErr)
	}

	logger.Error("FCM send failed after all retries",
		"userID", t.UserID, "totalAttempts", len(s.retryDelays)+1, "error", lastErr)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fcmPriorityForChannel returns the FCM Android delivery priority for a channel.
//
// "high"   – wakes the device immediately from Doze mode (Doze bypass).
// "normal" – delivered when the device is next active; does not wake from Doze.
//
// rental_request_messages, billsplitting_messages, and dispute_messages are all
// time-sensitive and require "high" to produce heads-up banners and wake the device.
// admin_messages and app_messages use "normal" to avoid unnecessary battery drain.
func fcmPriorityForChannel(channelID string) string {
	switch domain.NotificationChannel(channelID) {
	case domain.ChannelRentalRequest, domain.ChannelBillSplitting, domain.ChannelDispute:
		return "high"
	default:
		return "normal"
	}
}

func buildPayload(extra map[string]string, notificationID int64, title, message string) map[string]string {
	payload := make(map[string]string, len(extra)+3)
	for k, v := range extra {
		payload[k] = v
	}
	payload["notification_id"] = fmt.Sprintf("%d", notificationID)
	payload["title"] = title
	payload["body"] = message
	return payload
}

// tokenPrefix returns the first 8 chars of a token for safe log output.
func tokenPrefix(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "..."
}
