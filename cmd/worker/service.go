package main

import (
	"context"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/wb-go/wbf/retry"
)

type ImageWorkerService interface {
	UpdateStatus(ctx context.Context, id string, newStat model.Status) error
	SaveResult(ctx context.Context, input *model.Image) error
	Get(ctx context.Context, id string) (*model.Image, error)
}

// NoopPublisher - ЗАГЛУШКА, функциональность настоящего паблишера в очередь не нужна в рамках работы воркера
type NoopPublisher struct{}

func (NoopPublisher) SendWithRetry(ctx context.Context, strategy retry.Strategy, k []byte, v []byte) error {
	return nil
}
