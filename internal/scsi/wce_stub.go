//go:build !linux

package scsi

import "fmt"

func ParseWCEFromModeSense(data []byte) (bool, error) {
	if len(data) < 7 {
		return false, fmt.Errorf("mode sense response too short: %d", len(data))
	}
	wceBit := (data[6] >> 2) & 0x01
	return wceBit == 1, nil
}

func GetWCE(devPath string) (bool, error) {
	return false, fmt.Errorf("GetWCE unsupported on this platform")
}
