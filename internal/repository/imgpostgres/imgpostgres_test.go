package imgpostgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wb-go/wbf/dbpg"
)

func newRepoWithMock(t *testing.T) (PostgresRepo, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	pg := &dbpg.DB{Master: db}

	repo := PostgresRepo{DB: pg}

	return repo, mock
}

// CREATE - SUCCESS
func TestPostgresRepo_Create_OK(t *testing.T) {
	repo, mock := newRepoWithMock(t)

	ctime := time.Now()
	img := &model.Image{
		UID:       uuid.New(),
		Status:    model.StatusCreated,
		CreatedAt: &ctime,
	}

	mock.ExpectQuery(`INSERT INTO images`).
		WithArgs(
			img.UID,
			img.SourceKey,
			img.WatermarkKey,
			img.ResultKey,
			img.Operation,
			img.X,
			img.Y,
			img.Status,
			img.ErrMsg,
			img.CreatedAt,
			img.CreatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{}))

	err := repo.Create(context.Background(), img)
	require.NoError(t, err)
}

// GET - SUCCESS
func TestPostgresRepo_Get_OK(t *testing.T) {
	repo, mock := newRepoWithMock(t)

	id := uuid.New().String()

	rows := sqlmock.NewRows([]string{
		"image_uid", "source_key", "wm_key", "result_key",
		"operation", "x_axis", "y_axis",
		"status", "err_msg", "created_at", "updated_at",
	}).AddRow(
		id, "src", "", "",
		model.OpResize, 100, 100,
		model.StatusCreated, nil, time.Now(), time.Now(),
	)

	mock.ExpectQuery(`SELECT image_uid`).
		WithArgs(id).
		WillReturnRows(rows)

	img, err := repo.Get(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, img.UID.String())
}

// GET - NOT FOUND
func TestPostgresRepo_Get_NotFound(t *testing.T) {
	repo, mock := newRepoWithMock(t)

	mock.ExpectQuery(`SELECT image_uid`).
		WillReturnError(sql.ErrNoRows)

	_, err := repo.Get(context.Background(), uuid.New().String())
	require.ErrorIs(t, err, model.ErrImageNotFound)
}

// GETLIST - SUCCESS
func TestPostgresRepo_GetList_OK(t *testing.T) {
	repo, mock := newRepoWithMock(t)

	req := &model.ListRequest{
		Page:  1,
		Limit: 2,
		Sort:  "created_at",
		Order: "DESC",
	}

	rows := sqlmock.NewRows([]string{
		"image_uid", "operation", "x_axis", "y_axis",
		"status", "err_msg", "created_at", "updated_at",
	}).
		AddRow(uuid.New(), model.OpResize, 100, nil, model.StatusDone, nil, time.Now(), time.Now()).
		AddRow(uuid.New(), model.OpThumbNail, 50, 50, model.StatusCreated, nil, time.Now(), time.Now())

	mock.ExpectQuery(`SELECT image_uid, operation`).
		WithArgs(2, 0).
		WillReturnRows(rows)

	res, err := repo.GetList(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, res, 2)
}

// DELETE - SUCCESS - дописать аналогичные и для update с saveresult
func TestPostgresRepo_Delete_OK(t *testing.T) {
	repo, mock := newRepoWithMock(t)

	mock.ExpectExec(`DELETE FROM images`).
		WithArgs("id").
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected

	err := repo.Delete(context.Background(), "id")
	require.NoError(t, err)
}

// DELETE - NOT FOUND
func TestPostgresRepo_Delete_NotFound(t *testing.T) {
	repo, mock := newRepoWithMock(t)

	mock.ExpectExec(`DELETE FROM images`).
		WithArgs("id").
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	err := repo.Delete(context.Background(), "id")
	require.ErrorIs(t, err, model.ErrImageNotFound)
}

// DELETE - DBERROR
func TestPostgresRepo_Delete_DBError(t *testing.T) {
	repo, mock := newRepoWithMock(t)

	mock.ExpectExec(`DELETE FROM images`).
		WithArgs("id").
		WillReturnError(errors.New("db down"))

	err := repo.Delete(context.Background(), "id")
	require.Error(t, err)
}

// FETCHORPHANS - SUCCESS
func TestPostgresRepo_FetchOrphans_OK(t *testing.T) {
	repo, mock := newRepoWithMock(t)

	rows := sqlmock.NewRows([]string{"image_uid"}).
		AddRow("id1").
		AddRow("id2")

	mock.ExpectQuery(`SELECT image_uid`).
		WithArgs(model.StatusCreated, model.StatusInProgress, 2).
		WillReturnRows(rows)

	res, err := repo.FetchOrphans(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, []string{"id1", "id2"}, res)
}
