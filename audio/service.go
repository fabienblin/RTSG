package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

type AudioService struct {
	mu               sync.Mutex
	enumerator       *wca.IMMDeviceEnumerator
	device           *wca.IMMDevice
	client           *wca.IAudioClient
	capture          *wca.IAudioCaptureClient
	format           *wca.WAVEFORMATEX
	lastSamplesMutex sync.Mutex
	lastSamples      []float32
	sampleRate       int
	channels         int
	started          bool
	stopCh           chan struct{}
}

func New() *AudioService {
	return &AudioService{
		stopCh: make(chan struct{}),
	}
}
func (a *AudioService) Run() {}

func (a *AudioService) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.started {
		return nil
	}

	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return fmt.Errorf("coinitialize: %w", err)
	}

	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&a.enumerator,
	); err != nil {
		return fmt.Errorf("create device enumerator: %w", err)
	}

	if err := a.enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EMultimedia, &a.device); err != nil {
		return fmt.Errorf("default audio endpoint: %w", err)
	}

	if err := a.device.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, 0, &a.client); err != nil {
		return fmt.Errorf("activate audio client: %w", err)
	}

	if err := a.client.GetMixFormat(&a.format); err != nil {
		return fmt.Errorf("get mix format: %w", err)
	}

	a.sampleRate = int(a.format.NSamplesPerSec)
	a.channels = int(a.format.NChannels)

	const bufferDuration = time.Millisecond * 100
	hns := wca.REFERENCE_TIME(bufferDuration.Nanoseconds() / 100) // convert to 100ns units

	if err := a.client.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_LOOPBACK,
		hns,
		0,
		a.format,
		nil,
	); err != nil {
		return fmt.Errorf("initialize loopback client: %w", err)
	}

	if err := a.client.GetService(wca.IID_IAudioCaptureClient, &a.capture); err != nil {
		return fmt.Errorf("get capture service: %w", err)
	}

	if err := a.client.Start(); err != nil {
		return fmt.Errorf("start client: %w", err)
	}

	a.started = true
	return nil
}

func (a *AudioService) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.started {
		return nil
	}

	_ = a.client.Stop()

	if a.capture != nil {
		a.capture.Release()
	}
	if a.client != nil {
		a.client.Release()
	}
	if a.device != nil {
		a.device.Release()
	}
	if a.enumerator != nil {
		a.enumerator.Release()
	}

	ole.CoUninitialize()
	a.started = false
	return nil
}

func (a *AudioService) GetLastSamples() []float32 {
	samples, _ := a.readPCM()
	if len(samples) > 0 {
		a.setLastSamples(samples)
	}

	return slices.Clone(a.lastSamples)
}

func (a *AudioService) GetRingFrequencies(nbRings int) []float64 {
	const sampleMultiplier = 100.0
	samples := a.GetLastSamples()
	n := len(samples)
	if n == 0 || nbRings <= 0 {
		return nil
	}

	if nbRings > n/2 {
		nbRings = n / 2 // Nyquist limit
	}

	result := make([]float64, nbRings)

	for k := 0; k < nbRings; k++ {
		var real, imag float64
		for t := 0; t < n; t++ {
			angle := 2.0 * math.Pi * float64(k) * float64(t) / float64(n)
			real += float64(samples[t]) * math.Cos(angle)
			imag -= float64(samples[t]) * math.Sin(angle)
		}
		result[k] = float64(math.Sqrt(real*real+imag*imag) / float64(n)) * sampleMultiplier
	}

	return result
}

func (a *AudioService) setLastSamples(samples []float32) {
	if len(samples) == 0 {
		return
	}

	a.lastSamplesMutex.Lock()
	defer a.lastSamplesMutex.Unlock()

	a.lastSamples = slices.Clone(samples)
}

func (a *AudioService) readPCM() ([]float32, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.started {
		return nil, fmt.Errorf("audio service not started")
	}

	var packetLength uint32
	if err := a.capture.GetNextPacketSize(&packetLength); err != nil {
		return nil, err
	}

	if packetLength == 0 {
		return nil, nil
	}

	var data *byte
	var frames uint32
	var flags uint32

	if err := a.capture.GetBuffer(&data, &frames, &flags, nil, nil); err != nil {
		return nil, err
	}
	defer a.capture.ReleaseBuffer(frames)

	bytesPerSample := int(a.format.WBitsPerSample / 8)
	totalSamples := int(frames) * a.channels
	rawSize := totalSamples * bytesPerSample

	raw := unsafeByteSlice(data, rawSize)
	out := make([]float32, totalSamples)

	switch bytesPerSample {
	case 2:
		for i := 0; i < totalSamples; i++ {
			v := int16(binary.LittleEndian.Uint16(raw[i*2:]))
			out[i] = float32(v) / math.MaxInt16
		}
	case 4:
		for i := 0; i < totalSamples; i++ {
			bits := binary.LittleEndian.Uint32(raw[i*4:])
			out[i] = math.Float32frombits(bits)
		}
	default:
		return nil, fmt.Errorf("unsupported sample width: %d", bytesPerSample)
	}

	return out, nil
}

// unsafeByteSlice converts COM buffer memory into a Go byte slice.
// Replace with a safer helper if you already use unsafe utilities in your codebase.
func unsafeByteSlice(ptr *byte, length int) []byte {
	return unsafe.Slice(ptr, length)
}

func (a *AudioService) SinusoidTestSamples(size int) []float64 {
	const (
		speed float64 = math.Pi / 100
	)
	samples := make([]float64, size)

	sinRange := math.Pi * 2
	sinOffset := float64(time.Now().UnixMilli()) * speed
	increment := sinRange / float64(size)

	for i := range size {
		discretePoint := (math.Sin(sinOffset + increment*float64(i)))
		discretePoint = (discretePoint + 1) / 2 // normalize sin value
		samples[i] = discretePoint
	}

	return samples
}
