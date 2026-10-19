// Package img generates thumbnails for stored images.
package img

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Sizes for generated thumbnails (max dimension in px).
const (
	SizeS = 240
	SizeM = 480
	SizeL = 960
)

// MakeThumbs decodes data, writes s/m/l JPEG thumbnails into dir with the
// given base name, and returns their file names.
func MakeThumbs(data []byte, dir, base string) (sName, mName, lName string, err error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", "", "", err
	}
	mk := func(size int, suffix string) (string, error) {
		dst := resize(src, size)
		name := fmt.Sprintf("%s_%s.jpg", base, suffix)
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		defer f.Close()
		if err := jpeg.Encode(f, dst, &jpeg.Options{Quality: 82}); err != nil {
			return "", err
		}
		return name, nil
	}
	if sName, err = mk(SizeS, "s"); err != nil {
		return "", "", "", err
	}
	if mName, err = mk(SizeM, "m"); err != nil {
		return "", "", "", err
	}
	if lName, err = mk(SizeL, "l"); err != nil {
		return "", "", "", err
	}
	return sName, mName, lName, nil
}

// resize scales src so its longest side is maxDim (never upscales).
func resize(src image.Image, maxDim int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	longest := w
	if h > w {
		longest = h
	}
	if longest <= maxDim {
		return src
	}
	scale := float64(maxDim) / float64(longest)
	nw, nh := int(float64(w)*scale+0.5), int(float64(h)*scale+0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
