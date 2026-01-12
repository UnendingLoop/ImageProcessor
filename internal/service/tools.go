package service

import (
	"fmt"
	"strings"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
)

func validateQueryParams(req *model.ListRequest) {
	// Обрабатываем пустые значения, присваиваем дефолты если надо
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 30
	}
	if req.Sort == "" {
		req.Sort = model.ByCreated
	}
	if req.Order == "" {
		req.Order = model.OrderDESC
	}

	// Валидируем непустое поле типа сортировки
	req.Sort = strings.ToLower(req.Sort)
	req.Sort = strings.TrimSpace(req.Sort)
	switch {
	case strings.Contains(req.Sort, model.ByUUID):
		req.Sort = "uid"
	case strings.Contains(req.Sort, model.ByCreated):
		req.Sort = "created_at"
	default:
		req.Sort = "created_at" // по дефолту ставим сортировку по времени создания
	}

	// Валадируем непустой порядок
	req.Order = strings.ToLower(req.Order)
	req.Order = strings.TrimSpace(req.Order)
	switch {
	case strings.Contains(req.Order, model.OrderASC):
		req.Order = "ASC"
	case strings.Contains(req.Order, model.OrderDESC):
		req.Order = "DESC"
	default:
		req.Order = "DESC" // по дефолту ставим сортировку "новое-выше"
	}
}

func validateNormalizeImageInfo(raw *model.ImageCreateData, clean *model.Image) error {
	// корректно ли указана операция
	clean.Operation = model.Operation(raw.Operation)
	if !model.OperationsMap[clean.Operation] {
		return model.ErrIncorrectOp
	}

	// корректен ли исходник
	if raw.OrigImg == nil || raw.OrigImgSize <= 0 || !model.InImageTypeMap[raw.OrigContentType] {
		return model.ErrEmptySource
	}

	// корректен ли ватермарк
	if clean.Operation == model.OpWaterMark && (raw.WMImg == nil || raw.WMImgSize <= 0 || raw.WMContentType != model.PNG) {
		return model.ErrEmptyWMark
	}

	clean.X = raw.X
	clean.Y = raw.Y

	return validateNormalizeOperation(clean)
}

func validateNormalizeOperation(input *model.Image) error {
	switch input.Operation { // проверка согласно самой операции
	case model.OpResize: // допустимо что одно значение нулевое/нуловое
		if (input.X == nil || 0 >= *input.X) &&
			(input.Y == nil || 0 >= *input.Y) {
			return model.ErrIncorrectAxis
		}
	case model.OpThumbNail: // результат должен быть x==y
		if err := validateNormalizeAxisThumbnail(input); err != nil {
			return nil
		}
	}
	return nil
}

func validateNormalizeAxisThumbnail(input *model.Image) error {
	x, y := *input.X, *input.Y
	// кейс: обе оси - нули
	if x <= 0 && y <= 0 {
		return model.ErrIncorrectAxis
	}

	// кейс: одна из осей равна нулю
	if x <= 0 {
		input.X = input.Y
		input.ErrMsg = append(input.ErrMsg, fmt.Sprintf("X-axis incorrect value: using Y-axis value %d for X-axis for generating thumbnail", *input.X))
	}
	if y <= 0 {
		input.Y = input.X
		input.ErrMsg = append(input.ErrMsg, fmt.Sprintf("Y-axis incorrect value: using X-axis value %d for Y-axis for generating thumbnail", *input.Y))
	}

	// кейс: неодинаковые значения - берем меньшее
	if x != y {
		if x > y {
			input.X = input.Y
		} else {
			input.Y = input.X
		}
		input.ErrMsg = append(input.ErrMsg, fmt.Sprintf("Axis values must be equal for thumbnail: using smaller value %d", *input.X))
	}

	return nil
}
