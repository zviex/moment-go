package download

import "testing"

func TestParseFileRefMomentScheme(t *testing.T) {
	ref := parseFileRef("moment:///store_00010001/DCIM/100CANON/IMG_0002.JPG")
	if ref.folder != "/store_00010001/DCIM/100CANON" {
		t.Fatalf("folder = %q", ref.folder)
	}
	if ref.name != "IMG_0002.JPG" {
		t.Fatalf("name = %q", ref.name)
	}
}

func TestParseFileRefPlainPath(t *testing.T) {
	ref := parseFileRef("/store_00010001/DCIM/100CANON/IMG_0002.JPG")
	if ref.folder != "/store_00010001/DCIM/100CANON" {
		t.Fatalf("folder = %q", ref.folder)
	}
	if ref.name != "IMG_0002.JPG" {
		t.Fatalf("name = %q", ref.name)
	}
}

func TestParseFileRefRoot(t *testing.T) {
	ref := parseFileRef("/store_00010001")
	if ref.folder != "/" {
		t.Fatalf("folder = %q", ref.folder)
	}
	if ref.name != "store_00010001" {
		t.Fatalf("name = %q", ref.name)
	}
}
