package photo

import (
	"encoding/binary"
	"errors"
)

var (
	ErrHEIC          = errors.New("heic")
	ErrNotImage      = errors.New("not an image")
	ErrTooManyPixels = errors.New("too many pixels")
)

func Detect(header []byte) error {
	if isJPEG(header) || isPNG(header) || isWebP(header) {
		return nil
	}
	if isHEIC(header) {
		return ErrHEIC
	}
	return ErrNotImage
}

func isJPEG(b []byte) bool {
	return len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF
}

func isPNG(b []byte) bool {
	return len(b) >= 8 &&
		b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4E && b[3] == 0x47 &&
		b[4] == 0x0D && b[5] == 0x0A && b[6] == 0x1A && b[7] == 0x0A
}

func isWebP(b []byte) bool {
	return len(b) >= 12 &&
		b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F' &&
		b[8] == 'W' && b[9] == 'E' && b[10] == 'B' && b[11] == 'P'
}

func isHEIC(b []byte) bool {
	if len(b) < 12 || string(b[4:8]) != "ftyp" {
		return false
	}
	end := len(b)
	if size := int(binary.BigEndian.Uint32(b[:4])); size >= 8 && size < end {
		end = size
	}
	if heicBrand(b[8:12]) {
		return true
	}
	for i := 16; i+4 <= end; i += 4 {
		if heicBrand(b[i : i+4]) {
			return true
		}
	}
	return false
}

func heicBrand(b []byte) bool {
	switch string(b) {
	case "heic", "heix", "heif", "heis", "heim", "hevc", "hevx", "mif1", "msf1":
		return true
	}
	return false
}
