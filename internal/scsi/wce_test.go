package scsi

import "testing"

func TestParseWCEFromModeSense(t *testing.T) {
	data := make([]byte, 18)
	// set byte6 bit2
	data[6] = 0b00000100
	wce, err := ParseWCEFromModeSense(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !wce {
		t.Fatalf("expected wce true")
	}
	data[6] = 0
	wce, _ = ParseWCEFromModeSense(data)
	if wce {
		t.Fatalf("expected wce false")
	}
}
