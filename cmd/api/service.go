package main

import (
	"context"
	"io"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
)

type ImageAPIRepository interface {
	Create(ctx context.Context, n *model.Image) (*model.Image, error)
	Delete(ctx context.Context, id int) error
	GetList(ctx context.Context, req *model.ListRequest) ([]model.Image, error)
}
type ImageAPIService interface {
	Create(context.Context, *model.ImageCreateData) (*model.Image, error)
	LoadResult(ctx context.Context, id string) (io.ReadCloser, string, error)
	GetList(ctx context.Context, req *model.ListRequest) ([]model.Image, error)
	Delete(ctx context.Context, id string) error
}
