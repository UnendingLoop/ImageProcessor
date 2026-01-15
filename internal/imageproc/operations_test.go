package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/require"
)

func testImageReader(t *testing.T, w, h int, format imaging.Format) *bytes.Reader {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 200, A: 255})
		}
	}

	var buf bytes.Buffer
	err := imaging.Encode(&buf, img, format)
	require.NoError(t, err)

	return bytes.NewReader(buf.Bytes())
}

func mustDecode(t *testing.T, r io.Reader) image.Image {
	t.Helper()

	img, err := imaging.Decode(r)
	require.NoError(t, err)
	require.NotNil(t, img)

	return img
}

func TestResizer(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		x, y    int
		wantErr bool
	}{
		{
			name:    "OK resize",
			reader:  testImageReader(t, 200, 100, imaging.PNG),
			x:       50,
			y:       50,
			wantErr: false,
		},
		{
			name:    "nil reader",
			reader:  nil,
			x:       50,
			y:       50,
			wantErr: true,
		},
		{
			name:    "broken image",
			reader:  bytes.NewReader([]byte("not-an-image")),
			x:       50,
			y:       50,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, size, err := Resizer(tt.reader, tt.x, tt.y, imaging.PNG)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, r)
			require.Greater(t, size, int64(0))

			img := mustDecode(t, r)
			require.Equal(t, tt.x, img.Bounds().Dx())
			require.Equal(t, tt.y, img.Bounds().Dy())
		})
	}
}

func TestThumbnailer(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		x, y    int
		wantErr bool
	}{
		{
			name:    "OK thumbnail",
			reader:  testImageReader(t, 300, 200, imaging.PNG),
			x:       100,
			y:       100,
			wantErr: false,
		},
		{
			name:    "nil reader",
			reader:  nil,
			x:       100,
			y:       100,
			wantErr: true,
		},
		{
			name:    "broken image",
			reader:  bytes.NewReader([]byte("broken")),
			x:       100,
			y:       100,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, size, err := Thumbnailer(tt.reader, tt.x, tt.y, imaging.PNG)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, r)
			require.Greater(t, size, int64(0))

			img := mustDecode(t, r)
			require.Equal(t, tt.x, img.Bounds().Dx())
			require.Equal(t, tt.y, img.Bounds().Dy())
		})
	}
}

func TestWatermarker(t *testing.T) {
	tests := []struct {
		name    string
		base    io.Reader
		wm      io.Reader
		wantErr bool
	}{
		{
			name:    "OK watermark",
			base:    testImageReader(t, 400, 300, imaging.PNG),
			wm:      testImageReader(t, 100, 50, imaging.PNG),
			wantErr: false,
		},
		{
			name:    "nil base",
			base:    nil,
			wm:      testImageReader(t, 100, 50, imaging.PNG),
			wantErr: true,
		},
		{
			name:    "nil watermark",
			base:    testImageReader(t, 400, 300, imaging.PNG),
			wm:      nil,
			wantErr: true,
		},
		{
			name:    "broken base image",
			base:    bytes.NewReader([]byte("broken")),
			wm:      testImageReader(t, 100, 50, imaging.PNG),
			wantErr: true,
		},
		{
			name:    "broken watermark image",
			base:    testImageReader(t, 400, 300, imaging.PNG),
			wm:      bytes.NewReader([]byte("broken")),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, size, err := Watermarker(tt.base, tt.wm, imaging.PNG)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, r)
			require.Greater(t, size, int64(0))

			img := mustDecode(t, r)
			require.Equal(t, 400, img.Bounds().Dx())
			require.Equal(t, 300, img.Bounds().Dy())
		})
	}
}
