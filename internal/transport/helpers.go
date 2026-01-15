package transport

import (
	"errors"
	"io"
	"log"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
)

func errorCodeDefiner(err error) int {
	switch {
	case errors.Is(err, model.ErrCommon500):
		return 500
	case errors.Is(err, model.ErrImageNotFound),
		errors.Is(err, model.ErrResultNotReady):
		return 404
	case errors.Is(err, model.ErrIncorrectQuery),
		errors.Is(err, model.ErrIncorrectID),
		errors.Is(err, model.ErrIncorrectOp),
		errors.Is(err, model.ErrEmptySource),
		errors.Is(err, model.ErrEmptyWMark),
		errors.Is(err, model.ErrIncorrectAxis),
		errors.Is(err, model.ErrIncorrectStatus),
		errors.Is(err, model.ErrUnsupportedWMFormat),
		errors.Is(err, model.ErrUnsupportedFormat):
		return 400
	default:
		return 500
	}
}

func closeFileFlow(res io.ReadCloser) {
	if res == nil {
		return
	}
	if err := res.Close(); err != nil {
		log.Println("Handler failed to close fileflow:", err)
	}
}
