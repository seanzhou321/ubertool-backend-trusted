package postgres

import (
	"context"
	"database/sql"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type organizationRepository struct {
	db *sql.DB
}

func NewOrganizationRepository(db *sql.DB) repository.OrganizationRepository {
	return &organizationRepository{db: db}
}

func (r *organizationRepository) Create(ctx context.Context, o *domain.Organization) error {
	query := `INSERT INTO orgs (name, description, address, metro, admin_phone_number, admin_email, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	return r.db.QueryRowContext(ctx, query, o.Name, o.Description, o.Address, o.Metro, o.AdminPhoneNumber, o.AdminEmail, time.Now()).Scan(&o.ID)
}

func (r *organizationRepository) GetByID(ctx context.Context, id int32) (*domain.Organization, error) {
	o := &domain.Organization{}
	query := `SELECT id, name, description, address, metro, admin_phone_number, admin_email, created_on FROM orgs WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&o.ID, &o.Name, &o.Description, &o.Address, &o.Metro, &o.AdminPhoneNumber, &o.AdminEmail, &o.CreatedOn)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *organizationRepository) List(ctx context.Context) ([]domain.Organization, error) {
	query := `SELECT id, name, description, address, metro, admin_phone_number, admin_email, created_on FROM orgs`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []domain.Organization
	for rows.Next() {
		var o domain.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Description, &o.Address, &o.Metro, &o.AdminPhoneNumber, &o.AdminEmail, &o.CreatedOn); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	return orgs, nil
}

func (r *organizationRepository) Search(ctx context.Context, name, metro string) ([]domain.Organization, error) {
	query := `SELECT id, name, description, address, metro, admin_phone_number, admin_email, created_on FROM orgs 
	          WHERE name ILIKE $1 AND metro ILIKE $2`
	rows, err := r.db.QueryContext(ctx, query, "%"+name+"%", "%"+metro+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []domain.Organization
	for rows.Next() {
		var o domain.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Description, &o.Address, &o.Metro, &o.AdminPhoneNumber, &o.AdminEmail, &o.CreatedOn); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	return orgs, nil
}
