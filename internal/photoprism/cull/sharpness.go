package cull

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"

	"github.com/photoprism/photoprism/pkg/fs"
)

// LaplacianVariance returns a sharpness estimate for an image file.
// Higher values generally indicate a sharper image. Returns 0 on failure.
func LaplacianVariance(fileName string) float64 {
	if fileName == "" || !fs.FileExists(fileName) {
		return 0
	}

	f, err := os.Open(fileName)
	if err != nil {
		return 0
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return 0
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 3 || h < 3 {
		return 0
	}

	// Downsample to grayscale luminance.
	gray := make([]float64, w*h)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Standard luminance in 0-255 range.
			lum := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256.0
			gray[(y-bounds.Min.Y)*w+(x-bounds.Min.X)] = lum
		}
	}

	var sum, sumSq float64
	var n float64

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			// 4-neighbor Laplacian.
			v := -4*gray[i] + gray[i-1] + gray[i+1] + gray[i-w] + gray[i+w]
			sum += v
			sumSq += v * v
			n++
		}
	}

	if n < 1 {
		return 0
	}

	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance < 0 || math.IsNaN(variance) {
		return 0
	}

	return variance
}
