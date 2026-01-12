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
		return nil, -1, errors.New("nil-reader baseIMG provided to Watermarker")
	}
	if w == nil {
		return nil, -1, errors.New("nil-reader mwIMG provided to Watermarker")
	}

	base, err := imaging.Decode(b)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to DEcode baseIMG in Watermarker: %w", err)
	}
	wm, err := imaging.Decode(w)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to DEcode wmIMG in Watermarker: %w", err)
	}

	offset := image.Pt(
		base.Bounds().Dx()-wm.Bounds().Dx()-10,
		base.Bounds().Dy()-wm.Bounds().Dy()-10,
	)

	result := imaging.Overlay(base, wm, offset, 0.5)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, result, format); err != nil {
		return nil, 0, fmt.Errorf("failed to ENcode resultIMG in Watermarker: %w", err)
	}
	return &buf, int64(buf.Len()), nil
}
