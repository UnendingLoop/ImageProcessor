package imageproc

import (
	"bytes"
	"io"

	"github.com/disintegration/imaging"
)

func Resize(r io.Reader, x, y int, format imaging.Format) (io.Reader, int64, error) {
	img, err := imaging.Decode(r)
	if err != nil {
		return nil, 0, err
	}

	resized := imaging.Resize(img, x, y, imaging.Lanczos)

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, format); err != nil {
		return nil, 0, err
	}
	return &buf, int64(buf.Len()), nil
}
