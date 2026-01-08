// Package api provides methods for processing requests from endpoints
package api

import (
	"context"
	"errors"
	"io"
	"log"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/wb-go/wbf/ginext"
)

type ImageHandler struct {
	service ImageService
}

type ImageService interface {
	Create(ctx context.Context, newImage *model.ImageCreateData) (*model.Image, error)
	Delete(ctx context.Context, id string) error                                // удалить как в базе, так и в minio
	LoadResult(ctx context.Context, id string) (io.ReadCloser, string, error)   // прям скачать результат
	GetList(ctx context.Context, req *model.ListRequest) ([]model.Image, error) // получить список
}

func NewImageHandler(svc ImageService) *ImageHandler {
	return &ImageHandler{
		service: svc,
	}
}

func (h ImageHandler) SimplePinger(ctx *ginext.Context) {
	ctx.JSON(200, map[string]string{"message": "pong"})
}

func (h ImageHandler) Create(ctx *ginext.Context) {
	var newImageRaw model.ImageCreateData
	// чтение метаданных для задачи
	if err := ctx.BindJSON(&newImageRaw); err != nil {
		ctx.JSON(400, map[string]string{"error": err.Error()})
		return
	}

	if err := ctx.Request.ParseMultipartForm(32 << 20); err != nil {
		ctx.JSON(400, map[string]string{"error": "invalid multipart form"})
		return
	}

	// парсинг исходника
	imageFile, imageHeader, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(400, map[string]string{"error": "image is required"})
		return
	}
	defer imageFile.Close()

	newImageRaw.OrigImg = imageFile
	newImageRaw.OrigContentType = imageHeader.Header.Get("Content-Type")
	newImageRaw.OrigImgSize = imageHeader.Size

	// парсинг ватермарка если есть
	wmFile, wmHeader, err := ctx.Request.FormFile("watermark")
	if err != nil {
		// watermark опционален
		wmFile = nil
	} else {
		defer wmFile.Close()
	}
	newImageRaw.WMImg = wmFile
	newImageRaw.WMContentType = wmHeader.Header.Get("Content-Type")
	newImageRaw.WMImgSize = wmHeader.Size

	// передаем все в сервис
	res, err := h.service.Create(ctx.Request.Context(), &newImageRaw)
	if err != nil {
		ctx.JSON(errorCodeDefiner(err), map[string]string{"error": err.Error()})
		return
	}

	ctx.JSON(201, res)
}

func (h ImageHandler) GetAllImages(ctx *ginext.Context) {
	var req model.ListRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(400, map[string]string{"error": "failed to parse query-params"})
		return
	}

	res, err := h.service.GetList(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(errorCodeDefiner(err), map[string]string{"error": err.Error()})
		return
	}

	ctx.JSON(200, res)
}

func (h ImageHandler) LoadResult(ctx *ginext.Context) {
	id := ctx.Param("id")

	res, cType, err := h.service.LoadResult(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(errorCodeDefiner(err), map[string]string{"error": err.Error()})
		return
	}
	defer res.Close()

	ctx.Writer.Header().Set("Content-Type", cType)
	ctx.Writer.WriteHeader(200)
	if n, err := io.Copy(ctx.Writer, res); err != nil {
		log.Printf("Failed to write response at byte %d for file id %q: %v", n, id, err)
	}
}

func (h ImageHandler) Delete(ctx *ginext.Context) {
	id := ctx.Param("id")
	if err := h.service.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(errorCodeDefiner(err), map[string]string{"error": err.Error()})
		return
	}

	ctx.Status(204)
}

func errorCodeDefiner(err error) int {
	switch {
	case errors.Is(err, model.ErrCommon500):
		return 500
	case errors.Is(err, model.ErrImageNotFound):
		return 404
	case errors.Is(err, model.ErrIncorrectQuery) ||
		errors.Is(err, model.ErrIncorrectID) ||
		errors.Is(err, model.ErrIncorrectOp) ||
		errors.Is(err, model.ErrEmptySource) ||
		errors.Is(err, model.ErrEmptyWMark) ||
		errors.Is(err, model.ErrIncorrectAxis) ||
		errors.Is(err, model.ErrIncorrectStatus):
		return 400
	default:
		return 500
	}
}
