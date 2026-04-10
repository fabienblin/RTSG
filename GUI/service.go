package GUI

import (
	"main/audio"
	"main/config"
	"main/render"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

type GUIService struct {
	config        config.Configuration
	fyneApp       fyne.App
	window        fyne.Window
	renderService *render.RenderService
	audioService  *audio.AudioService
}

func New(config config.Configuration, renderService *render.RenderService, auioService *audio.AudioService) *GUIService {
	fyneApp := app.New()
	window := fyneApp.NewWindow("Real Time Sound Graphics")
	window.Resize(fyne.NewSize(config.GUI.GraphicsWindow.Width, config.GUI.GraphicsWindow.Height))

	objects := renderService.Objects()
	graphicsContainer := container.NewWithoutLayout(objects...)
	window.SetContent(graphicsContainer)

	return &GUIService{
		config:        config,
		fyneApp:       fyneApp,
		window:        window,
		renderService: renderService,
		audioService:  auioService,
	}
}

func (s *GUIService) Run() {
	s.startAnimation()
	s.window.Show()
	s.fyneApp.Run()
}

func (s *GUIService) startAnimation() {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.GUI.Fps))

	go func() {
		for range ticker.C {
			s.fyneApp.Driver().DoFromGoroutine(
				func() {
					nbRings := s.renderService.NbRings()
					samples := s.audioService.GetRingFrequencies(nbRings)
					s.renderService.Render(samples)
				},
				true,
			)
		}
	}()
}
