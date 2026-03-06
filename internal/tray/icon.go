package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// generateIcon creates a 32x32 ICO with a simple clock/timer design.
func generateIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	cx, cy := float64(size)/2, float64(size)/2
	outerR := float64(size)/2 - 1
	innerR := outerR - 3

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx + 0.5
			dy := float64(y) - cy + 0.5
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist <= outerR && dist >= innerR {
				// Ring: blue-purple
				img.SetRGBA(x, y, color.RGBA{R: 90, G: 60, B: 220, A: 255})
			} else if dist < innerR {
				// Fill: dark
				img.SetRGBA(x, y, color.RGBA{R: 30, G: 25, B: 50, A: 255})
			}
		}
	}

	// Clock hand pointing ~2 o'clock
	for i := 0; i < 10; i++ {
		t := float64(i) / 10.0
		hx := cx + t*8
		hy := cy - t*6
		ix, iy := int(hx), int(hy)
		if ix >= 0 && ix < size && iy >= 0 && iy < size {
			img.SetRGBA(ix, iy, color.RGBA{R: 180, G: 160, B: 255, A: 255})
			if ix+1 < size {
				img.SetRGBA(ix+1, iy, color.RGBA{R: 180, G: 160, B: 255, A: 255})
			}
		}
	}
	// Short hand pointing ~10 o'clock
	for i := 0; i < 6; i++ {
		t := float64(i) / 6.0
		hx := cx - t*6
		hy := cy - t*4
		ix, iy := int(hx), int(hy)
		if ix >= 0 && ix < size && iy >= 0 && iy < size {
			img.SetRGBA(ix, iy, color.RGBA{R: 180, G: 160, B: 255, A: 255})
			if iy+1 < size {
				img.SetRGBA(ix, iy+1, color.RGBA{R: 180, G: 160, B: 255, A: 255})
			}
		}
	}
	// Center dot
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			img.SetRGBA(int(cx)+dx, int(cy)+dy, color.RGBA{R: 220, G: 200, B: 255, A: 255})
		}
	}

	// Encode as PNG
	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)
	pngBytes := pngBuf.Bytes()

	// Wrap in ICO container (PNG-compressed, supported on Vista+)
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&ico, binary.LittleEndian, uint16(1)) // type = icon
	binary.Write(&ico, binary.LittleEndian, uint16(1)) // count = 1
	// ICONDIRENTRY
	ico.WriteByte(uint8(size))                                     // width
	ico.WriteByte(uint8(size))                                     // height
	ico.WriteByte(0)                                               // color count
	ico.WriteByte(0)                                               // reserved
	binary.Write(&ico, binary.LittleEndian, uint16(1))             // planes
	binary.Write(&ico, binary.LittleEndian, uint16(32))            // bpp
	binary.Write(&ico, binary.LittleEndian, uint32(len(pngBytes))) // data size
	binary.Write(&ico, binary.LittleEndian, uint32(22))            // data offset (6 + 16)
	ico.Write(pngBytes)

	return ico.Bytes()
}
