package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/disintegration/imaging"
)

func Resizer(r io.Reader, x, y int, format imaging.Format) (io.Reader, int64, error) {
	if r == nil {
		return nil, -1, errors.New("nil-reader baseIMG provided to Resizer")
	}

	img, err := imaging.Decode(r)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to DEcode baseIMG in Resizer: %w", err)
	}

	resized := imaging.Resize(img, x, y, imaging.Lanczos)

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, format); err != nil {
		return nil, -1, fmt.Errorf("failed to ENcode resultIMG in Resizer: %w", err)
	}
	return &buf, int64(buf.Len()), nil
}
