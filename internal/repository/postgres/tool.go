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
	query := `SELECT id, owner_id, name, COALESCE(description, ''), categories, price_per_day_cents, COALESCE(price_per_week_cents, 0), COALESCE(price_per_month_cents, 0), COALESCE(replacement_cost_cents, 0), condition, metro, status, created_on, deleted_on FROM tools WHERE id = $1`
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
	query := `SELECT id, owner_id, name, COALESCE(description, ''), categories, price_per_day_cents, COALESCE(price_per_week_cents, 0), COALESCE(price_per_month_cents, 0), COALESCE(replacement_cost_cents, 0), condition, metro, status, created_on, deleted_on 
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
	query := `SELECT id, owner_id, name, COALESCE(description, ''), categories, price_per_day_cents, COALESCE(price_per_week_cents, 0), COALESCE(price_per_month_cents, 0), COALESCE(replacement_cost_cents, 0), condition, metro, status, created_on, deleted_on 
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
	sql := `SELECT id, owner_id, name, COALESCE(description, ''), categories, price_per_day_cents, COALESCE(price_per_week_cents, 0), COALESCE(price_per_month_cents, 0), COALESCE(replacement_cost_cents, 0), condition, metro, status, created_on, deleted_on 
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

// CreateImage creates a new image record (can be pending or confirmed)
func (r *toolRepository) CreateImage(ctx context.Context, img *domain.ToolImage) error {
	query := `INSERT INTO tool_images (tool_id, user_id, file_name, file_path, thumbnail_path, 
	          file_size, mime_type, is_primary, display_order, status, expires_at, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`
	return r.db.QueryRowContext(ctx, query, img.ToolID, img.UserID, img.FileName,
		img.FilePath, img.ThumbnailPath, img.FileSize, img.MimeType,
		img.IsPrimary, img.DisplayOrder, img.Status, img.ExpiresAt, time.Now()).Scan(&img.ID)
}

// GetImageByID retrieves a single image by ID
func (r *toolRepository) GetImageByID(ctx context.Context, imageID int32) (*domain.ToolImage, error) {
	query := `SELECT id, tool_id, user_id, file_name, file_path, thumbnail_path, file_size, 
	          mime_type, is_primary, display_order, status, expires_at, created_on, confirmed_on, deleted_on
	          FROM tool_images WHERE id = $1`

	img := &domain.ToolImage{}
	err := r.db.QueryRowContext(ctx, query, imageID).Scan(
		&img.ID, &img.ToolID, &img.UserID, &img.FileName, &img.FilePath,
		&img.ThumbnailPath, &img.FileSize, &img.MimeType,
		&img.IsPrimary, &img.DisplayOrder, &img.Status, &img.ExpiresAt, &img.CreatedOn,
		&img.ConfirmedOn, &img.DeletedOn)

	if err != nil {
		return nil, err
	}
	return img, nil
}

// GetImages retrieves all confirmed images for a tool
func (r *toolRepository) GetImages(ctx context.Context, toolID int32) ([]domain.ToolImage, error) {
	query := `SELECT id, tool_id, user_id, file_name, file_path, thumbnail_path, file_size, 
	          mime_type, is_primary, display_order, status, created_on, confirmed_on
	          FROM tool_images 
	          WHERE tool_id = $1 AND status = 'CONFIRMED' AND deleted_on IS NULL 
	          ORDER BY is_primary DESC, display_order ASC, created_on ASC`

	rows, err := r.db.QueryContext(ctx, query, toolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []domain.ToolImage
	for rows.Next() {
		var img domain.ToolImage
		if err := rows.Scan(&img.ID, &img.ToolID, &img.UserID, &img.FileName,
			&img.FilePath, &img.ThumbnailPath, &img.FileSize, &img.MimeType,
			&img.IsPrimary, &img.DisplayOrder, &img.Status, &img.CreatedOn, &img.ConfirmedOn); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

// GetPendingImagesByUser retrieves pending images for a user
func (r *toolRepository) GetPendingImagesByUser(ctx context.Context, userID int32) ([]domain.ToolImage, error) {
	query := `SELECT id, tool_id, user_id, file_name, file_path, thumbnail_path, file_size, 
	          mime_type, is_primary, display_order, status, expires_at, created_on
	          FROM tool_images 
	          WHERE user_id = $1 AND status = 'PENDING' AND deleted_on IS NULL
	          ORDER BY created_on DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []domain.ToolImage
	for rows.Next() {
		var img domain.ToolImage
		if err := rows.Scan(&img.ID, &img.ToolID, &img.UserID, &img.FileName,
			&img.FilePath, &img.ThumbnailPath, &img.FileSize, &img.MimeType,
			&img.IsPrimary, &img.DisplayOrder, &img.Status, &img.ExpiresAt, &img.CreatedOn); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

// UpdateImage updates an existing image record
func (r *toolRepository) UpdateImage(ctx context.Context, img *domain.ToolImage) error {
	query := `UPDATE tool_images 
	          SET tool_id = $2, file_path = $3, thumbnail_path = $4, file_size = $5, 
	              is_primary = $6, display_order = $7, status = $8, confirmed_on = $9
	          WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, img.ID, img.ToolID, img.FilePath, img.ThumbnailPath,
		img.FileSize, img.IsPrimary, img.DisplayOrder, img.Status, img.ConfirmedOn)
	return err
}

// ConfirmImage transitions a pending image to confirmed status
func (r *toolRepository) ConfirmImage(ctx context.Context, imageID int32, toolID int32) error {
	query := `UPDATE tool_images 
	          SET status = 'CONFIRMED', tool_id = $2, confirmed_on = $3 
	          WHERE id = $1 AND status = 'PENDING'`

	result, err := r.db.ExecContext(ctx, query, imageID, toolID, time.Now())
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("image not found or already confirmed")
	}

	return nil
}

// DeleteImage soft deletes an image
func (r *toolRepository) DeleteImage(ctx context.Context, imageID int32) error {
	query := `UPDATE tool_images SET status = 'DELETED', deleted_on = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), imageID)
	return err
}

// SetPrimaryImage sets a specific image as primary for a tool
func (r *toolRepository) SetPrimaryImage(ctx context.Context, toolID int32, imageID int32) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset all primaries for this tool
	_, err = tx.ExecContext(ctx, `UPDATE tool_images SET is_primary = false WHERE tool_id = $1 AND status = 'CONFIRMED'`, toolID)
	if err != nil {
		return err
	}

	// Set new primary
	result, err := tx.ExecContext(ctx, `UPDATE tool_images SET is_primary = true WHERE id = $1 AND tool_id = $2 AND status = 'CONFIRMED'`, imageID, toolID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("image not found or not confirmed")
	}

	return tx.Commit()
}

// DeleteExpiredPendingImages removes expired pending images
func (r *toolRepository) DeleteExpiredPendingImages(ctx context.Context) error {
	query := `UPDATE tool_images 
	          SET status = 'DELETED', deleted_on = $1 
	          WHERE status = 'PENDING' AND expires_at < $1`
	_, err := r.db.ExecContext(ctx, query, time.Now())
	return err
}
