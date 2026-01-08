package worker

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"strings"

	"github.com/UnendingLoop/ImageProcessor/internal/imageproc"
	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/UnendingLoop/ImageProcessor/internal/service"
	"github.com/disintegration/imaging"
	kafkago "github.com/segmentio/kafka-go"
	wbfkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/retry"
)

// NoopPublisher - ЗАГЛУШКА, функциональность настоящего паблишера в очередь не нужна в рамках работы воркера
type NoopPublisher struct{}

func (NoopPublisher) SendWithRetry(ctx context.Context, strategy retry.Strategy, task *model.Image) error {
	return nil
}

type ImageWorkerService interface {
	UpdateStatus(ctx context.Context, id string, newStat string) error
	SaveResult(ctx context.Context, id string, resFile io.Reader, resSize int64, cType string) error
	Get(ctx context.Context, id string) (*model.Image, error)
}

type Worker struct {
	storage      service.ImageStorage
	service      ImageWorkerService
	queue        <-chan kafkago.Message
	consumer     *wbfkafka.Consumer
	resultPrefix string
}

func NewWorkerInstance(strg service.ImageStorage, svc ImageWorkerService, q <-chan kafkago.Message, cons *wbfkafka.Consumer, resPr string) *Worker {
	return &Worker{storage: strg, service: svc, queue: q, consumer: cons, resultPrefix: resPr}
}

func (w *Worker) StartWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-w.queue:
			if !ok { // может дочитывать сообщения даже если канал закрыт?
				return
			}
			id := string(msg.Key)
			if err := w.initProcessor(ctx, id); err != nil && !errors.Is(err, model.ErrImageNotFound) {
				log.Printf("task %s failed: %v", id, err)
				w.service.UpdateStatus(ctx, id, string(model.StatusFailed)) // потом добавить отправку в DLQ
				continue
			}
			w.consumer.Commit(ctx, msg)
		}
	}
}

func (w *Worker) initProcessor(ctx context.Context, id string) error {
	// считать из базы задачу
	task, err := w.service.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("Worker failed to fetch image info %q from DB: %w", id, err)
	}
	// проверить статус
	switch task.Status {
	case model.StatusDone:
		return nil
	case model.StatusInProgress:
		return fmt.Errorf("already in progress")
	}

	// на всякий случай проверить поле с результатом
	if strings.Contains(task.ResultKey, w.resultPrefix) {
		if err := w.service.UpdateStatus(ctx, id, string(model.StatusDone)); err != nil {
			return fmt.Errorf("Failed to update status of done task int DB: %w", err)
		}
		return nil
	}

	// обновить статус
	if err := w.service.UpdateStatus(ctx, id, string(model.StatusInProgress)); err != nil {
	}

	if err := w.processTask(ctx, task); err != nil {
	}

	return nil
}

func (w *Worker) processTask(ctx context.Context, task *model.Image) error {
	// достать из storage исходники
	base, _, err := w.storage.Get(ctx, task.SourceKey)
	if err != nil {
		return fmt.Errorf("worker failed to fetch base image %q from storage: %w", task.UID.String(), err)
	}

	wm, _, err := w.storage.Get(ctx, task.WatermarkKey)
	if err != nil && task.Operation == model.OpWaterMark {
		return fmt.Errorf("worker failed to fetch wm image %q from storage: %w", task.UID.String(), err)
	}

	// определить формат выходного файла из cType исходника
	pBase, format, err := validateImgFormat(base, false)
	if err != nil {
		return fmt.Errorf("worker failed to validate base image %q  format: %w", task.UID.String(), err)
	}
	pWm, _, err := validateImgFormat(wm, true)
	if err != nil && task.Operation == model.OpWaterMark {
		return fmt.Errorf("worker failed to validate wm image %q  format: %w", task.UID.String(), err)
	}

	// выполнить операцию
	var result io.Reader
	var size int64
	switch task.Operation {
	case model.OpResize:
		result, size, err = imageproc.Resize(pBase, *task.X, *task.Y, format)
		if err != nil {
			return fmt.Errorf("worker failed to resize image %q: %w", task.UID.String(), err)
		}
	case model.OpThumbNail:
		result, size, err = imageproc.Thumbnail(pBase, *task.X, *task.Y, format)
		if err != nil {
			return fmt.Errorf("worker failed to generate thumbnail from image %q: %w", task.UID.String(), err)
		}
	case model.OpWaterMark:
		result, size, err = imageproc.Watermark(pBase, pWm, format)
		if err != nil {
			return fmt.Errorf("worker failed to apply wm on image %q: %w", task.UID.String(), err)
		}
	default:
		return model.ErrIncorrectOp
	}

	// положить результат в сторедж если ошибок нет на предыдущем этапе
	resCType := model.GetCType[format]
	resKey := w.resultPrefix + task.UID.String() + model.GetImageFileExt[resCType]
	if err := w.storage.Put(ctx, resKey, size, resCType, result); err != nil {
		return fmt.Errorf("worker failed to put reeult image %q to storage: %w", task.UID.String(), err)
	}

	task.Status = model.StatusDone
	task.ResultKey = resKey

	return nil
}

func validateImgFormat(r io.ReadCloser, wm bool) (io.Reader, imaging.Format, error) {
	br := bufio.NewReader(r)

	// читаем первые 512 байт для определения формата - должно быть достаточно?
	header, err := br.Peek(512)
	if err != nil {
		return nil, -1, err
	}

	_, f, err := image.DecodeConfig(bytes.NewReader(header))
	if err != nil {
		return nil, -1, err
	}

	format, err := imaging.FormatFromExtension(f)
	if err != nil {
		return nil, -1, err
	}

	// отдельная проверка для формата ватермарка
	if wm && format != imaging.PNG {
		return nil, -1, model.ErrUnsupportedWMFormat
	}

	if format != imaging.JPEG || format != imaging.PNG || format != imaging.GIF {
		return nil, -1, model.ErrUnsupportedFormat
	}

	// возвращаем результат - все ок
	return br, format, nil
}
