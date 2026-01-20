//go:build linux

package scsi

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	SGIO              = 0x2285
	SG_DXFER_FROM_DEV = -3
)

type sgIOHdr struct {
	InterfaceID    int32
	DxferDirection int32
	CmdLen         uint8
	MxSbLen        uint8
	IovecCount     uint16
	DxferLen       uint32
	Dxferp         uintptr
	Cmdp           uintptr
	Sbp            uintptr
	Timeout        uint32
	Flags          uint32
	PackID         int32
	UsrPtr         uintptr
	Status         uint8
	MaskedStatus   uint8
	MsgStatus      uint8
	SbLenWr        uint8
	HostStatus     uint16
	DriverStatus   uint16
	Resid          int32
	Duration       uint32
	Info           uint32
}

func modeSense6CDB() []byte {
	return []byte{0x1A, 0x08, 0x08, 0x00, 0x12, 0x00}
}

// ParseWCEFromModeSense extracts the Write Cache Enable (WCE) bit from a MODE SENSE(6) caching page response.
// The python implementation checks bit 2 of byte 6.
func ParseWCEFromModeSense(data []byte) (bool, error) {
	if len(data) < 7 {
		return false, fmt.Errorf("mode sense response too short: %d", len(data))
	}
	wceBit := (data[6] >> 2) & 0x01
	return wceBit == 1, nil
}

// GetWCE queries a SCSI disk using SG_IO MODE SENSE(6) caching page.
func GetWCE(devPath string) (bool, error) {
	f, err := os.OpenFile(devPath, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer f.Close()

	cdb := modeSense6CDB()
	data := make([]byte, 18)
	sense := make([]byte, 32)

	hdr := sgIOHdr{
		InterfaceID:    int32('S'),
		DxferDirection: SG_DXFER_FROM_DEV,
		CmdLen:         uint8(len(cdb)),
		MxSbLen:        uint8(len(sense)),
		DxferLen:       uint32(len(data)),
		Dxferp:         uintptr(unsafe.Pointer(&data[0])),
		Cmdp:           uintptr(unsafe.Pointer(&cdb[0])),
		Sbp:            uintptr(unsafe.Pointer(&sense[0])),
		Timeout:        5000,
	}

	// ioctl(fd, SG_IO, &hdr)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(SGIO), uintptr(unsafe.Pointer(&hdr)))
	if errno != 0 {
		return false, errno
	}

	if hdr.Status != 0 {
		return false, fmt.Errorf("SCSI command failed status=%d", hdr.Status)
	}
	return ParseWCEFromModeSense(data)
}
