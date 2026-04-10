package config

import "image/color"

type Configuration struct {
	GUI      GUI      `json:"GUI"`
	Renderer Renderer `json:"renderer"`
	Audio    Audio    `json:"audio"`
}

type Window struct {
	Height float32 `json:"height"`
	Width  float32 `json:"width"`
}

type GUI struct {
	Fps            int    `json:"fps"`
	MainWindow     Window `json:"mainwindow"`
	GraphicsWindow Window `json:"graphicsWindow"`
}

type Renderer struct {
	Size    Image   `json:"size"`
	BgColor Color   `json:"bgColor"`
	Lerp    float64 `json:"lerp"`
}

type Image struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type Color struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"`
}

func (c Color) AsRGBA() color.RGBA {
	return color.RGBA{c.R, c.G, c.B, c.A}
}

type Audio struct {
	SampleMultiplier float64 `json:"sampleMultiplier"`
}
