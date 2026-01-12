package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/disintegration/imaging"
)

func Thumbnailer(r io.Reader, x, y int, format imaging.Format) (io.Reader, int64, error) {
	if r == nil {
		return nil, -1, errors.New("nil-reader baseIMG provided to Thumbnailer")
	}
	img, err := imaging.Decode(r)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to DEcode baseIMG in Thumbnailer: %w", err)
	}
	thumb := imaging.Thumbnail(img, x, y, imaging.Lanczos)

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumb, format); err != nil {
		return nil, 0, fmt.Errorf("failed to ENcode resultIMG in Thumbnailer: %w", err)
	}
	return &buf, int64(buf.Len()), nil
}
