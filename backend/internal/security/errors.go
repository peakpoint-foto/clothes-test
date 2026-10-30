package security

import "fmt"

type APIError struct {
	HTTPStatus int    `json:"-"`
	BizCode    int    `json:"code"`
	ErrorCode  string `json:"error_code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%d:%s] %s", e.BizCode, e.ErrorCode, e.Message)
}

func NewAPIError(httpStatus, bizCode int, errCode, message string) *APIError {
	return &APIError{
		HTTPStatus: httpStatus,
		BizCode:    bizCode,
		ErrorCode:  errCode,
		Message:    message,
	}
}

var (
	ErrAuthRequired = NewAPIError(401, 40101, "AUTH_REQUIRED", "Hãy nhập token truy cập backend trong Trung tâm cài đặt")
	ErrAuthInvalid  = NewAPIError(401, 40102, "AUTH_INVALID", "Token truy cập backend không hợp lệ")

	ErrOriginNotAllowed = NewAPIError(403, 40301, "ORIGIN_NOT_ALLOWED", "Nguồn trang hiện tại không được phép truy cập backend")

	ErrRateLimited        = NewAPIError(429, 42901, "RATE_LIMITED", "Quá nhiều request, vui lòng thử lại sau")
	ErrDailyQuotaExceeded = NewAPIError(429, 42902, "DAILY_QUOTA_EXCEEDED", "Đã hết hạn mức tạo trong ngày")
	ErrConcurrencyLimited = NewAPIError(429, 42903, "CONCURRENCY_LIMITED", "Đang có nhiều tác vụ tạo, vui lòng thử lại sau")

	ErrRequestTooLarge         = NewAPIError(413, 41301, "REQUEST_TOO_LARGE", "Nội dung tải lên quá lớn, hãy nén ảnh rồi thử lại")
	ErrInvalidImageBase64      = NewAPIError(422, 42201, "INVALID_IMAGE_BASE64", "Dữ liệu ảnh bị hỏng, hãy chọn lại ảnh")
	ErrUnsupportedImageType    = NewAPIError(422, 42202, "UNSUPPORTED_IMAGE_TYPE", "Chỉ hỗ trợ JPEG, PNG và WebP")
	ErrImageFileTooLarge       = NewAPIError(422, 42203, "IMAGE_FILE_TOO_LARGE", "File ảnh quá lớn, hãy nén rồi thử lại")
	ErrImageDimensionsInvalid  = NewAPIError(422, 42204, "IMAGE_DIMENSIONS_INVALID", "Kích thước ảnh không hợp lệ")
	ErrImagePixelLimitExceeded = NewAPIError(422, 42205, "IMAGE_PIXEL_LIMIT_EXCEEDED", "Số pixel ảnh quá cao, hãy thu nhỏ rồi thử lại")
)
