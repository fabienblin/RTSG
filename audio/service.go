package audio

import (
	"encoding/binary"
	"fmt"
	"main/config"
	"math"
	"slices"
	"sync"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

type AudioService struct {
	config           config.Audio
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

func New(config config.Audio) *AudioService {
	return &AudioService{
		config: config,
		stopCh: make(chan struct{}),
	}
}

func (s *AudioService) Run() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
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
		&s.enumerator,
	); err != nil {
		return fmt.Errorf("create device enumerator: %w", err)
	}

	if err := s.enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EMultimedia, &s.device); err != nil {
		return fmt.Errorf("default audio endpoint: %w", err)
	}

	if err := s.device.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, 0, &s.client); err != nil {
		return fmt.Errorf("activate audio client: %w", err)
	}

	if err := s.client.GetMixFormat(&s.format); err != nil {
		return fmt.Errorf("get mix format: %w", err)
	}

	s.sampleRate = int(s.format.NSamplesPerSec)
	s.channels = int(s.format.NChannels)

	if err := s.client.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_LOOPBACK,
		wca.REFERENCE_TIME(1000000),
		0,
		s.format,
		nil,
	); err != nil {
		return fmt.Errorf("initialize loopback client: %w", err)
	}

	if err := s.client.GetService(wca.IID_IAudioCaptureClient, &s.capture); err != nil {
		return fmt.Errorf("get capture service: %w", err)
	}

	if err := s.client.Start(); err != nil {
		return fmt.Errorf("start client: %w", err)
	}

	s.started = true
	return nil
}

func (s *AudioService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	_ = s.client.Stop()

	if s.capture != nil {
		s.capture.Release()
	}
	if s.client != nil {
		s.client.Release()
	}
	if s.device != nil {
		s.device.Release()
	}
	if s.enumerator != nil {
		s.enumerator.Release()
	}

	ole.CoUninitialize()
	s.started = false
	return nil
}

func (s *AudioService) getLastSamples() []float32 {
	samples, _ := s.readPCM()
	if len(samples) > 0 {
		s.setLastSamples(samples)
	}

	return slices.Clone(s.lastSamples)
}

func (s *AudioService) GetRingFrequencies(nbRings int) []float64 {
	samples := s.getLastSamples()
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
		result[k] = float64(math.Sqrt(real*real+imag*imag)/float64(n))
		result[k] *= s.config.SampleMultiplier
	}

	return result
}

func (s *AudioService) setLastSamples(samples []float32) {
	if len(samples) == 0 {
		return
	}

	s.lastSamplesMutex.Lock()
	defer s.lastSamplesMutex.Unlock()

	s.lastSamples = slices.Clone(samples)
}

func (s *AudioService) readPCM() ([]float32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil, fmt.Errorf("audio service not started")
	}

	var packetLength uint32
	if err := s.capture.GetNextPacketSize(&packetLength); err != nil {
		return nil, err
	}

	if packetLength == 0 {
		return nil, nil
	}

	var data *byte
	var frames uint32
	var flags uint32

	if err := s.capture.GetBuffer(&data, &frames, &flags, nil, nil); err != nil {
		return nil, err
	}
	defer s.capture.ReleaseBuffer(frames)

	bytesPerSample := int(s.format.WBitsPerSample / 8)
	totalSamples := int(frames) * s.channels
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

func unsafeByteSlice(ptr *byte, length int) []byte {
	return unsafe.Slice(ptr, length)
}

func (s *AudioService) SinusoidTestSamples(size int) []float64 {
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
