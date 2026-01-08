package imageproc

import (
	"bytes"
	"image"
	"io"

	"github.com/disintegration/imaging"
)

func Watermark(b, w io.Reader, format imaging.Format) (io.Reader, int64, error) {
	base, err := imaging.Decode(b)
	if err != nil {
		return nil, 0, err
	}
	wm, err := imaging.Decode(w)
	if err != nil {
		return nil, 0, err
	}

	offset := image.Pt(
		base.Bounds().Dx()-wm.Bounds().Dx()-10,
		base.Bounds().Dy()-wm.Bounds().Dy()-10,
	)

	result := imaging.Overlay(base, wm, offset, 0.5)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, result, format); err != nil {
		return nil, 0, err
	}
	return &buf, int64(buf.Len()), nil
}
