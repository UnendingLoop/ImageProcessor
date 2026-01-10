package imgpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/wb-go/wbf/dbpg"
)

type PostgresRepo struct {
	DB *dbpg.DB
}

func (p PostgresRepo) Create(ctx context.Context, n *model.Image) error {
	query := `INSERT INTO images (image_uid, source_key, wm_key, result_key, operation, x_axis, y_axis, status, err_msg, created_at, updated_at )
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	return p.DB.QueryRowContext(ctx, query, n.UID, n.SourceKey, n.WatermarkKey, n.ResultKey, n.Operation, n.X, n.Y, n.Status, n.ErrMsg, n.CreatedAt, n.CreatedAt).Err()
}

func (p PostgresRepo) Get(ctx context.Context, id string) (*model.Image, error) {
	query := `SELECT image_uid, source_key, wm_key, result_key, operation, status, err_msg, created_at, updated_at 
	FROM images 
	WHERE image_uid = $1`
	var image model.Image

	err := p.DB.QueryRowContext(ctx, query, id).Scan(&image.UID,
		&image.SourceKey,
		&image.WatermarkKey,
		&image.ResultKey,
		&image.Operation,
		&image.Status,
		&image.ErrMsg,
		&image.CreatedAt,
		&image.UpdatedAt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, model.ErrImageNotFound
		default:
			return nil, err // 500
		}
	}
	return &image, nil
}

func (p PostgresRepo) GetList(ctx context.Context, req *model.ListRequest) ([]model.Image, error) {
	query := fmt.Sprintf(`SELECT image_uid, operation, x_axis, y_axis, status, err_msg, created_at, updated_at 
	FROM images
	ORDER BY %s %s 
	LIMIT $1 
	OFFSET $2`, req.Sort, req.Order)

	offset := (req.Page - 1) * req.Limit

	rows, err := p.DB.QueryContext(ctx, query, req.Limit, offset)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error while closing *sql.Rows after scanning: %v", err)
		}
	}()

	images := make([]model.Image, 0, req.Limit)
	for rows.Next() {
		var image model.Image
		if err := rows.Scan(&image.UID,
			&image.Operation,
			&image.X,
			&image.Y,
			&image.Status,
			&image.ErrMsg,
			&image.CreatedAt,
			&image.UpdatedAt); err != nil {
			return nil, err
		}
		images = append(images, image)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return images, nil
}

func (p PostgresRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM images
	WHERE image_uid = $1`

	row := p.DB.QueryRowContext(ctx, query, id)
	if row.Err() != nil {
		switch {
		case errors.Is(row.Err(), sql.ErrNoRows):
			return model.ErrImageNotFound // 404
		default:
			return row.Err() // 500
		}
	}
	return nil
}

func (p PostgresRepo) UpdateStatus(ctx context.Context, id string, newStat model.Status) error {
	query := `UPDATE images SET status=$1, updated_at = now() WHERE id = $2`
	row := p.DB.QueryRowContext(ctx, query, newStat, id)

	if row.Err() != nil {
		switch {
		case errors.Is(row.Err(), sql.ErrNoRows):
			return model.ErrImageNotFound // 404
		default:
			return row.Err() // 500
		}
	}
	return nil
}

func (p PostgresRepo) SaveResult(ctx context.Context, input *model.Image) error {
	query := `UPDATE images SET status = $1, updated_at = $2, result_key = $3 WHERE id = $4`
	row := p.DB.QueryRowContext(ctx, query, input.Status, input.UpdatedAt, input.ResultKey, input.UID)

	if row.Err() != nil {
		switch {
		case errors.Is(row.Err(), sql.ErrNoRows):
			return model.ErrImageNotFound // 404
		default:
			return row.Err() // 500
		}
	}

	return nil
}

func (p PostgresRepo) FetchOrphans(ctx context.Context, limit int) ([]string, error) {
	query := `SELECT image_uid 
	FROM images 
	WHERE status IN ($1, $2) 
	AND updated_at < now() - interval '10 minutes'
	LIMIT $3`

	rows, err := p.DB.QueryContext(ctx, query, model.StatusCreated, model.StatusInProgress, limit)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error while closing *sql.Rows after scanning: %v", err)
		}
	}()

	orphans := make([]string, 0, limit)
	for rows.Next() {
		uid := ""
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		orphans = append(orphans, uid)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return orphans, nil
}
