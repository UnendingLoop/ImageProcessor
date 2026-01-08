package imageproc

import (
	"bytes"
	"io"

	"github.com/disintegration/imaging"
)

func Thumbnail(r io.Reader, x, y int, format imaging.Format) (io.Reader, int64, error) {
	img, err := imaging.Decode(r)
	if err != nil {
		return nil, 0, err
	}
	thumb := imaging.Thumbnail(img, x, y, imaging.Lanczos)

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumb, format); err != nil {
		return nil, 0, err
	}
	return &buf, int64(buf.Len()), nil
}
