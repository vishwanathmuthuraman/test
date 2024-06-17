package graphs

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
)

// DecodeLineGraphValues returns a map of instant_second to retention percent
func DecodeLineGraphValues(imagePath string, nSeconds int) (map[int]int, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	valuesMap := make(map[int]float64)
	valuesMapScaled := make(map[int]int)

	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	xStart := 0
	xEnd := width
	yTop := 0
	yBottom := height

	firstX := -1
	firstY := -1
	lastX := 0

search:
	for x := xEnd / 2; x <= xEnd; x++ {
		for y := yBottom; y > yTop; y-- {
			if isGuidePixel(img, x, y) {
				// check that it is in fact a guide and not a label etc
				if isGuidePixel(img, x+1, y) &&
					isGuidePixel(img, x+10, y) {
					yBottom = y
					break search // we only want the bottom most gray line
				}
			}
		}
	}

	lineWidth := 0
	{
		for x := xStart; x <= xEnd; x++ {
			for y := yTop; y <= yBottom; y++ {
				if isLinePixel(img, x, y) {
					if firstX == -1 {
						firstX = x
					}
					if firstY == -1 {
						firstY = y
					}
					if y == firstY {
						lineWidth++
					}
				}
			}
		}
	}

	{ // check linewidth bp
		// Iterate over the x-axis and extract values from the line graph.
		for x := xStart + lineWidth/2; x <= xEnd; x++ {
			// Iterate over a vertical strip of pixels along the y-axis.
			for y := yTop; y <= yBottom; y++ {
				// You may need to adjust this threshold based on the line color in your graph.
				if isLinePixel(img, x, y) {
					if y == firstY {
						lineWidth++
					}
					value := float64(yBottom-(y+lineWidth/2)) / float64(yBottom-firstY)
					if value >= 0 && value <= 1 {
						lastX = x
						valuesMap[x] = value
						break
					} else {
						fmt.Println("value out of range", value)
					}
				}
			}
			if valuesMap[x] == 0 {
				fmt.Println("no line pixel", x)
			}
		}

		for i := 0; i < nSeconds; i++ {
			pixelsPerSecond := float64(lastX-firstX) / float64(nSeconds)
			originalIndex := int(float64(i)*pixelsPerSecond) + firstX
			valuesMapScaled[i] = int(valuesMap[originalIndex] * 100)
		}
	}

	return valuesMapScaled, nil
}

func isLinePixel(img image.Image, x, y int) bool {
	// You may need to adjust this threshold based on the line color in your graph.
	//lineColorThreshold := 100
	r, g, b, _ := img.At(x, y).RGBA()
	//avgColor := int((r + g + b) / 3)
	r = r / 257
	g = g / 257
	b = b / 257
	return (r <= 200 && r >= 0) && (g <= 255 && g >= 50) && (b <= 255 && b >= 50)
}

func isGuidePixel(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	//avgColor := int((r + g + b) / 3)
	r = r / 257
	g = g / 257
	b = b / 257
	return (r <= 250 && r >= 28) && (g <= 250 && g >= 28) && (b <= 250 && b >= 28)
}
