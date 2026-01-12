// Package imageproc provides operations for images: resizing, thumbnail generation and watermark application.
package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/disintegration/imaging"
)

func Watermarker(b, w io.Reader, format imaging.Format) (io.Reader, int64, error) {
	if b == nil {
		return nil, 0, errors.New("nil-reader baseIMG provided")
	}
	if w == nil {
		return nil, 0, errors.New("nil-reader wmIMG provided")
	}

	base, err := imaging.Decode(b)
	if err != nil {
		return nil, 0, fmt.Errorf("decode base image: %w", err)
	}

	wm, err := imaging.Decode(w)
	if err != nil {
		return nil, 0, fmt.Errorf("decode watermark image: %w", err)
	}

	baseW := base.Bounds().Dx()
	baseH := base.Bounds().Dy()

	// масштабируем watermark до 70 процентов ширины основы
	targetW := int(float64(baseW) * 0.7)

	wm = imaging.Resize(wm, targetW, 0, imaging.Lanczos) // 0 - сохраняет ратио ватермарка

	wmW := wm.Bounds().Dx()
	wmH := wm.Bounds().Dy()

	// находим центр основного изображения
	offset := image.Pt(
		(baseW-wmW)/2,
		(baseH-wmH)/2,
	)

	// само наложение:
	result := imaging.Overlay(base, wm, offset, 0.5)

	// готовим результат к возврату
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, result, format); err != nil {
		return nil, 0, fmt.Errorf("encode result image: %w", err)
	}

	return &buf, int64(buf.Len()), nil
}
