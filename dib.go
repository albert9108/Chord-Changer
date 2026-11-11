package main

// Clipboard screenshots arrive as a DIB (CF_DIB); the API wants PNG.

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func dibToPNG(d []byte) ([]byte, error) {
	if len(d) < 40 {
		return nil, fmt.Errorf("clipboard picture is damaged")
	}
	hdr := le32(d[0:])
	w := int(int32(le32(d[4:])))
	h := int(int32(le32(d[8:])))
	bpp := int(uint16(d[14]) | uint16(d[15])<<8)
	comp := le32(d[16:])
	clrUsed := int(le32(d[32:]))
	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("unsupported clipboard picture (%d-bit)", bpp)
	}
	off := int(hdr)
	if comp == 3 && hdr == 40 { // BI_BITFIELDS masks follow the header
		off += 12
	}
	off += clrUsed * 4
	bottomUp := h > 0
	if h < 0 {
		h = -h
	}
	stride := ((w*bpp + 31) / 32) * 4
	if w <= 0 || h <= 0 || off+stride*h > len(d) {
		return nil, fmt.Errorf("clipboard picture is damaged")
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	bytesPP := bpp / 8
	for y := 0; y < h; y++ {
		sy := y
		if bottomUp {
			sy = h - 1 - y
		}
		row := d[off+sy*stride:]
		for x := 0; x < w; x++ {
			p := row[x*bytesPP:]
			img.SetNRGBA(x, y, color.NRGBA{p[2], p[1], p[0], 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
