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

	query := `INSERT INTO notifications (user_id, org_id, title, message, attributes)
	          VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`
	logger.DatabaseCall("INSERT", "notifications", "userID", n.UserID, "orgID", n.OrgID)

	var createdAt time.Time
	err = r.db.QueryRowContext(ctx, query, n.UserID, n.OrgID, n.Title, n.Message, attrs).Scan(&n.ID, &createdAt)
	n.CreatedAt = &createdAt
	logger.DatabaseResult("INSERT", 1, err, "notificationID", n.ID)

	if err != nil {
		logger.ExitMethodWithError("notificationRepository.Create", err, "userID", n.UserID, "orgID", n.OrgID)
	} else {
		logger.ExitMethod("notificationRepository.Create", "notificationID", n.ID)
	}
	return err
}

func (r *notificationRepository) List(ctx context.Context, userID int32, limit, offset int32) ([]domain.Notification, int32, error) {
	query := `SELECT id, user_id, org_id, title, message, delivered_at, clicked_at, read_at, attributes, created_at
	          FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var count int32
	countQuery := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
	if err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&count); err != nil {
		return nil, 0, err
	}

	var notes []domain.Notification
	for rows.Next() {
		var n domain.Notification
		var attrs []byte
		var deliveredAt, clickedAt, readAt, createdAt sql.NullTime
		if err := rows.Scan(&n.ID, &n.UserID, &n.OrgID, &n.Title, &n.Message,
			&deliveredAt, &clickedAt, &readAt, &attrs, &createdAt); err != nil {
			return nil, 0, err
		}
		if deliveredAt.Valid {
			n.DeliveredAt = &deliveredAt.Time
		}
		if clickedAt.Valid {
			n.ClickedAt = &clickedAt.Time
		}
		if readAt.Valid {
			n.ReadAt = &readAt.Time
		}
		if createdAt.Valid {
			n.CreatedAt = &createdAt.Time
		}
		if len(attrs) > 0 {
			if err := json.Unmarshal(attrs, &n.Attributes); err != nil {
				return nil, 0, err
			}
		}
		notes = append(notes, n)
	}
	return notes, count, nil
}

func (r *notificationRepository) MarkAsRead(ctx context.Context, id int64, userID int32) error {
	query := `UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE id = $1 AND user_id = $2`
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

func (r *notificationRepository) MarkDelivered(ctx context.Context, id int64, userID int32, t time.Time) error {
	query := `UPDATE notifications SET delivered_at = COALESCE(delivered_at, $3) WHERE id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, id, userID, t)
	return err
}

func (r *notificationRepository) MarkClicked(ctx context.Context, id int64, userID int32, t time.Time) error {
	query := `UPDATE notifications SET clicked_at = COALESCE(clicked_at, $3) WHERE id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, id, userID, t)
	return err
}
