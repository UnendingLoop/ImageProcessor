package worker

import (
	"context"
	"io"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
)

type mockWorkerService struct {
	getFn        func(ctx context.Context, id string) (*model.Image, error)
	updateFn     func(ctx context.Context, id string, st model.Status) error
	saveResultFn func(ctx context.Context, img *model.Image) error
}

func (m *mockWorkerService) Get(ctx context.Context, id string) (*model.Image, error) {
	return m.getFn(ctx, id)
}

func (m *mockWorkerService) UpdateStatus(ctx context.Context, id string, st model.Status) error {
	return m.updateFn(ctx, id, st)
}

func (m *mockWorkerService) SaveResult(ctx context.Context, img *model.Image) error {
	return m.saveResultFn(ctx, img)
}

//----------------------------------

type mockStorage struct {
	getFn func(ctx context.Context, key string) (io.ReadCloser, string, error)
	putFn func(ctx context.Context, key string, size int64, ct string, r io.Reader) error
}

func (m *mockStorage) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return m.getFn(ctx, key)
}

func (m *mockStorage) Put(ctx context.Context, key string, size int64, ct string, r io.Reader) error {
	return m.putFn(ctx, key, size, ct, r)
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	return nil
}
