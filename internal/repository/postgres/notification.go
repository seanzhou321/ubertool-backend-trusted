package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository"
)

type notificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) repository.NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	logger.EnterMethod("notificationRepository.Create", "userID", n.UserID, "orgID", n.OrgID, "title", n.Title)

	attrs, err := json.Marshal(n.Attributes)
	if err != nil {
		logger.ExitMethodWithError("notificationRepository.Create", err, "reason", "failed to marshal attributes")
		return err
	}
	logger.Debug("Notification attributes marshaled", "attributesJSON", string(attrs))

	query := `INSERT INTO notifications (user_id, org_id, title, message, is_read, attributes, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	logger.DatabaseCall("INSERT", "notifications", "userID", n.UserID, "orgID", n.OrgID)

	now := time.Now().Format("2006-01-02")
	err = r.db.QueryRowContext(ctx, query, n.UserID, n.OrgID, n.Title, n.Message, n.IsRead, attrs, now).Scan(&n.ID)
	logger.DatabaseResult("INSERT", 1, err, "notificationID", n.ID)

	if err != nil {
		logger.ExitMethodWithError("notificationRepository.Create", err, "userID", n.UserID, "orgID", n.OrgID)
	} else {
		logger.ExitMethod("notificationRepository.Create", "notificationID", n.ID)
	}
	return err
}

func (r *notificationRepository) List(ctx context.Context, userID int32, limit, offset int32) ([]domain.Notification, int32, error) {
	query := `SELECT id, user_id, org_id, title, message, is_read, attributes, created_on 
	          FROM notifications WHERE user_id = $1 ORDER BY created_on DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var count int32
	countQuery := `SELECT count(*) FROM notifications WHERE user_id = $1`
	err = r.db.QueryRowContext(ctx, countQuery, userID).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	var notes []domain.Notification
	for rows.Next() {
		var n domain.Notification
		var attrs []byte
		var createdOn time.Time
		if err := rows.Scan(&n.ID, &n.UserID, &n.OrgID, &n.Title, &n.Message, &n.IsRead, &attrs, &createdOn); err != nil {
			return nil, 0, err
		}
		n.CreatedOn = createdOn.Format("2006-01-02")
		if len(attrs) > 0 {
			if err := json.Unmarshal(attrs, &n.Attributes); err != nil {
				return nil, 0, err
			}
		}
		notes = append(notes, n)
	}
	return notes, count, nil
}

func (r *notificationRepository) MarkAsRead(ctx context.Context, id, userID int32) error {
	query := `UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("notification not found or access denied")
	}
	return nil
}
