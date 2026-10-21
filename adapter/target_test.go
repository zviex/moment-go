package adapter

import (
	"context"
	"testing"

	"ci.zevix.uk/moment-go/pkg/moment"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

type fakeDiscoverer struct {
	cams []moment.DetectedCamera
	err  error
}

func (f fakeDiscoverer) Detect(context.Context) ([]moment.DetectedCamera, error) {
	return f.cams, f.err
}

func usbOption(vendor, product uint32) *connectionv1.ConnectOption {
	return &connectionv1.ConnectOption{
		Mode: connectionv1.ConnectionMode_CONNECTION_MODE_USB,
		Payload: &connectionv1.ConnectOption_Usb{
			Usb: &connectionv1.ConnectCameraUSB{
				UsbVendorId:  vendor,
				UsbProductId: product,
			},
		},
	}
}

func ptpOption(portPath string) *connectionv1.ConnectOption {
	return &connectionv1.ConnectOption{
		Mode: connectionv1.ConnectionMode_CONNECTION_MODE_PTPIP,
		Payload: &connectionv1.ConnectOption_Ptp{
			Ptp: &connectionv1.ConnectCameraPTP{PortPath: portPath},
		},
	}
}

func TestSplitHostPort(t *testing.T) {
	cases := []struct {
		in      string
		addr    string
		port    int
		wantErr bool
	}{
		{"192.168.1.100", "192.168.1.100", 0, false},
		{"192.168.1.100:15740", "192.168.1.100", 15740, false},
		{"ptpip:192.168.1.100", "192.168.1.100", 0, false},
		{"ptpip:192.168.1.100:8080", "192.168.1.100", 8080, false},
		{"camera.local", "camera.local", 0, false},
		{"[fe80::1]", "fe80::1", 0, false},
		{"[fe80::1]:15740", "fe80::1", 15740, false},
		{"", "", 0, true},
		{"ptpip:", "", 0, true},
		{"[", "", 0, true},
	}
	for _, c := range cases {
		addr, port, err := splitHostPort(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("splitHostPort(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("splitHostPort(%q): %v", c.in, err)
		}
		if addr != c.addr || port != c.port {
			t.Fatalf("splitHostPort(%q) = (%q, %d), want (%q, %d)", c.in, addr, port, c.addr, c.port)
		}
	}
}

func TestResolvePTPIP(t *testing.T) {
	r := NewTargetResolver(nil)
	target, err := r.Resolve(context.Background(), ptpOption("192.168.1.100"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.PortPath != "ptpip:192.168.1.100:15740" {
		t.Fatalf("port path = %q", target.PortPath)
	}
	if target.Model != ptpipCameraModel {
		t.Fatalf("model = %q", target.Model)
	}
}

func TestResolvePTPIPRequiresPayload(t *testing.T) {
	r := NewTargetResolver(nil)
	if _, err := r.Resolve(context.Background(), &connectionv1.ConnectOption{
		Mode: connectionv1.ConnectionMode_CONNECTION_MODE_PTPIP,
	}); err == nil {
		t.Fatal("expected error for missing ptp payload")
	}
}

func TestResolvePTPIPRequiresHost(t *testing.T) {
	r := NewTargetResolver(nil)
	if _, err := r.Resolve(context.Background(), ptpOption("")); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestResolveUSBByIDs(t *testing.T) {
	r := NewTargetResolver(fakeDiscoverer{cams: []moment.DetectedCamera{
		{Model: "CamA", PortPath: "usb:001,002", USBVendorID: 0x1111, USBProductID: 0x2222},
		{Model: "CamB", PortPath: "usb:001,003", USBVendorID: 0x3333, USBProductID: 0x4444},
	}})
	target, err := r.Resolve(context.Background(), usbOption(0x3333, 0x4444))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.Model != "CamB" || target.PortPath != "usb:001,003" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestResolveUSBUnknownIDs(t *testing.T) {
	r := NewTargetResolver(fakeDiscoverer{cams: []moment.DetectedCamera{
		{Model: "CamA", PortPath: "usb:001,002", USBVendorID: 0x1111, USBProductID: 0x2222},
	}})
	if _, err := r.Resolve(context.Background(), usbOption(0x9999, 0x8888)); err == nil {
		t.Fatal("expected error for unknown usb ids")
	}
}

func TestResolveUSBWithoutIDsUsesFirstDetected(t *testing.T) {
	r := NewTargetResolver(fakeDiscoverer{cams: []moment.DetectedCamera{
		{Model: "CamA", PortPath: "usb:001,002", USBVendorID: 0x1111, USBProductID: 0x2222},
	}})
	target, err := r.Resolve(context.Background(), usbOption(0, 0))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.Model != "CamA" || target.PortPath != "usb:001,002" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestResolveInfersModeFromPayload(t *testing.T) {
	r := NewTargetResolver(nil)
	opt := ptpOption("camera.local")
	opt.Mode = connectionv1.ConnectionMode_CONNECTION_MODE_UNSPECIFIED
	target, err := r.Resolve(context.Background(), opt)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.PortPath != "ptpip:camera.local:15740" {
		t.Fatalf("port path = %q", target.PortPath)
	}
}

func TestResolveAutoFirstDetected(t *testing.T) {
	r := NewTargetResolver(fakeDiscoverer{cams: []moment.DetectedCamera{
		{Model: "CamA", PortPath: "usb:001,002"},
		{Model: "CamB", PortPath: "usb:001,003"},
	}})
	target, err := r.Resolve(context.Background(), &connectionv1.ConnectOption{
		Mode: connectionv1.ConnectionMode_CONNECTION_MODE_AUTO,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.Model != "CamA" || target.PortPath != "usb:001,002" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestResolveNoCamera(t *testing.T) {
	r := NewTargetResolver(fakeDiscoverer{})
	if _, err := r.Resolve(context.Background(), nil); err == nil {
		t.Fatal("expected error when no camera detected")
	}
}
