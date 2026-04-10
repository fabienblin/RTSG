package main

import (
	"main/GUI"
	"main/audio"
	"main/config"
	"main/render"
)

var configService *config.ConfigService
var GUIService *GUI.GUIService
var renderService *render.RenderService
var audioService *audio.AudioService

func init() {
	configService = config.New()
	audioService = audio.New(configService.Config.Audio)
	renderService = render.New(configService.Config.Renderer)
	GUIService = GUI.New(configService.Config, renderService, audioService)
}

func main() {
	go audioService.Run()
	GUIService.Run()
}
