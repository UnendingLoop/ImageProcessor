package service

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"mime/multipart"
	"testing"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wb-go/wbf/retry"
)

// CREATE - SUCCESS
func TestImageService_Create_OK(t *testing.T) {
	ctx := context.Background()

	repo := &mockRepo{
		createFn: func(ctx context.Context, img *model.Image) error {
			require.NotEmpty(t, img.UID)
			require.Equal(t, model.StatusCreated, img.Status)
			return nil
		},
	}

	storage := &mockStorage{
		putFn: func(ctx context.Context, key string, size int64, ct string, r io.Reader) error {
			return nil
		},
	}

	pub := &mockPublisher{
		sendFn: func(ctx context.Context, s retry.Strategy, key []byte, v []byte) error {
			require.NotEmpty(t, key)
			return nil
		},
	}

	svc := ImageService{
		repo:         repo,
		storage:      storage,
		publisher:    pub,
		srcKeyPrefix: "src/",
	}

	x := 100
	imgData := &model.ImageCreateData{
		Operation:       string(model.OpResize),
		OrigImg:         newFakeFile("img"),
		OrigImgSize:     10,
		OrigContentType: model.JPEG,
		X:               &x,
	}

	img, err := svc.Create(ctx, imgData)
	require.NoError(t, err)
	require.NotNil(t, img)
}

// CREATE - VALIDATION FAIL
func TestImageService_Create_InvalidInput(t *testing.T) {
	svc := ImageService{}

	_, err := svc.Create(context.Background(), &model.ImageCreateData{})
	require.ErrorIs(t, err, model.ErrIncorrectOp)
}

// CREATE - STORAGE PUT FAIL
func TestImageService_Create_StorageError(t *testing.T) {
	repo := &mockRepo{}
	storage := &mockStorage{
		putFn: func(ctx context.Context, key string, size int64, ct string, r io.Reader) error {
			return errors.New("storage is down")
		},
	}

	svc := ImageService{
		repo:         repo,
		storage:      storage,
		srcKeyPrefix: "src/",
	}

	_, err := svc.Create(context.Background(), validCreateData())
	require.ErrorIs(t, err, model.ErrCommon500)
}

// GETLIST - SUCCESS
func TestImageService_GetList_OK(t *testing.T) {
	repo := &mockRepo{
		getListFn: func(ctx context.Context, req *model.ListRequest) ([]model.Image, error) {
			require.Equal(t, 1, req.Page)
			return []model.Image{{UID: uuid.New()}}, nil
		},
	}

	svc := ImageService{repo: repo}

	res, err := svc.GetList(context.Background(), &model.ListRequest{})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

// GET - SUCCESS
func TestImageService_Get_OK(t *testing.T) {
	id := uuid.New().String()

	repo := &mockRepo{
		getFn: func(ctx context.Context, uid string) (*model.Image, error) {
			return &model.Image{UID: uuid.MustParse(uid)}, nil
		},
	}

	svc := ImageService{repo: repo}

	img, err := svc.Get(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, img.UID.String())
}

// GET - FAIL
func TestImageService_Get_InvalidID(t *testing.T) {
	svc := ImageService{}
	_, err := svc.Get(context.Background(), "bad-id")
	require.ErrorIs(t, err, model.ErrIncorrectID)
}

// LOADRESULT - FAIL
func TestImageService_LoadResult_NotReady(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, id string) (*model.Image, error) {
			return &model.Image{Status: model.StatusCreated}, nil
		},
	}

	svc := ImageService{repo: repo}

	_, _, err := svc.LoadResult(context.Background(), uuid.New().String())
	require.ErrorIs(t, err, model.ErrResultNotReady)
}

// DELETE - FAIL - NOT FOUND
func TestImageService_Delete_NotFound(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, id string) (*model.Image, error) {
			return nil, sql.ErrNoRows
		},
	}

	svc := ImageService{repo: repo}
	err := svc.Delete(context.Background(), uuid.New().String())
	require.ErrorIs(t, err, model.ErrImageNotFound)
}

// UPDATESTATUS - SUCCESS
func TestImageService_UpdateStatus_OK(t *testing.T) {
	repo := &mockRepo{
		updateStatusFn: func(ctx context.Context, id string, st model.Status) error {
			require.Equal(t, model.StatusDone, st)
			return nil
		},
	}

	svc := ImageService{repo: repo}
	err := svc.UpdateStatus(context.Background(), uuid.New().String(), model.StatusDone)
	require.NoError(t, err)
}

// SAVERESULT - SUCCESS
func TestImageService_SaveResult_OK(t *testing.T) {
	repo := &mockRepo{
		saveResultFn: func(ctx context.Context, img *model.Image) error {
			require.NotNil(t, img.UpdatedAt)
			return nil
		},
	}

	svc := ImageService{repo: repo}
	err := svc.SaveResult(context.Background(), &model.Image{})
	require.NoError(t, err)
}

// REVIVEORPHANS - SUCCESS
func TestImageService_ReviveOrphans(t *testing.T) {
	called := 0

	repo := &mockRepo{
		fetchOrphansFn: func(ctx context.Context, limit int) ([]string, error) {
			return []string{"id1", "id2"}, nil
		},
	}

	pub := &mockPublisher{
		sendFn: func(ctx context.Context, s retry.Strategy, key []byte, v []byte) error {
			called++
			return nil
		},
	}

	svc := ImageService{repo: repo, publisher: pub}
	svc.ReviveOrphans(context.Background(), 10)

	require.Equal(t, 2, called)
}

// хелпер для создания файла
func newFakeFile(content string) multipart.File {
	return &fakeMultipartFile{
		Reader: bytes.NewReader([]byte(content)),
	}
}

// хелпер для генерации корректного ImageCreateData
func validCreateData() *model.ImageCreateData {
	x := 100

	return &model.ImageCreateData{
		Operation:       string(model.OpResize),
		OrigImg:         newFakeFile("image-bytes"),
		OrigImgSize:     int64(len("image-bytes")),
		OrigContentType: model.JPEG,
		X:               &x,
	}
}
