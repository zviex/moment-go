package gphoto2

import (
	"errors"
	"fmt"

	"ci.zevix.uk/moment-go/pkg/moment"
)

// libgphoto2 返回码，取值与 libgphoto2-android/include/gphoto2/gphoto2-result.h、
// gphoto2-port-result.h 一致。负值表示错误，0（GP_OK）表示成功。
const (
	gpOK                     = 0
	gpError                  = -1
	gpErrorBadParameters     = -2
	gpErrorNoMemory          = -3
	gpErrorLibrary           = -4
	gpErrorUnknownPort       = -5
	gpErrorNotSupported      = -6
	gpErrorIO                = -7
	gpErrorFixedLimit        = -8
	gpErrorTimeout           = -10
	gpErrorIOSupportedSerial = -20
	gpErrorIOSupportedUSB    = -21
	gpErrorIOInit            = -31
	gpErrorIORead            = -34
	gpErrorIOWrite           = -35
	gpErrorIOUpdate          = -37
	gpErrorIOSerialSpeed     = -41
	gpErrorIOUSBClearHalt    = -51
	gpErrorIOUSBFind         = -52
	gpErrorIOUSBClaim        = -53
	gpErrorIOLock            = -60
	gpErrorHAL               = -70
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
)

// resultName 为常见返回码提供可读名称（调试用）。
var resultName = map[int]string{
	gpError:                  "GP_ERROR",
	gpErrorBadParameters:     "GP_ERROR_BAD_PARAMETERS",
	gpErrorNoMemory:          "GP_ERROR_NO_MEMORY",
	gpErrorLibrary:           "GP_ERROR_LIBRARY",
	gpErrorUnknownPort:       "GP_ERROR_UNKNOWN_PORT",
	gpErrorNotSupported:      "GP_ERROR_NOT_SUPPORTED",
	gpErrorIO:                "GP_ERROR_IO",
	gpErrorFixedLimit:        "GP_ERROR_FIXED_LIMIT_EXCEEDED",
	gpErrorTimeout:           "GP_ERROR_TIMEOUT",
	gpErrorIOSupportedSerial: "GP_ERROR_IO_SUPPORTED_SERIAL",
	gpErrorIOSupportedUSB:    "GP_ERROR_IO_SUPPORTED_USB",
	gpErrorIOInit:            "GP_ERROR_IO_INIT",
	gpErrorIORead:            "GP_ERROR_IO_READ",
	gpErrorIOWrite:           "GP_ERROR_IO_WRITE",
	gpErrorIOUpdate:          "GP_ERROR_IO_UPDATE",
	gpErrorIOSerialSpeed:     "GP_ERROR_IO_SERIAL_SPEED",
	gpErrorIOUSBClearHalt:    "GP_ERROR_IO_USB_CLEAR_HALT",
	gpErrorIOUSBFind:         "GP_ERROR_IO_USB_FIND",
	gpErrorIOUSBClaim:        "GP_ERROR_IO_USB_CLAIM",
	gpErrorIOLock:            "GP_ERROR_IO_LOCK",
	gpErrorHAL:               "GP_ERROR_HAL",
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
		sentinel = moment.ErrModelNotFound
	case gpErrorDirectoryNotFound:
		sentinel = moment.ErrDirectoryNotFound
	case gpErrorFileNotFound:
		sentinel = moment.ErrFileNotFound
	case gpErrorCameraBusy:
		sentinel = moment.ErrCameraBusy
	case gpErrorCancel:
		sentinel = moment.ErrCancelled
	case gpErrorOSFailure, gpErrorIO:
		sentinel = moment.ErrOSFailure
	case gpErrorNotSupported:
		sentinel = moment.ErrUnsupported
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
