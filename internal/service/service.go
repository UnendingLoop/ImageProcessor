// Package service provides business-logic for the app
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/UnendingLoop/ImageProcessor/internal/mwlogger"
	"github.com/UnendingLoop/ImageProcessor/internal/repository"
	"github.com/google/uuid"
	"github.com/wb-go/wbf/retry"
)

type ImageService struct {
	repo            repository.ImageRepo
	publisher       TaskPublisher
	storage         ImageStorage
	srcKeyPrefix    string
	wmKeyPrefix     string
	resultKeyPrefix string
}

func NewImageService(commentRep repository.ImageRepo, pub TaskPublisher, strg ImageStorage) *ImageService {
	return &ImageService{
		repo:      commentRep,
		publisher: pub,
		storage:   strg,
	}
}

// TaskPublisher - контракт для работы с очередью
type TaskPublisher interface {
	SendWithRetry(ctx context.Context, strategy retry.Strategy, key []byte, v []byte) error
}

// ImageStorage - контракт для работы с хранилищем
type ImageStorage interface {
	Delete(ctx context.Context, uid string) error
	Get(ctx context.Context, key string) (output io.ReadCloser, ctype string, err error)
	Put(ctx context.Context, key string, size int64, contentType string, r io.Reader) error
}

// Стратегия ретрая отправки в очередь - можно потом вынести значения в конфиг/env
var retryStrategy = retry.Strategy{
	Attempts: 5,
	Delay:    3 * time.Second,
	Backoff:  1.5,
}

func (c ImageService) Create(ctx context.Context, imageData *model.ImageCreateData) (*model.Image, error) {
	logger := mwlogger.LoggerFromContext(ctx)
	newImage := &model.Image{}

	// Валидируем операцию
	if err := validateNormalizeImageInfo(imageData, newImage); err != nil {
		return nil, err
	}

	// генерируем UUID
	newImage.UID = uuid.New()

	// кладем в хранилище сорсник
	srcKey := c.srcKeyPrefix + newImage.UID.String() + model.GetImageFileExt[imageData.OrigContentType]

	if err := c.storage.Put(ctx, srcKey, imageData.OrigImgSize, imageData.OrigContentType, imageData.OrigImg); err != nil {
		logger.Error().Err(err).Msg("Failed to save src-image in Storage")
		return nil, model.ErrCommon500
	}

	// кладем в хранилище ватермарк - если надо по типу операции
	if newImage.Operation == model.OpWaterMark {
		wmKey := c.wmKeyPrefix + newImage.UID.String() + model.GetImageFileExt[imageData.WMContentType]

		if err := c.storage.Put(ctx, wmKey, imageData.WMImgSize, imageData.WMContentType, imageData.WMImg); err != nil {
			logger.Error().Err(err).Msg("Failed to save watermark in Storage")
			return nil, model.ErrCommon500
		}
	}

	// ставим статус и таймстамп
	newImage.Status = model.StatusCreated
	now := time.Now().UTC()
	newImage.CreatedAt = &now

	// шлем в базу
	if err := c.repo.Create(ctx, newImage); err != nil {
		logger.Error().Err(err).Msg("Failed to create image in DB")
		return nil, model.ErrCommon500
	}

	// кладем в очередь задач(в кафку)
	if err := c.publisher.SendWithRetry(ctx, retryStrategy, []byte(newImage.UID.String()), nil); err != nil {
		logger.Error().Err(err).Msg(fmt.Sprintf("Failed to publish image %q to task-queue", newImage.UID))
		return nil, model.ErrCommon500
	}
	return newImage, nil
}

func (c ImageService) GetList(ctx context.Context, req *model.ListRequest) ([]model.Image, error) {
	logger := mwlogger.LoggerFromContext(ctx)
	validateQueryParams(req)

	res, err := c.repo.GetList(ctx, req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch all images list from DB")
		return nil, model.ErrCommon500
	}

	return res, nil
}

func (c ImageService) Get(ctx context.Context, id string) (*model.Image, error) {
	logger := mwlogger.LoggerFromContext(ctx)
	if err := uuid.Validate(id); err != nil {
		return nil, model.ErrIncorrectID
	}

	res, err := c.repo.Get(ctx, id)
	if err != nil {
		logger.Error().Err(err).Msg(fmt.Sprintf("Failed to fetch image %q from DB", id))
		return nil, model.ErrCommon500
	}

	return res, nil
}

func (c ImageService) LoadResult(ctx context.Context, id string) (io.ReadCloser, string, error) {
	logger := mwlogger.LoggerFromContext(ctx)
	if err := uuid.Validate(id); err != nil {
		return nil, "", model.ErrIncorrectID
	}

	res, err := c.repo.Get(ctx, id)
	if err != nil {
		logger.Error().Err(err).Msg(fmt.Sprintf("Failed to fetch image %q from DB", id))
		return nil, "", model.ErrCommon500
	}
	if res.Status != model.StatusDone {
		return nil, "", model.ErrResultNotReady
	}

	// достаем из хранилища
	data, cType, err := c.storage.Get(ctx, res.ResultKey)
	if err != nil {
		logger.Error().Err(err).Msg(fmt.Sprintf("Failed to fetch result-image %q from Storage", id))
		return nil, "", model.ErrCommon500
	}
	return data, cType, nil
}

func (c ImageService) Delete(ctx context.Context, id string) error {
	logger := mwlogger.LoggerFromContext(ctx)
	if err := uuid.Validate(id); err != nil {
		return model.ErrIncorrectID
	}

	// читаем из базы
	res, err := c.repo.Get(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return model.ErrImageNotFound // 404
		default:
			logger.Error().Err(err).Msg(fmt.Sprintf("Failed to fetch image %q from DB", id))
			return model.ErrCommon500
		}
	}

	// удаляем из базы
	if err := c.repo.Delete(ctx, id); err != nil {
		logger.Error().Err(err).Msg("Failed to delete image from DB")
		return model.ErrCommon500
	}

	// удаляем из хранилища сорсник, результат и ватермарк(если они есть)
	if err := c.storage.Delete(ctx, res.SourceKey); err != nil {
		logger.Error().Err(err).Msg("Failed to delete src-image from Storage")
		return model.ErrCommon500
	}
	if res.Status == model.StatusDone {
		if err := c.storage.Delete(ctx, res.ResultKey); err != nil {
			logger.Error().Err(err).Msg("Failed to delete result-image from Storage")
			return model.ErrCommon500
		}
	}
	if res.Operation == model.OpWaterMark {
		if err := c.storage.Delete(ctx, res.WatermarkKey); err != nil {
			logger.Error().Err(err).Msg("Failed to delete watermark from Storage")
			return model.ErrCommon500
		}
	}

	return nil
}

func (c ImageService) UpdateStatus(ctx context.Context, id string, newStat model.Status) error {
	if err := uuid.Validate(id); err != nil {
		return model.ErrIncorrectID
	}

	logger := mwlogger.LoggerFromContext(ctx)

	if err := c.repo.UpdateStatus(ctx, id, newStat); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return model.ErrImageNotFound // 404
		default:
			logger.Error().Err(err).Msg("Failed to update image status in DB")
			return model.ErrCommon500 // 500
		}
	}

	return nil
}

func (c ImageService) SaveResult(ctx context.Context, input *model.Image) error {
	logger := mwlogger.LoggerFromContext(ctx)
	t := time.Now().UTC()
	input.UpdatedAt = &t
	if err := c.repo.SaveResult(ctx, input); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return model.ErrImageNotFound // 404
		default:
			logger.Error().Err(err).Msg("Failed to save result image in DB")
			return model.ErrCommon500 // 500
		}
	}

	return nil
}

func (c ImageService) ReviveOrphans(ctx context.Context, limit int) {
	logger := mwlogger.LoggerFromContext(ctx)

	orphans, err := c.repo.FetchOrphans(ctx, limit)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to load orphans from DB")
		return
	}

	for _, v := range orphans {
		if err := c.publisher.SendWithRetry(ctx, retryStrategy, []byte(v), nil); err != nil {
			logger.Error().Err(err).Msg("Failed to publish orphan to queue")
		}
	}
}
