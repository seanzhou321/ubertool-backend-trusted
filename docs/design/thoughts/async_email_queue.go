package email

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// EmailJob represents an email to be sent asynchronously
type EmailJob struct {
	ID        string    `json:"id"`
	To        string    `json:"to"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	IsHTML    bool      `json:"is_html"`
	Retries   int       `json:"retries"`
	CreatedAt time.Time `json:"created_at"`
}

// EmailQueue handles async email sending with retries
type EmailQueue struct {
	emailService EmailSender
	jobs         chan EmailJob
	maxRetries   int
	workers      int
}

// EmailSender interface for any email backend
type EmailSender interface {
	SendEmail(msg EmailMessage) error
}

func NewEmailQueue(sender EmailSender, workers, queueSize, maxRetries int) *EmailQueue {
	return &EmailQueue{
		emailService: sender,
		jobs:         make(chan EmailJob, queueSize),
		maxRetries:   maxRetries,
		workers:      workers,
	}
}

// Start begins processing emails asynchronously
func (eq *EmailQueue) Start(ctx context.Context) {
	for i := 0; i < eq.workers; i++ {
		go eq.worker(ctx, i)
	}
}

func (eq *EmailQueue) worker(ctx context.Context, id int) {
	log.Printf("Email worker %d started", id)
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("Email worker %d stopping", id)
			return
		case job := <-eq.jobs:
			eq.processJob(job)
		}
	}
}

func (eq *EmailQueue) processJob(job EmailJob) {
	log.Printf("Processing email job %s to %s", job.ID, job.To)
	
	msg := EmailMessage{
		To:      []string{job.To},
		Subject: job.Subject,
		Body:    job.Body,
		IsHTML:  job.IsHTML,
	}
	
	err := eq.emailService.SendEmail(msg)
	if err != nil {
		log.Printf("Failed to send email %s: %v", job.ID, err)
		
		if job.Retries < eq.maxRetries {
			// Retry with exponential backoff
			job.Retries++
			backoff := time.Duration(job.Retries*job.Retries) * time.Second
			log.Printf("Retrying email %s in %v (attempt %d/%d)", job.ID, backoff, job.Retries, eq.maxRetries)
			
			time.AfterFunc(backoff, func() {
				eq.jobs <- job
			})
		} else {
			log.Printf("Email %s failed after %d retries", job.ID, eq.maxRetries)
			// Could save to dead letter queue here
		}
		return
	}
	
	log.Printf("Email %s sent successfully to %s", job.ID, job.To)
}

// Enqueue adds an email to the queue
func (eq *EmailQueue) Enqueue(to, subject, body string, isHTML bool) error {
	job := EmailJob{
		ID:        generateID(),
		To:        to,
		Subject:   subject,
		Body:      body,
		IsHTML:    isHTML,
		Retries:   0,
		CreatedAt: time.Now(),
	}
	
	select {
	case eq.jobs <- job:
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

func generateID() string {
	return fmt.Sprintf("email-%d", time.Now().UnixNano())
}

// Redis-backed persistent queue (optional enhancement)
type RedisEmailQueue struct {
	queue        *EmailQueue
	redisClient  interface{} // Use your Redis client
	queueKey     string
}

func (req *RedisEmailQueue) PersistJob(job EmailJob) error {
	// Marshal job to JSON and push to Redis list
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	
	// Redis LPUSH operation
	// req.redisClient.LPush(ctx, req.queueKey, data)
	_ = data // placeholder
	return nil
}

func (req *RedisEmailQueue) LoadJobs(ctx context.Context) error {
	// Redis RPOP to retrieve jobs and add to in-memory queue
	// This allows recovery after restart
	return nil
}

/*
// Usage example:

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create email sender (SMTP or SendGrid)
	sender := NewEmailService("smtp.gmail.com", "587", "admin@gmail.com", "password", "App")
	
	// Create queue with 3 workers, 100 job buffer, max 3 retries
	queue := NewEmailQueue(sender, 3, 100, 3)
	queue.Start(ctx)
	
	// Enqueue emails (non-blocking)
	err := queue.Enqueue(
		"user@yahoo.com",
		"Welcome to Tool Sharing",
		"Thanks for joining!",
		false,
	)
	if err != nil {
		log.Printf("Failed to enqueue: %v", err)
	}
	
	// Keep running
	select {}
}
*/