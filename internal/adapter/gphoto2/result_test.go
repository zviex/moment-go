package gphoto2

import (
	"errors"
	"strings"
	"testing"

	"ci.zevix.uk/moment-go/internal/domain"
)

func TestResultToError(t *testing.T) {
	if err := resultToError(0); err != nil {
		t.Fatalf("GP_OK should be nil, got %v", err)
	}

	cases := []struct {
		code  int
		error error
	}{
		{gpErrorModelNotFound, domain.ErrModelNotFound},
		{gpErrorDirectoryNotFound, domain.ErrDirectoryNotFound},
		{gpErrorFileNotFound, domain.ErrFileNotFound},
		{gpErrorCameraBusy, domain.ErrCameraBusy},
		{gpErrorCancel, domain.ErrCancelled},
		{gpErrorOSFailure, domain.ErrOSFailure},
		{gpErrorIO, domain.ErrOSFailure},
		{gpErrorNotSupported, domain.ErrUnsupported},
	}
	for _, c := range cases {
		err := resultToError(c.code)
		if err == nil {
			t.Fatalf("code %d should error", c.code)
		}
		if !errors.Is(err, c.error) {
			t.Fatalf("code %d: got %v, want %v", c.code, err, c.error)
		}
	}

	unknown := resultToError(-42)
	if unknown == nil {
		t.Fatal("unknown code should error")
	}
	if !strings.Contains(unknown.Error(), "libgphoto2") {
		t.Fatalf("unexpected message: %v", unknown)
	}
}
