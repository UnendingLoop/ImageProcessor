package main

import (
	"context"
	"io"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/wb-go/wbf/retry"
)

type ImageWorkerService interface {
	UpdateStatus(ctx context.Context, id string, newStat string) error
	SaveResult(ctx context.Context, id string, resFile io.Reader, resSize int64, cType string) error
	Get(ctx context.Context, id string) (*model.Image, error)
}

// NoopPublisher - ЗАГЛУШКА, функциональность настоящего паблишера в очередь не нужна в рамках работы воркера
type NoopPublisher struct{}

func (NoopPublisher) SendWithRetry(ctx context.Context, strategy retry.Strategy, k []byte, v []byte) error {
	return nil
}
