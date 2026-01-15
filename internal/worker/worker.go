// Package worker contains methods for worker to init at start, and to process images
package worker

import (
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

type ImageWorkerService interface { // дублируется из cmd/worker - может вынести такие структуры/контракты в отдельный пакет(не model)?
	UpdateStatus(ctx context.Context, id string, newStat model.Status) error
	SaveResult(ctx context.Context, res *model.Image) error
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
			if !ok {
				log.Println("Queue channel closed, stopping worker...")
				return
			}
			id := string(msg.Key)
			if err := w.initProcessor(ctx, id); err != nil && !errors.Is(err, model.ErrImageNotFound) {
				log.Printf("Task %s failed: %v", id, err)
				continue
			}
			if err := w.consumer.Commit(ctx, msg); err != nil {
				log.Printf("Failed to commit queue-message: %v", err)
			}
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
		if err := w.service.UpdateStatus(ctx, id, model.StatusDone); err != nil {
			return fmt.Errorf("failed to update status of already-done task in DB: %w", err)
		}
		return nil
	}

	// обновить статус
	if err := w.service.UpdateStatus(ctx, id, model.StatusInProgress); err != nil {
		return fmt.Errorf("failed to update status of task %q to `in_progress` in DB: %w", id, err)
	}

	// выполняем саму операцию
	if pErr := w.processTask(ctx, task); pErr != nil {
		if uErr := w.service.UpdateStatus(ctx, id, model.StatusFailed); uErr != nil {
			return fmt.Errorf("failed to set status of task %q to `failed` in DB: %w \nAFTER\n error while processing task: %w", id, uErr, pErr)
		}
		return fmt.Errorf("failed to process task %q: %w", id, pErr)
	}

	return nil
}

func (w *Worker) processTask(ctx context.Context, task *model.Image) error {
	// достать из storage исходники
	base, _, err := w.storage.Get(ctx, task.SourceKey)
	if err != nil {
		return fmt.Errorf("worker failed to fetch base-image from storage: %w", err)
	}
	defer closeFileFlow(base)

	wm, _, err := w.storage.Get(ctx, task.WatermarkKey)
	if err != nil && task.Operation == model.OpWaterMark {
		return fmt.Errorf("worker failed to fetch wm-image from storage: %w", err)
	}
	defer closeFileFlow(wm)

	// определить формат выходного файла из cType исходника
	pBase, format, err := validateImgFormat(base, false)
	if err != nil {
		return fmt.Errorf("worker failed to validate base-image format: %w", err)
	}

	// свалидировать формат ватермарка
	pWm, _, err := validateImgFormat(wm, true)
	if err != nil && task.Operation == model.OpWaterMark {
		return fmt.Errorf("worker failed to validate wm-image format: %w", err)
	}

	// выполнить операцию
	var result io.Reader
	var size int64
	switch task.Operation {
	case model.OpResize:
		result, size, err = imageproc.Resizer(pBase, *task.X, *task.Y, format)
		if err != nil {
			return fmt.Errorf("worker failed to resize image: %w", err)
		}
	case model.OpThumbNail:
		result, size, err = imageproc.Thumbnailer(pBase, *task.X, *task.Y, format)
		if err != nil {
			return fmt.Errorf("worker failed to generate thumbnail from image: %w", err)
		}
	case model.OpWaterMark:
		result, size, err = imageproc.Watermarker(pBase, pWm, format)
		if err != nil {
			return fmt.Errorf("worker failed to apply wm on image: %w", err)
		}
	default:
		return model.ErrIncorrectOp
	}

	// положить результат в сторедж если ошибок нет на предыдущем этапе
	resCType := model.GetCType[format]
	resKey := w.resultPrefix + task.UID.String() + model.GetImageFileExt[resCType]
	if err := w.storage.Put(ctx, resKey, size, resCType, result); err != nil {
		return fmt.Errorf("worker failed to put result image to storage: %w", err)
	}

	task.Status = model.StatusDone
	task.ResultKey = resKey

	// обновить запись в БД
	if err := w.service.SaveResult(ctx, task); err != nil {
		return fmt.Errorf("worker failed to save result to DB: %w", err)
	}
	return nil
}

func validateImgFormat(r io.ReadCloser, wm bool) (io.Reader, imaging.Format, error) {
	if r == nil {
		return nil, -1, errors.New("nil-reader provided")
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, -1, err
	}

	_, f, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, -1, err
	}

	format, err := imaging.FormatFromExtension(f)
	if err != nil {
		return nil, -1, err
	}

	if wm && format != imaging.PNG {
		return nil, -1, model.ErrUnsupportedWMFormat
	}

	switch format {
	case imaging.PNG, imaging.JPEG, imaging.GIF:
	default:
		return nil, -1, model.ErrUnsupportedFormat
	}

	return bytes.NewReader(data), format, nil
}

func closeFileFlow(res io.ReadCloser) {
	if res == nil {
		return
	}

	if err := res.Close(); err != nil {
		log.Println("Worker failed to close fileflow:", err)
	}
}
