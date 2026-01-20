package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"

	"github.com/lib/pq"
)

type toolRepository struct {
	db *sql.DB
}

func NewToolRepository(db *sql.DB) repository.ToolRepository {
	return &toolRepository{db: db}
}

func (r *toolRepository) Create(ctx context.Context, t *domain.Tool) error {
	query := `INSERT INTO tools (owner_id, name, description, categories, price_per_day_cents, price_per_week_cents, price_per_month_cents, replacement_cost_cents, duration_unit, condition, metro, status, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id`
	return r.db.QueryRowContext(ctx, query, t.OwnerID, t.Name, t.Description, pq.Array(t.Categories), t.PricePerDayCents, t.PricePerWeekCents, t.PricePerMonthCents, t.ReplacementCostCents, "day", t.Condition, t.Metro, t.Status, time.Now()).Scan(&t.ID)
}

func (r *toolRepository) GetByID(ctx context.Context, id int32) (*domain.Tool, error) {
	t := &domain.Tool{}
	query := `SELECT id, owner_id, name, description, categories, price_per_day_cents, price_per_week_cents, price_per_month_cents, replacement_cost_cents, condition, metro, status, created_on, deleted_on FROM tools WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&t.ID, &t.OwnerID, &t.Name, &t.Description, pq.Array(&t.Categories), &t.PricePerDayCents, &t.PricePerWeekCents, &t.PricePerMonthCents, &t.ReplacementCostCents, &t.Condition, &t.Metro, &t.Status, &t.CreatedOn, &t.DeletedOn)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *toolRepository) Update(ctx context.Context, t *domain.Tool) error {
	query := `UPDATE tools SET name=$1, description=$2, categories=$3, price_per_day_cents=$4, price_per_week_cents=$5, price_per_month_cents=$6, replacement_cost_cents=$7, condition=$8, metro=$9, status=$10 WHERE id=$11`
	_, err := r.db.ExecContext(ctx, query, t.Name, t.Description, pq.Array(t.Categories), t.PricePerDayCents, t.PricePerWeekCents, t.PricePerMonthCents, t.ReplacementCostCents, t.Condition, t.Metro, t.Status, t.ID)
	return err
}

func (r *toolRepository) Delete(ctx context.Context, id int32) error {
	query := `UPDATE tools SET deleted_on = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *toolRepository) ListByOrg(ctx context.Context, orgID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	// Note: tools table doesn't have org_id, but the requirement says "List tools in an organization".
	// This usually means tools belonging to users who are members of the organization.
	// However, the schema shows tools are global but can be filtered by metro.
	// Looking at the PRD: "Initial Context: User selects a 'Current Org' (e.g., Church A) to start searching. ... Auto-Metro Filter: The search automatically filters for Tools in Church A's metro."
	// So we'll filter by metro of the organization.
	
	orgQuery := `SELECT metro FROM orgs WHERE id = $1`
	var metro string
	err := r.db.QueryRowContext(ctx, orgQuery, orgID).Scan(&metro)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `SELECT id, owner_id, name, description, categories, price_per_day_cents, price_per_week_cents, price_per_month_cents, replacement_cost_cents, condition, metro, status, created_on, deleted_on 
	          FROM tools WHERE metro = $1 AND deleted_on IS NULL LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, metro, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var count int32
	countQuery := `SELECT count(*) FROM tools WHERE metro = $1 AND deleted_on IS NULL`
	err = r.db.QueryRowContext(ctx, countQuery, metro).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	var tools []domain.Tool
	for rows.Next() {
		var t domain.Tool
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.Name, &t.Description, pq.Array(&t.Categories), &t.PricePerDayCents, &t.PricePerWeekCents, &t.PricePerMonthCents, &t.ReplacementCostCents, &t.Condition, &t.Metro, &t.Status, &t.CreatedOn, &t.DeletedOn); err != nil {
			return nil, 0, err
		}
		tools = append(tools, t)
	}
	return tools, count, nil
}

func (r *toolRepository) ListByOwner(ctx context.Context, ownerID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	offset := (page - 1) * pageSize
	query := `SELECT id, owner_id, name, description, categories, price_per_day_cents, price_per_week_cents, price_per_month_cents, replacement_cost_cents, condition, metro, status, created_on, deleted_on 
	          FROM tools WHERE owner_id = $1 AND deleted_on IS NULL LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, ownerID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var count int32
	countQuery := `SELECT count(*) FROM tools WHERE owner_id = $1 AND deleted_on IS NULL`
	err = r.db.QueryRowContext(ctx, countQuery, ownerID).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	var tools []domain.Tool
	for rows.Next() {
		var t domain.Tool
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.Name, &t.Description, pq.Array(&t.Categories), &t.PricePerDayCents, &t.PricePerWeekCents, &t.PricePerMonthCents, &t.ReplacementCostCents, &t.Condition, &t.Metro, &t.Status, &t.CreatedOn, &t.DeletedOn); err != nil {
			return nil, 0, err
		}
		tools = append(tools, t)
	}
	return tools, count, nil
}

func (r *toolRepository) Search(ctx context.Context, orgID int32, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error) {
	orgQuery := `SELECT metro FROM orgs WHERE id = $1`
	var metro string
	err := r.db.QueryRowContext(ctx, orgQuery, orgID).Scan(&metro)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	sql := `SELECT id, owner_id, name, description, categories, price_per_day_cents, price_per_week_cents, price_per_month_cents, replacement_cost_cents, condition, metro, status, created_on, deleted_on 
	          FROM tools WHERE metro = $1 AND deleted_on IS NULL`
	
	args := []interface{}{metro}
	argIdx := 2
	
	if query != "" {
		sql += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+query+"%")
		argIdx++
	}
	if len(categories) > 0 {
		sql += fmt.Sprintf(" AND categories && $%d", argIdx)
		args = append(args, pq.Array(categories))
		argIdx++
	}
	if maxPrice > 0 {
		sql += fmt.Sprintf(" AND price_per_day_cents <= $%d", argIdx)
		args = append(args, maxPrice)
		argIdx++
	}
	if condition != "" {
		sql += fmt.Sprintf(" AND condition = $%d", argIdx)
		args = append(args, condition)
		argIdx++
	}

	var count int32
	countSql := "SELECT count(*) FROM (" + sql + ") as sub"
	err = r.db.QueryRowContext(ctx, countSql, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	sql += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tools []domain.Tool
	for rows.Next() {
		var t domain.Tool
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.Name, &t.Description, pq.Array(&t.Categories), &t.PricePerDayCents, &t.PricePerWeekCents, &t.PricePerMonthCents, &t.ReplacementCostCents, &t.Condition, &t.Metro, &t.Status, &t.CreatedOn, &t.DeletedOn); err != nil {
			return nil, 0, err
		}
		tools = append(tools, t)
	}
	return tools, count, nil
}

func (r *toolRepository) AddImage(ctx context.Context, img *domain.ToolImage) error {
	query := `INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, width, height, is_primary, display_order, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`
	return r.db.QueryRowContext(ctx, query, img.ToolID, img.FileName, img.FilePath, img.ThumbnailPath, img.FileSize, img.MimeType, img.Width, img.Height, img.IsPrimary, img.DisplayOrder, time.Now()).Scan(&img.ID)
}

func (r *toolRepository) GetImages(ctx context.Context, toolID int32) ([]domain.ToolImage, error) {
	query := `SELECT id, tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, width, height, is_primary, display_order, created_on 
	          FROM tool_images WHERE tool_id = $1 AND deleted_on IS NULL ORDER BY display_order`
	rows, err := r.db.QueryContext(ctx, query, toolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []domain.ToolImage
	for rows.Next() {
		var img domain.ToolImage
		var createdOn time.Time
		if err := rows.Scan(&img.ID, &img.ToolID, &img.FileName, &img.FilePath, &img.ThumbnailPath, &img.FileSize, &img.MimeType, &img.Width, &img.Height, &img.IsPrimary, &img.DisplayOrder, &createdOn); err != nil {
			return nil, err
		}
		img.CreatedOn = createdOn.Format("2006-01-02")
		images = append(images, img)
	}
	return images, nil
}

func (r *toolRepository) DeleteImage(ctx context.Context, imageID int32) error {
	query := `UPDATE tool_images SET deleted_on = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), imageID)
	return err
}

func (r *toolRepository) SetPrimaryImage(ctx context.Context, toolID, imageID int32) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset all primaries for this tool
	_, err = tx.ExecContext(ctx, `UPDATE tool_images SET is_primary = false WHERE tool_id = $1`, toolID)
	if err != nil {
		return err
	}

	// Set new primary
	_, err = tx.ExecContext(ctx, `UPDATE tool_images SET is_primary = true WHERE id = $1`, imageID)
	if err != nil {
		return err
	}

	return tx.Commit()
}
