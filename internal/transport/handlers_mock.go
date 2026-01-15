package transport

import (
	"context"
	"io"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/gin-gonic/gin"
)

type mockImageService struct {
	createFn     func(ctx context.Context, d *model.ImageCreateData) (*model.Image, error)
	deleteFn     func(ctx context.Context, id string) error
	loadResultFn func(ctx context.Context, id string) (io.ReadCloser, string, error)
	getListFn    func(ctx context.Context, req *model.ListRequest) ([]model.Image, error)
}

func (m *mockImageService) Create(ctx context.Context, d *model.ImageCreateData) (*model.Image, error) {
	return m.createFn(ctx, d)
}

func (m *mockImageService) Delete(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}

func (m *mockImageService) LoadResult(ctx context.Context, id string) (io.ReadCloser, string, error) {
	return m.loadResultFn(ctx, id)
}

func (m *mockImageService) GetList(ctx context.Context, req *model.ListRequest) ([]model.Image, error) {
	return m.getListFn(ctx, req)
}

func init() {
	gin.SetMode(gin.TestMode)
}
