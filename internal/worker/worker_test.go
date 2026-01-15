package worker

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorker_initProcessor(t *testing.T) {
	ctx := context.Background()
	id := uuid.New().String()

	tests := []struct {
		name      string
		image     *model.Image
		getErr    error
		updateErr error
		wantErr   bool
	}{
		{
			name:    "already done",
			image:   &model.Image{Status: model.StatusDone},
			wantErr: false,
		},
		{
			name:    "in progress",
			image:   &model.Image{Status: model.StatusInProgress},
			wantErr: true,
		},
		{
			name:    "image not found",
			getErr:  model.ErrImageNotFound,
			wantErr: true,
		},
		{
			name:      "update status error",
			image:     &model.Image{Status: model.StatusCreated},
			updateErr: errors.New("db down"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockWorkerService{
				getFn: func(ctx context.Context, _ string) (*model.Image, error) {
					return tt.image, tt.getErr
				},
				updateFn: func(ctx context.Context, _ string, _ model.Status) error {
					return tt.updateErr
				},
				saveResultFn: func(ctx context.Context, _ *model.Image) error {
					return nil
				},
			}

			w := &Worker{
				service:      svc,
				storage:      &mockStorage{},
				resultPrefix: "res/",
			}

			err := w.initProcessor(ctx, id)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWorker_processTask_OK(t *testing.T) {
	ctx := context.Background()

	img := &model.Image{
		UID:       uuid.New(),
		Operation: model.OpResize,
		Status:    model.StatusInProgress,
		SourceKey: "src.png",
		X:         ptr(100),
		Y:         ptr(100),
	}

	storage := &mockStorage{
		getFn: func(ctx context.Context, key string) (io.ReadCloser, string, error) {
			return io.NopCloser(bytes.NewReader(validPNG())), model.PNG, nil
		},
		putFn: func(ctx context.Context, key string, size int64, ct string, r io.Reader) error {
			require.Contains(t, key, "res/")
			return nil
		},
	}

	svc := &mockWorkerService{
		saveResultFn: func(ctx context.Context, img *model.Image) error {
			require.Equal(t, model.StatusDone, img.Status)
			require.NotEmpty(t, img.ResultKey)
			return nil
		},
		updateFn: func(ctx context.Context, _ string, _ model.Status) error {
			return nil
		},
		getFn: func(ctx context.Context, _ string) (*model.Image, error) {
			return img, nil
		},
	}

	w := &Worker{
		storage:      storage,
		service:      svc,
		resultPrefix: "res/",
	}

	require.NoError(t, w.processTask(ctx, img))
}

func TestWorker_processTask_BaseImageError(t *testing.T) {
	w := &Worker{
		storage: &mockStorage{
			getFn: func(ctx context.Context, key string) (io.ReadCloser, string, error) {
				return nil, "", errors.New("storage down")
			},
		},
	}

	err := w.processTask(context.Background(), &model.Image{
		Operation: model.OpResize,
	})
	require.Error(t, err)
}

func TestWorker_processTask_UnsupportedFormat(t *testing.T) {
	storage := &mockStorage{
		getFn: func(ctx context.Context, key string) (io.ReadCloser, string, error) {
			return io.NopCloser(bytes.NewReader([]byte("not-an-image"))), "", nil
		},
	}

	w := &Worker{storage: storage}

	err := w.processTask(context.Background(), &model.Image{
		Operation: model.OpResize,
	})
	require.Error(t, err)
}

func TestValidateImgFormat(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wm      bool
		wantErr bool
	}{
		{"valid png", validPNG(), false, false},
		{"valid png wm", validPNG(), true, false},
		{"invalid wm jpeg", validJPEG(), true, true},
		{"invalid data", []byte("xxx"), false, true},
		{"nil reader", nil, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r io.ReadCloser
			if tt.data != nil {
				r = io.NopCloser(bytes.NewReader(tt.data))
			}

			_, _, err := validateImgFormat(r, tt.wm)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func validPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func validJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}
