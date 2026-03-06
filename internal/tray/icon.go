package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// IconActive returns the tray icon for active tracking (purple).
func IconActive() []byte {
	return buildIcon(
		color.RGBA{R: 90, G: 60, B: 220, A: 255},  // ring
		color.RGBA{R: 30, G: 25, B: 50, A: 255},    // fill
		color.RGBA{R: 180, G: 160, B: 255, A: 255},  // hands
		color.RGBA{R: 220, G: 200, B: 255, A: 255},  // center
	)
}

// IconPaused returns the tray icon for paused tracking (grey).
func IconPaused() []byte {
	return buildIcon(
		color.RGBA{R: 120, G: 120, B: 120, A: 255}, // ring
		color.RGBA{R: 50, G: 50, B: 50, A: 255},    // fill
		color.RGBA{R: 170, G: 170, B: 170, A: 255},  // hands
		color.RGBA{R: 200, G: 200, B: 200, A: 255},  // center
	)
}

func buildIcon(ring, fill, hand, center color.RGBA) []byte {
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
				img.SetRGBA(x, y, ring)
			} else if dist < innerR {
				img.SetRGBA(x, y, fill)
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
			img.SetRGBA(ix, iy, hand)
			if ix+1 < size {
				img.SetRGBA(ix+1, iy, hand)
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
			img.SetRGBA(ix, iy, hand)
			if iy+1 < size {
				img.SetRGBA(ix, iy+1, hand)
			}
		}
	}
	// Center dot
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			img.SetRGBA(int(cx)+dx, int(cy)+dy, center)
		}
	}

	// Encode as PNG
	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)
	pngBytes := pngBuf.Bytes()

	// Wrap in ICO container (PNG-compressed, supported on Vista+)
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, uint16(0))
	binary.Write(&ico, binary.LittleEndian, uint16(1))
	binary.Write(&ico, binary.LittleEndian, uint16(1))
	ico.WriteByte(uint8(size))
	ico.WriteByte(uint8(size))
	ico.WriteByte(0)
	ico.WriteByte(0)
	binary.Write(&ico, binary.LittleEndian, uint16(1))
	binary.Write(&ico, binary.LittleEndian, uint16(32))
	binary.Write(&ico, binary.LittleEndian, uint32(len(pngBytes)))
	binary.Write(&ico, binary.LittleEndian, uint32(22))
	ico.Write(pngBytes)

	return ico.Bytes()
}
