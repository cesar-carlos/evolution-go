// Adapted from Evolution Go PR #212 by mediam4kers; local bounds and EXIF validation.
package send_service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"google.golang.org/protobuf/proto"
	"image"
)

const maxDecodedImagePixels = 25_000_000

// checkedImageConfig bounds decoded bitmap allocation before decoding pixels.
func checkedImageConfig(raw []byte) (image.Config, string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return cfg, format, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || uint64(cfg.Width) > uint64(maxDecodedImagePixels)/uint64(cfg.Height) {
		return cfg, format, errors.New("image exceeds decoded pixel limit")
	}
	return cfg, format, nil
}

// imageDimensions reads the pixel size of an image without decoding it fully.
// WhatsApp clients use ImageMessage.Width/Height to size the chat bubble before
// the media is downloaded; without them iOS falls back to a square bubble and
// center-crops portrait and landscape images. It returns nil pointers when the
// size cannot be read, so the message is still sent (just without dimensions).
func imageDimensions(fileData []byte) (width, height *uint32) {
	cfg, format, err := checkedImageConfig(fileData)
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return nil, nil
	}
	w, h := cfg.Width, cfg.Height
	// Phone cameras often store the JPEG sideways with an EXIF orientation of 5-8;
	// clients display it rotated, so the bubble must use the displayed size.
	if format == "jpeg" && jpegOrientation(fileData) >= 5 {
		w, h = h, w
	}
	return proto.Uint32(uint32(w)), proto.Uint32(uint32(h))
}

// jpegOrientation returns the EXIF orientation (1-8) of a JPEG, or 1 when the
// data is not a JPEG or carries no readable orientation tag.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF: // fill byte
			i++
			continue
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7): // markers without a length
			i += 2
			continue
		case marker == 0xDA || marker == 0xD9: // image data starts: no EXIF after this point
			return 1
		}
		size := int(data[i+2])<<8 | int(data[i+3])
		if size < 2 || i+2+size > len(data) {
			return 1
		}
		if marker == 0xE1 {
			if orientation := exifOrientation(data[i+4 : i+2+size]); orientation != 0 {
				return orientation
			}
		}
		i += 2 + size
	}
	return 1
}

// exifOrientation reads tag 0x0112 from IFD0 of an APP1 "Exif" segment payload.
// It returns 0 when the segment is not EXIF or has no valid orientation.
func exifOrientation(segment []byte) int {
	if len(segment) < 14 || string(segment[:6]) != "Exif\x00\x00" {
		return 0
	}
	tiff := segment[6:]
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 0
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 0
	}
	entries := int(order.Uint16(tiff[ifd : ifd+2]))
	for k := 0; k < entries; k++ {
		entry := ifd + 2 + k*12
		if entry+12 > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[entry:entry+2]) == 0x0112 {
			if order.Uint16(tiff[entry+2:entry+4]) != 3 || order.Uint32(tiff[entry+4:entry+8]) != 1 {
				return 0
			}
			orientation := int(order.Uint16(tiff[entry+8 : entry+10]))
			if orientation >= 1 && orientation <= 8 {
				return orientation
			}
			return 0
		}
	}
	return 0
}

// orientRGBA returns src transformed the way a viewer applies an EXIF
// orientation (2-8): mirrored and/or rotated so it reads upright.
func orientRGBA(src *image.RGBA, orientation int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dw, dh := sw, sh
	if orientation >= 5 {
		dw, dh = sh, sw
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch orientation {
			case 2: // mirror horizontal
				sx, sy = sw-1-x, y
			case 3: // rotate 180
				sx, sy = sw-1-x, sh-1-y
			case 4: // mirror vertical
				sx, sy = x, sh-1-y
			case 5: // transpose
				sx, sy = y, x
			case 6: // rotate 90 clockwise
				sx, sy = y, sh-1-x
			case 7: // transverse
				sx, sy = sw-1-y, sh-1-x
			case 8: // rotate 90 counter-clockwise
				sx, sy = sw-1-y, x
			default:
				sx, sy = x, y
			}
			dst.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}
