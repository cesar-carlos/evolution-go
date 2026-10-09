package send_service

import (
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"testing"
)

func TestEveryEXIFTransform(t *testing.T) {
	expected := [][]uint8{
		{1, 2, 3, 4, 5, 6}, {3, 2, 1, 6, 5, 4}, {6, 5, 4, 3, 2, 1}, {4, 5, 6, 1, 2, 3},
		{1, 4, 2, 5, 3, 6}, {4, 1, 5, 2, 6, 3}, {6, 3, 5, 2, 4, 1}, {3, 6, 2, 5, 1, 4},
	}
	src := image.NewRGBA(image.Rect(3, 4, 6, 6))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			src.SetRGBA(x+3, y+4, color.RGBA{R: uint8(y*3 + x + 1), A: 255})
		}
	}
	for orientation, want := range expected {
		dst := orientRGBA(src, orientation+1)
		for i, v := range want {
			x, y := i%dst.Bounds().Dx(), i/dst.Bounds().Dx()
			if dst.RGBAAt(x, y).R != v {
				t.Fatalf("orientation %d pixel %d", orientation+1, i)
			}
		}
	}
}

func TestEXIFTagValidation(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		segment := make([]byte, 32)
		copy(segment, "Exif\x00\x00")
		if order == binary.BigEndian {
			copy(segment[6:], "MM")
		} else {
			copy(segment[6:], "II")
		}
		order.PutUint16(segment[8:], 42)
		order.PutUint32(segment[10:], 8)
		order.PutUint16(segment[14:], 1)
		order.PutUint16(segment[16:], 0x112)
		order.PutUint16(segment[18:], 3)
		order.PutUint32(segment[20:], 1)
		order.PutUint16(segment[24:], 7)
		if exifOrientation(segment) != 7 {
			t.Fatal("byte order not supported")
		}
		order.PutUint16(segment[18:], 4)
		if exifOrientation(segment) != 0 {
			t.Fatal("wrong EXIF type accepted")
		}
		order.PutUint16(segment[18:], 3)
		order.PutUint32(segment[20:], 2)
		if exifOrientation(segment) != 0 {
			t.Fatal("wrong EXIF count accepted")
		}
		order.PutUint32(segment[20:], 1)
		order.PutUint32(segment[10:], 0xffffffff)
		if exifOrientation(segment) != 0 {
			t.Fatal("bad IFD offset accepted")
		}
	}
}

func TestOversizedBitmapRejectedBeforeDecode(t *testing.T) {
	raw := encodePNG(t, 8, 8)
	binary.BigEndian.PutUint32(raw[16:], 30000)
	binary.BigEndian.PutUint32(raw[20:], 30000)
	binary.BigEndian.PutUint32(raw[29:], crc32.ChecksumIEEE(raw[12:29]))
	if w, h := imageDimensions(raw); w != nil || h != nil {
		t.Fatal("oversized dimensions accepted")
	}
	if makeJPEGThumbnail(raw, 72) != nil {
		t.Fatal("oversized thumbnail decoded")
	}
}

func FuzzEXIFParsing(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("Exif\x00\x00MM\x00*\x00\x00\x00\x08"))
	f.Add([]byte{0xff, 0xd8, 0xff, 0xe1, 0, 0})
	f.Fuzz(func(t *testing.T, raw []byte) {
		a, b := exifOrientation(raw), jpegOrientation(raw)
		if a < 0 || a > 8 || b < 1 || b > 8 {
			t.Fatal("invalid orientation")
		}
	})
}
