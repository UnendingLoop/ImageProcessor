// Package model provides data-structs for internal app-usage
package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

type (
	Status    string
	Operation string
)

const (
	StatusCreated    Status = "created"
	StatusInProgress Status = "in_progress"
	StatusFailed     Status = "failed"
	StatusDone       Status = "done"
)

var StatusMap = map[Status]bool{
	StatusCreated:    true,
	StatusInProgress: true,
	StatusFailed:     true,
	StatusDone:       true,
}

const (
	OpResize    Operation = "resize"
	OpWaterMark Operation = "watermark"
	OpThumbNail Operation = "thumbnail"
)

var OperationsMap = map[Operation]bool{
	OpResize:    true,
	OpWaterMark: true,
	OpThumbNail: true,
}

//---------------------

type Image struct {
	UID          uuid.UUID   `json:"uid"`
	SourceKey    string      `json:"-"`
	WatermarkKey string      `json:"-"`
	ResultKey    string      `json:"-"`
	Operation    Operation   `json:"operation"`
	X            *int        `json:"x_axis,omitempty"`
	Y            *int        `json:"y_axis,omitempty"`
	Status       Status      `json:"status,omitempty"`
	ErrMsg       StringSlice `json:"error,omitempty"`
	CreatedAt    *time.Time  `json:"created_at,omitempty"`
	UpdatedAt    *time.Time  `json:"updated_at,omitempty"`
}

//-------------------

type ListRequest struct {
	Page  int    `form:"page"`
	Limit int    `form:"limit"`
	Sort  string `form:"sort"`
	Order string `form:"order"`
}

const (
	ByUUID    = "uid"
	ByCreated = "created"
	OrderASC  = "ascend"
	OrderDESC = "descend"
)

type ImageCreateData struct {
	Operation       string
	X               *int
	Y               *int
	OrigImg         multipart.File
	OrigContentType string
	OrigImgSize     int64
	WMImg           multipart.File
	WMContentType   string
	WMImgSize       int64
}

// ------------------

var (
	ErrCommon500           error = errors.New("something went wrong. Try again later") // 500
	ErrIncorrectQuery      error = errors.New("incorrect query parameters")            // 400
	ErrIncorrectID         error = errors.New("incorrect image UUID")                  // 400
	ErrImageNotFound       error = errors.New("specified image UUID doesn't exist")    // 404
	ErrResultNotReady      error = errors.New("requested image is not processed yet")  // 404
	ErrIncorrectOp         error = errors.New("operation is not supported")            // 400
	ErrEmptySource         error = errors.New("empty/incorrect source image provided") // 400
	ErrEmptyWMark          error = errors.New("empty/incorrect watermark provided")    // 400
	ErrIncorrectAxis       error = errors.New("incorrect axis values provided")        // 400
	ErrIncorrectStatus     error = errors.New("incorrect status provided")             // 400
	ErrUnsupportedWMFormat error = errors.New("unsupported watermark-image format")    // 400
	ErrUnsupportedFormat   error = errors.New("unsupported base image format")         // 400
)

//--------------------

const (
	JPEG = "image/jpeg"
	PNG  = "image/png"
	GIF  = "image/gif"
)

var GetImageFileExt = map[string]string{
	JPEG: ".jpg",
	PNG:  ".png",
	GIF:  ".gif",
}

var InImageTypeMap = map[string]bool{
	JPEG: true,
	PNG:  true,
	GIF:  true,
}

var GetCType = map[imaging.Format]string{
	imaging.JPEG: JPEG,
	imaging.GIF:  GIF,
	imaging.PNG:  PNG,
}

//--------------------

type StringSlice []string

func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = []string{}
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("invalid type for StringSlice")
	}

	if err := json.Unmarshal(b, s); err != nil {
		return fmt.Errorf("failed to unmarshal JSONB to []StringSlice: %w", err)
	}
	return nil
}

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 || s == nil {
		return []byte(`[]`), nil
	}
	res, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal []StringSlice to JSONB: %w", err)
	}

	return res, nil
}
