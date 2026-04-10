package render

import (
	"image/color"
	"main/config"
	"math"
	"math/rand/v2"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

const (
	ringSpacing     float64 = 25.0
	ringsDotSpacing float64 = ringSpacing
	dotRadius       float64 = 5
)

type RenderService struct {
	config   config.Renderer
	dotRings [][]*Dot
}

type Dot struct {
	Circle  *canvas.Circle
	CenterX float64
	CenterY float64
}

func New(config config.Renderer) *RenderService {
	service := &RenderService{
		config: config,
	}
	service.init()

	return service
}

func (s *RenderService) init() {
	nbRings := countRings(float64(s.config.Size.Width), float64(s.config.Size.Height))
	s.dotRings = make([][]*Dot, nbRings)

	for ringIndex := range nbRings {
		circumference := ringCircumference(ringIndex)
		nbDotsOnRing := countDotsPerRing(circumference)
		s.dotRings[ringIndex] = make([]*Dot, nbDotsOnRing)
		ringColor := color.Black

		for dotIndexOnRing := range nbDotsOnRing {
			x, y := dotCenterPosition(
				nbDotsOnRing,
				dotIndexOnRing,
				ringIndex,
				float64(s.config.Size.Width),
				float64(s.config.Size.Height),
			)

			dot := &Dot{
				Circle:  canvas.NewCircle(ringColor),
				CenterX: x,
				CenterY: y,
			}

			dotDiameter := float32(dotRadius * 2)
			dot.Circle.Resize(fyne.NewSize(dotDiameter, dotDiameter))
			dot.Circle.Move(fyne.NewPos(float32(x-dotRadius), float32(y-dotRadius)))

			s.dotRings[ringIndex][dotIndexOnRing] = dot
		}
	}
}

func (s *RenderService) Render(samples []float64) {
	if len(samples) < len(s.dotRings) {
		return
	}

	for ringIdx, ring := range s.dotRings {
		if len(ring) == 0 {
			continue
		}

		currentDiameter := float64(ring[0].Circle.Size().Width)
		targetDiameter := 2 * dotRadius * samples[ringIdx]

		lerpedDiameter := currentDiameter + (targetDiameter - currentDiameter) * s.config.Lerp

		newRadius := lerpedDiameter / 2

		for _, dot := range ring {
			dot.Circle.Resize(fyne.NewSize(
				float32(lerpedDiameter),
				float32(lerpedDiameter),
			))

			dot.Circle.Move(fyne.NewPos(
				float32(dot.CenterX-newRadius),
				float32(dot.CenterY-newRadius),
			))
		}
	}
}

func (s *RenderService) Objects() []fyne.CanvasObject {
	list := []fyne.CanvasObject{}

	for i := range s.dotRings {
		for j := range s.dotRings[i] {
			list = append(list, s.dotRings[i][j].Circle)
		}
	}

	return list
}

func randomColor() color.Color {
	r := uint8(rand.UintN(255))
	g := uint8(rand.UintN(255))
	b := uint8(rand.UintN(255))
	return color.RGBA{r, g, b, 255}
}

func countRings(width, height float64) int {
	areaRadius := math.Min(width, height) / 2
	return int((areaRadius) / ringSpacing)
}

func (s *RenderService) NbRings() int {
	return len(s.dotRings)
}

func countDotsPerRing(circumference float64) int {
	dotsSeparation := dotRadius*2 + ringsDotSpacing
	if dotsSeparation <= 0 || circumference <= 0 {
		return 1
	}

	return int(circumference / dotsSeparation)
}

func ringCircumference(ringIndex int) float64 {
	return 2 * math.Pi * ringDistanceToCenter(ringIndex)
}

func angleOnRing(nbDotsOnRing int, dotIndexOnRing int) float64 {
	if nbDotsOnRing == 0 {
		return 0
	}

	angleIncrement := float64(2*math.Pi) / float64(nbDotsOnRing)
	return angleIncrement * float64(dotIndexOnRing)
}

func ringDistanceToCenter(ringIndex int) float64 {
	return float64(ringIndex) * ringSpacing
}

func dotCenterPosition(nbDotsOnRing, dotIndexOnRing, ringIndex int, width, height float64) (float64, float64) {
	centerX := width / 2
	centerY := height / 2
	angle := float64(angleOnRing(nbDotsOnRing, dotIndexOnRing))
	distance := float64(ringDistanceToCenter(ringIndex))

	x := (centerX + distance*math.Cos(angle))
	y := (centerY + distance*math.Sin(angle))

	return x, y
}
