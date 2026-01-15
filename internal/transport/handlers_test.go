package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wb-go/wbf/ginext"
)

func TestImageHandler_Ping(t *testing.T) {
	r := gin.New()
	h := NewImageHandler(nil)

	r.GET("/ping", func(c *gin.Context) {
		h.SimplePinger((*ginext.Context)(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "pong", body["message"])
}

func newMultipartRequest(t *testing.T, fields map[string]string, files map[string][]byte) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	for name, content := range files {
		fw, err := w.CreateFormFile(name, name+".jpg")
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/images", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestImageHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		req        *http.Request
		mock       *mockImageService
		wantStatus int
	}{
		{
			name: "success",
			req: newMultipartRequest(t,
				map[string]string{"operation": string(model.OpResize), "x_axis": "100"},
				map[string][]byte{"image": []byte("img")},
			),
			mock: &mockImageService{
				createFn: func(ctx context.Context, d *model.ImageCreateData) (*model.Image, error) {
					require.NotNil(t, d.OrigImg)
					return &model.Image{UID: uuid.New()}, nil
				},
			},
			wantStatus: 201,
		},
		{
			name: "missing image",
			req: newMultipartRequest(t,
				map[string]string{"operation": string(model.OpResize)},
				nil,
			),
			mock:       &mockImageService{},
			wantStatus: 400,
		},
		{
			name: "service validation error",
			req: newMultipartRequest(t,
				map[string]string{"operation": "bad-op"},
				map[string][]byte{"image": []byte("img")},
			),
			mock: &mockImageService{
				createFn: func(ctx context.Context, d *model.ImageCreateData) (*model.Image, error) {
					return nil, model.ErrIncorrectOp
				},
			},
			wantStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			h := NewImageHandler(tt.mock)

			r.POST("/images", func(c *gin.Context) {
				h.Create((*ginext.Context)(c))
			})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, tt.req)

			require.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestImageHandler_GetAllImages(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		mock       *mockImageService
		wantStatus int
	}{
		{
			name:  "success",
			query: "?page=1&limit=10",
			mock: &mockImageService{
				getListFn: func(ctx context.Context, req *model.ListRequest) ([]model.Image, error) {
					return []model.Image{{}}, nil
				},
			},
			wantStatus: 200,
		},
		{
			name:       "bad query",
			query:      "?page=abc",
			mock:       &mockImageService{},
			wantStatus: 400,
		},
		{
			name:  "service error",
			query: "",
			mock: &mockImageService{
				getListFn: func(ctx context.Context, req *model.ListRequest) ([]model.Image, error) {
					return nil, model.ErrCommon500
				},
			},
			wantStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			h := NewImageHandler(tt.mock)

			r.GET("/images", func(c *gin.Context) {
				h.GetAllImages((*ginext.Context)(c))
			})

			req := httptest.NewRequest(http.MethodGet, "/images"+tt.query, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)
			require.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestImageHandler_LoadResult(t *testing.T) {
	tests := []struct {
		name       string
		mock       *mockImageService
		wantStatus int
	}{
		{
			name: "success",
			mock: &mockImageService{
				loadResultFn: func(ctx context.Context, id string) (io.ReadCloser, string, error) {
					return io.NopCloser(bytes.NewReader([]byte("ok"))), "image/jpeg", nil
				},
			},
			wantStatus: 200,
		},
		{
			name: "not ready",
			mock: &mockImageService{
				loadResultFn: func(ctx context.Context, id string) (io.ReadCloser, string, error) {
					return nil, "", model.ErrResultNotReady
				},
			},
			wantStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			h := NewImageHandler(tt.mock)

			r.GET("/images/:id/result", func(c *gin.Context) {
				h.LoadResult((*ginext.Context)(c))
			})

			req := httptest.NewRequest(http.MethodGet, "/images/123/result", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)
			require.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestImageHandler_Delete(t *testing.T) {
	tests := []struct {
		name       string
		mock       *mockImageService
		wantStatus int
	}{
		{
			name: "success",
			mock: &mockImageService{
				deleteFn: func(ctx context.Context, id string) error {
					return nil
				},
			},
			wantStatus: 204,
		},
		{
			name: "not found",
			mock: &mockImageService{
				deleteFn: func(ctx context.Context, id string) error {
					return model.ErrImageNotFound
				},
			},
			wantStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			h := NewImageHandler(tt.mock)

			r.DELETE("/images/:id", func(c *gin.Context) {
				h.Delete((*ginext.Context)(c))
			})

			req := httptest.NewRequest(http.MethodDelete, "/images/123", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)
			require.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
