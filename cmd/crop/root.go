package crop

import (
	"fmt"
	"image"
	"image/color"
)

const maxGrayDarkness = 128

func Crop(img image.Image, bounds image.Rectangle) (image.Image, error) {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	if img, ok := img.(subImager); !ok {
		return nil, fmt.Errorf(`image does not support cropping or not a valid image`)
	} else {
		return img.SubImage(bounds), nil
	}
}

func Limits(img image.Image, limit float32) image.Rectangle {
	bounds := img.Bounds()
	maxPixels := float32((bounds.Dx()+bounds.Dy())/2) * limit
	return Bounds(img).Union(bounds.Inset(int(maxPixels)))
}

func Bounds(img image.Image) image.Rectangle {
	left := findBorder(img, image.Pt(1, 0))
	right := findBorder(img, image.Pt(-1, 0))
	top := findBorder(img, image.Pt(0, 1))
	bottom := findBorder(img, image.Pt(0, -1))

	return image.Rect(left.X, top.Y, right.X, bottom.Y)
}

func findBorder(img image.Image, dir image.Point) image.Point {
	bounds := img.Bounds()
	scan := image.Pt(dir.Y, dir.X)
	pt := scanPoint(bounds, dir)

	for !scanImage(img, pt, scan) {
		pt = pt.Add(dir)
		if !pt.In(bounds) {
			pt = scanPoint(bounds, dir)
			break
		}
	}

	if dir.X < 0 || dir.Y < 0 {
		return pt.Sub(dir)
	} else {
		return pt
	}
}

func scanPoint(rect image.Rectangle, dir image.Point) image.Point {
	if dir.X < 0 && dir.Y < 0 {
		return rect.Max.Sub(image.Pt(1, 1))
	}
	return rect.Min
}
func scanImage(img image.Image, pt image.Point, scan image.Point) bool {
	for ; pt.In(img.Bounds()); pt = pt.Add(scan) {
		if gray, ok := color.GrayModel.Convert(img.At(pt.X, pt.Y)).(color.Gray16); ok {
			if gray.Y <= maxGrayDarkness {
				return true
			}
		}
	}
	return false
}

func splitImage(img image.Image) (image.Image, image.Image, error) {
	type subImager interface {
		image.Image
		SubImage(r image.Rectangle) image.Image
	}

	if img, ok := img.(subImager); !ok {
		return nil, nil, fmt.Errorf(`error: image "%v" does not support cropping`, img)
	} else {
		mainBounds := img.Bounds()

		sideX := mainBounds.Dx() / 2
		leftBounds := image.Rectangle{
			Min: image.Point{0, 0},
			Max: image.Point{sideX, mainBounds.Dy()},
		}
		rightBounds := image.Rectangle{
			Min: image.Point{sideX, 0},
			Max: image.Point{mainBounds.Dx(), mainBounds.Dy()},
		}

		return img.SubImage(leftBounds), img.SubImage(rightBounds), nil
	}
}
