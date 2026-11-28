//go:build esp32p4

package machine

// I2S implements the shared machine.I2S interface on the ESP32-P4.
// The I2STx and I2SRx types remain available for P4-specific use.
type I2S struct {
	ctrl    I2SController
	config  I2SConfig
	tx      *I2STx
	rx      *I2SRx
	running bool
}

var (
	// I2S0 is the general purpose I2S controller used by the Nano ES8311.
	I2S0 = I2S{ctrl: I2SController0}
	// I2S1 is the general purpose I2S controller used by the CrowPanel.
	I2S1 = I2S{ctrl: I2SController1}
)

// Configure sets up the selected controller. Only 16-bit Philips I2S in
// internal-clock mode is supported; duplex and PDM return errI2SUnsupported.
func (i2s *I2S) Configure(config I2SConfig) error {
	i2s.close()

	if config.DataFormat == I2SDataFormatDefault {
		config.DataFormat = I2SDataFormat16bit
	}
	if config.DataFormat != I2SDataFormat16bit ||
		config.Standard != I2StandardPhilips ||
		config.ClockSource != I2SClockSourceInternal ||
		config.Mode == I2SModePDM ||
		config.Mode == I2SModeSourceReceiver {
		return errI2SUnsupported
	}
	if config.AudioFrequency == 0 {
		config.AudioFrequency = 44100
	}

	tx := config.Mode == I2SModeSource
	rx := config.Mode == I2SModeReceiver
	if !tx && !rx {
		return errI2SUnsupported
	}
	if config.SCK == NoPin || config.WS == NoPin ||
		(tx && config.SDO == NoPin) || (rx && config.SDI == NoPin) {
		return errI2SPins
	}

	mclk := NoPin
	mclkFrequency := uint32(0)
	if config.MainClockOutput {
		if config.MCLK == NoPin {
			return errI2SPins
		}
		mclk = config.MCLK
		mclkFrequency = config.MCLKFrequency
		if mclkFrequency == 0 {
			mclkFrequency = config.AudioFrequency * 256
		}
	}

	i2s.config = config
	i2s.running = true

	if tx {
		t, err := NewI2STx(I2STxConfig{
			Controller:    i2s.ctrl,
			SCK:           config.SCK,
			WS:            config.WS,
			SDO:           config.SDO,
			MCLK:          mclk,
			MCLKFrequency: mclkFrequency,
			SampleRate:    config.AudioFrequency,
			BitsPerSample: 16,
			Mono:          !config.Stereo,
		})
		if err != nil {
			i2s.running = false
			return err
		}
		i2s.tx = t
		return nil
	}

	r, err := NewI2SRx(I2SRxConfig{
		Controller:    i2s.ctrl,
		SCK:           config.SCK,
		WS:            config.WS,
		SDI:           config.SDI,
		MCLK:          mclk,
		MCLKFrequency: mclkFrequency,
		SampleRate:    config.AudioFrequency,
		BitsPerSample: 16,
		Mono:          !config.Stereo,
	})
	if err != nil {
		i2s.running = false
		return err
	}
	i2s.rx = r
	return nil
}

// SetSampleFrequency reconfigures the controller at the new sample rate.
func (i2s *I2S) SetSampleFrequency(freq uint32) error {
	if freq == 0 {
		return ErrInvalidSampleFrequency
	}
	if i2s.tx == nil && i2s.rx == nil {
		return errI2SNotRunning
	}
	wasRunning := i2s.running
	config := i2s.config
	config.AudioFrequency = freq
	if err := i2s.Configure(config); err != nil {
		return err
	}
	i2s.running = wasRunning
	return nil
}

// Enable stops or re-arms the interface. TX and RX start on the next
// Write or Read call.
func (i2s *I2S) Enable(enabled bool) {
	i2s.running = enabled
	if enabled {
		return
	}
	if i2s.tx != nil {
		i2s.tx.hw.SetTX_CONF_TX_START(0)
		i2sTxUpdate(i2s.tx.hw)
		i2s.tx.running = false
	}
	if i2s.rx != nil {
		i2s.rx.hw.SetRX_CONF_RX_START(0)
		i2sRxUpdate(i2s.rx.hw)
	}
}

// WriteMono writes 16-bit mono samples. The controller must be configured
// with Stereo set to false.
func (i2s *I2S) WriteMono(b []uint16) (int, error) {
	if i2s.tx == nil || i2s.config.Stereo {
		return 0, errI2SUnsupported
	}
	if !i2s.running {
		return 0, errI2SNotRunning
	}
	if len(b) == 0 {
		return 0, nil
	}
	samples := make([]int16, len(b))
	for i, v := range b {
		samples[i] = int16(v)
	}
	if err := i2s.tx.Write(samples); err != nil {
		return 0, err
	}
	return len(b), nil
}

// WriteStereo writes 16-bit stereo frames, with left in the low half of each
// uint32. The controller must be configured with Stereo set to true.
func (i2s *I2S) WriteStereo(b []uint32) (int, error) {
	if i2s.tx == nil || !i2s.config.Stereo {
		return 0, errI2SUnsupported
	}
	if !i2s.running {
		return 0, errI2SNotRunning
	}
	if len(b) == 0 {
		return 0, nil
	}
	samples := make([]int16, len(b)*2)
	for i, v := range b {
		samples[i*2] = int16(v)
		samples[i*2+1] = int16(v >> 16)
	}
	if err := i2s.tx.Write(samples); err != nil {
		return 0, err
	}
	return len(b), nil
}

// ReadMono reads 16-bit mono samples. The controller must be configured with
// Stereo set to false.
func (i2s *I2S) ReadMono(b []uint16) (int, error) {
	if i2s.rx == nil || i2s.config.Stereo {
		return 0, errI2SUnsupported
	}
	if !i2s.running {
		return 0, errI2SNotRunning
	}
	const maxSamples = maxI2SDescBytes / 2
	for done := 0; done < len(b); {
		n := min(len(b)-done, maxSamples)
		samples := make([]int16, n)
		if err := i2s.rx.Read(samples); err != nil {
			return done, err
		}
		for i, v := range samples {
			b[done+i] = uint16(v)
		}
		done += n
	}
	return len(b), nil
}

// ReadStereo reads 16-bit stereo frames into packed uint32 values.
func (i2s *I2S) ReadStereo(b []uint32) (int, error) {
	if i2s.rx == nil || !i2s.config.Stereo {
		return 0, errI2SUnsupported
	}
	if !i2s.running {
		return 0, errI2SNotRunning
	}
	const maxFrames = maxI2SDescBytes / 4
	for done := 0; done < len(b); {
		n := min(len(b)-done, maxFrames)
		samples := make([]int16, n*2)
		if err := i2s.rx.Read(samples); err != nil {
			return done, err
		}
		for i := 0; i < n; i++ {
			b[done+i] = uint32(uint16(samples[i*2])) | uint32(uint16(samples[i*2+1]))<<16
		}
		done += n
	}
	return len(b), nil
}

func (i2s *I2S) close() {
	if i2s.tx != nil {
		i2s.tx.Close()
		i2s.tx = nil
	}
	if i2s.rx != nil {
		i2s.rx.Close()
		i2s.rx = nil
	}
	i2s.running = false
}
