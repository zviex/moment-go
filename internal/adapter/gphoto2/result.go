package gphoto2

import (
	"errors"
	"fmt"

	"ci.zevix.uk/moment-go/internal/domain"
)

// libgphoto2 返回码（gphoto2-result.h）。负值表示错误，0 表示成功。
const (
	gpOK                     = 0
	gpError                  = -1
	gpErrorNoMemory          = -2
	gpErrorCorruptedData     = -102
	gpErrorFileExists        = -103
	gpErrorModelNotFound     = -105
	gpErrorDirectoryNotFound = -107
	gpErrorFileNotFound      = -108
	gpErrorDirectoryExists   = -109
	gpErrorCameraBusy        = -110
	gpErrorPathNotAbsolute   = -111
	gpErrorCancel            = -112
	gpErrorCameraError       = -113
	gpErrorOSFailure         = -114
	gpErrorNoSpace           = -115
	gpErrorIO                = -116
	gpErrorTimeout           = -118
	gpErrorNotSupported      = -119
	gpErrorIORead            = -121
	gpErrorIOWrite           = -122
	gpErrorIOUSBClaim        = -126
	gpErrorIOLock            = -127
	gpErrorIOPTPCancel       = -143
	gpErrorIOPTPPermission   = -144
	gpErrorIOPTPTimeout      = -146
	gpErrorIOPTPRefused      = -147
	gpErrorIOPTPEOF          = -148
	gpErrorIOPTPDeviceBusy   = -149
	gpErrorIOPTPUnknown      = -150
)

// resultName 为常见返回码提供可读名称（调试用）。
var resultName = map[int]string{
	gpError:                  "GP_ERROR",
	gpErrorNoMemory:          "GP_ERROR_NO_MEMORY",
	gpErrorCorruptedData:     "GP_ERROR_CORRUPTED_DATA",
	gpErrorFileExists:        "GP_ERROR_FILE_EXISTS",
	gpErrorModelNotFound:     "GP_ERROR_MODEL_NOT_FOUND",
	gpErrorDirectoryNotFound: "GP_ERROR_DIRECTORY_NOT_FOUND",
	gpErrorFileNotFound:      "GP_ERROR_FILE_NOT_FOUND",
	gpErrorDirectoryExists:   "GP_ERROR_DIRECTORY_EXISTS",
	gpErrorCameraBusy:        "GP_ERROR_CAMERA_BUSY",
	gpErrorPathNotAbsolute:   "GP_ERROR_PATH_NOT_ABSOLUTE",
	gpErrorCancel:            "GP_ERROR_CANCEL",
	gpErrorCameraError:       "GP_ERROR_CAMERA_ERROR",
	gpErrorOSFailure:         "GP_ERROR_OS_FAILURE",
	gpErrorNoSpace:           "GP_ERROR_NO_SPACE",
	gpErrorIO:                "GP_ERROR_IO",
	gpErrorTimeout:           "GP_ERROR_TIMEOUT",
	gpErrorNotSupported:      "GP_ERROR_NOT_SUPPORTED",
	gpErrorIORead:            "GP_ERROR_IO_READ",
	gpErrorIOWrite:           "GP_ERROR_IO_WRITE",
	gpErrorIOUSBClaim:        "GP_ERROR_IO_USB_CLAIM",
	gpErrorIOLock:            "GP_ERROR_IO_LOCK",
	gpErrorIOPTPCancel:       "GP_ERROR_IO_PTP_CANCEL",
	gpErrorIOPTPPermission:   "GP_ERROR_IO_PTP_PERMISSION",
	gpErrorIOPTPTimeout:      "GP_ERROR_IO_PTP_TIMEOUT",
	gpErrorIOPTPRefused:      "GP_ERROR_IO_PTP_REFUSED",
	gpErrorIOPTPEOF:          "GP_ERROR_IO_PTP_EOF",
	gpErrorIOPTPDeviceBusy:   "GP_ERROR_IO_PTP_DEVICEBUSY",
	gpErrorIOPTPUnknown:      "GP_ERROR_IO_PTP_UNKNOWN",
}

// resultToError 将 libgphoto2 返回码转换为 Go 错误。
// 0 表示成功，返回 nil。
func resultToError(code int) error {
	if code == gpOK {
		return nil
	}
	var sentinel error
	switch code {
	case gpErrorModelNotFound:
		sentinel = domain.ErrModelNotFound
	case gpErrorDirectoryNotFound:
		sentinel = domain.ErrDirectoryNotFound
	case gpErrorFileNotFound:
		sentinel = domain.ErrFileNotFound
	case gpErrorCameraBusy, gpErrorIOPTPDeviceBusy:
		sentinel = domain.ErrCameraBusy
	case gpErrorCancel, gpErrorIOPTPCancel:
		sentinel = domain.ErrCancelled
	case gpErrorOSFailure:
		sentinel = domain.ErrOSFailure
	case gpErrorNotSupported:
		sentinel = domain.ErrUnsupported
	}
	name := resultName[code]
	if name == "" {
		name = "unknown"
	}
	msg := fmt.Sprintf("libgphoto2: %s (code=%d)", name, code)
	if sentinel != nil {
		return fmt.Errorf("%w: %s", sentinel, msg)
	}
	return errors.New(msg)
}
