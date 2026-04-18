package postgres

import (
	"context"
	"database/sql"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type legalConsentRepository struct {
	db *sql.DB
}

func NewLegalConsentRepository(db *sql.DB) repository.LegalConsentRepository {
	return &legalConsentRepository{db: db}
}

func (r *legalConsentRepository) Record(ctx context.Context, userID int32, docNames []string, version string) error {
	for _, doc := range docNames {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO user_legal_consents (user_id, doc_name, version)
			 VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING`,
			userID, doc, version,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *legalConsentRepository) ListByUser(ctx context.Context, userID int32) ([]domain.LegalConsent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT user_id, doc_name, version, consented_at
		 FROM user_legal_consents
		 WHERE user_id = $1
		 ORDER BY doc_name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var consents []domain.LegalConsent
	for rows.Next() {
		var c domain.LegalConsent
		if err := rows.Scan(&c.UserID, &c.DocName, &c.Version, &c.ConsentedAt); err != nil {
			return nil, err
		}
		consents = append(consents, c)
	}
	return consents, rows.Err()
}
