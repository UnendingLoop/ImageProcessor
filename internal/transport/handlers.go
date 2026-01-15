// Package transport provides methods for processing requests from endpoints
package transport

import (
	"context"
	"io"
	"log"
	"strconv"

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
	operation := ctx.PostForm("operation")
	xStr := ctx.PostForm("x_axis")
	yStr := ctx.PostForm("y_axis")

	// конвертация x, y в int если они есть
	var x, y *int
	if xStr != "" {
		val, _ := strconv.Atoi(xStr)
		x = &val
	}
	if yStr != "" {
		val, _ := strconv.Atoi(yStr)
		y = &val
	}

	// парсинг исходника
	var imageSize int64
	imageFile, imageHeader, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(400, map[string]string{"error": "image is required"})
		return
	}
	defer closeFileFlow(imageFile)
	imageCType := imageHeader.Header.Get("Content-Type")
	imageSize = imageHeader.Size
	// парсинг ватермарка если есть
	var wmCType string
	var wmSize int64
	wmFile, wmHeader, err := ctx.Request.FormFile("watermark")
	if err != nil {
		// watermark опционален
		wmFile = nil
	} else {
		wmCType = wmHeader.Header.Get("Content-Type")
		wmSize = wmHeader.Size
		defer closeFileFlow(wmFile)
	}

	// собираем все в структуру
	var newImageRaw model.ImageCreateData
	newImageRaw.Operation = operation
	newImageRaw.X = x
	newImageRaw.Y = y
	newImageRaw.OrigImg = imageFile
	newImageRaw.OrigContentType = imageCType
	newImageRaw.OrigImgSize = imageSize
	newImageRaw.WMImg = wmFile
	newImageRaw.WMContentType = wmCType
	newImageRaw.WMImgSize = wmSize

	// передаем в сервис
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
	defer closeFileFlow(res)

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
