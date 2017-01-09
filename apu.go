package famigo

type apu struct {
	FrameCounterInterruptInhibit bool
	FrameCounterSequencerMode    byte
	FrameInterruptRequested      bool
	FrameCounter                 uint64

	buffer apuCircleBuf

	Pulse1   sound
	Pulse2   sound
	Triangle sound
	DMC      sound
	Noise    sound
}

const (
	squareSoundType   = 0
	triangleSoundType = 1
	dmcSoundType      = 2
	noiseSoundType    = 3
)

func (apu *apu) init() {
	apu.Pulse1.SoundType = squareSoundType
	apu.Pulse2.SoundType = squareSoundType
	apu.Triangle.SoundType = triangleSoundType
	apu.DMC.SoundType = dmcSoundType
	apu.Noise.SoundType = noiseSoundType
}

const (
	amountGenerateAhead = 4
	samplesPerSecond    = 44100
	timePerSample       = 1.0 / samplesPerSecond
)

const apuCircleBufSize = amountGenerateAhead

// NOTE: size must be power of 2
type apuCircleBuf struct {
	writeIndex uint
	readIndex  uint
	buf        [apuCircleBufSize]byte
}

func (c *apuCircleBuf) write(bytes []byte) (writeCount int) {
	for _, b := range bytes {
		if c.full() {
			return writeCount
		}
		c.buf[c.mask(c.writeIndex)] = b
		c.writeIndex++
		writeCount++
	}
	return writeCount
}
func (c *apuCircleBuf) read(preSizedBuf []byte) []byte {
	readCount := 0
	for i := range preSizedBuf {
		if c.size() == 0 {
			break
		}
		preSizedBuf[i] = c.buf[c.mask(c.readIndex)]
		c.readIndex++
		readCount++
	}
	return preSizedBuf[:readCount]
}
func (c *apuCircleBuf) mask(i uint) uint { return i & (uint(len(c.buf)) - 1) }
func (c *apuCircleBuf) size() uint       { return c.writeIndex - c.readIndex }
func (c *apuCircleBuf) full() bool       { return c.size() == uint(len(c.buf)) }

func (apu *apu) runCycle(cs *cpuState) {

	if apu.FrameCounterSequencerMode == 0 {
		c := apu.FrameCounter
		if c == 3728 || c == 7456 || c == 11185 || c == 14914 {
			apu.runEnvCycle()
		}
		if c == 14914 {
			if !apu.FrameCounterInterruptInhibit {
				apu.FrameInterruptRequested = true
				cs.IRQ = true
			}
		}
		if c == 14915 {
			apu.FrameCounter = 0
		}
	} else {
		c := apu.FrameCounter
		if c == 3728 || c == 7456 || c == 11185 || c == 18640 {
			apu.runEnvCycle()
		}
		if c == 18641 {
			apu.FrameCounter = 0
		}
	}
	apu.FrameCounter++

	if !apu.buffer.full() {

		left, right := 0.0, 0.0

		left0, right0 := apu.Pulse1.getSample()
		left1, right1 := apu.Pulse2.getSample()
		// left2, right2 := apu.Triangle.getSample()
		// left3, right3 := apu.DMC.getSample()
		// left4, right4 := apu.Noise.getSample()
		// left = (left0 + left1 + left2 + left3 + left4) * 0.2
		// right = (right0 + right1 + right2 + right3 + right4) * 0.2

		left = (left0 + left1) * 0.5
		right = (right0 + right1) * 0.5

		left = left*2.0 - 1.0
		right = right*2.0 - 1.0

		sampleL, sampleR := int16(left*32767.0), int16(right*32767.0)
		apu.buffer.write([]byte{
			byte(sampleL & 0xff),
			byte(sampleL >> 8),
			byte(sampleR & 0xff),
			byte(sampleR >> 8),
		})
	}
}

func (apu *apu) runFreqCycle() {
	apu.Pulse1.runFreqCycle()
	apu.Pulse2.runFreqCycle()
	apu.Triangle.runFreqCycle()
	apu.DMC.runFreqCycle()
	apu.Noise.runFreqCycle()
}

func (apu *apu) runEnvCycle() {
	apu.Pulse1.runEnvCycle()
	apu.Pulse2.runEnvCycle()
	apu.Noise.runEnvCycle()
}

func (sound *sound) runFreqCycle() {

	sound.T += sound.Freq * timePerSample

	for sound.T > 1.0 {
		sound.T -= 1.0
	}
}

func (sound *sound) updateFreq() {
	switch sound.SoundType {
	case dmcSoundType:
	case noiseSoundType:
	case triangleSoundType:
	case squareSoundType:
		sound.Freq = 1789773.0 / (16.0 * float64(sound.PeriodTimer+1))
	default:
		panic("unexpected sound type")
	}
}

func (sound *sound) inDutyCycle() bool {
	switch sound.DutyCycleSelector {
	case 0:
		return sound.T > 0.125 && sound.T < 0.250
	case 1:
		return sound.T > 0.125 && sound.T < 0.375
	case 2:
		return sound.T > 0.125 && sound.T < 0.625
	case 3:
		return sound.T < 0.125 || sound.T > 0.375
	default:
		panic("unknown wave duty")
	}
}

func (sound *sound) getSample() (float64, float64) {
	sample := 0.0
	if sound.On {
		sound.runFreqCycle()
		switch sound.SoundType {
		case squareSoundType:
			vol := float64(sound.getCurrentVolume()) / 15.0
			if sound.PeriodTimer >= 8 && vol > 0 {
				if sound.inDutyCycle() {
					sample = vol
				} else {
					sample = 0.0
				}
			}
		case triangleSoundType:
		case dmcSoundType:
		case noiseSoundType:
		}
	}

	left, right := sample, sample
	return left, right
}

type sound struct {
	SoundType uint8

	On bool

	T    float64
	Freq float64

	// square waves only
	DutyCycleSelector byte
	SweepEnable       bool
	SweepNegate       bool
	SweepReload       bool
	SweepDivider      byte
	SweepShift        byte

	TriangleLinearCounter            byte
	TriangleLinearCounterControlFlag bool

	DMCSampleLength       uint16
	DMCSampleAddr         uint16
	DMCCurrentValue       byte
	DMCIRQEnabled         bool
	DMCLoopEnabled        bool
	DMCRateSelector       byte
	DMCInterruptRequested bool

	NoiseLoopEnabled bool
	NoisePeriod      byte

	UsesConstantVolume bool
	InitialVolume      byte
	VolumeDivider      byte
	VolumeDecayCounter byte
	VolumeRestart      bool

	PeriodTimer uint16

	LengthCounter     byte
	LengthCounterHalt bool
}

func (sound *sound) getCurrentVolume() byte {
	if sound.UsesConstantVolume {
		return sound.InitialVolume
	}
	return sound.VolumeDecayCounter
}

func (sound *sound) runEnvCycle() {
	if sound.VolumeRestart {
		sound.VolumeRestart = false
		sound.VolumeDecayCounter = 0x0f
		sound.VolumeDivider = 0
	}
	if sound.VolumeDivider == 0 {
		sound.VolumeDivider = sound.InitialVolume
		if sound.VolumeDecayCounter == 0 {
			// length counter halt also == env loop
			if sound.LengthCounterHalt {
				sound.VolumeDecayCounter = 0x0f
			}
		} else {
			sound.VolumeDecayCounter--
		}
	}
	sound.VolumeDivider--
}

func (sound *sound) writeLinearCounterReg(val byte) {
	sound.TriangleLinearCounterControlFlag = val&0x80 == 0x80
	sound.TriangleLinearCounter = val & 0x7f
}

func (sound *sound) writePeriodLowReg(val byte) {
	sound.PeriodTimer &^= 0x00ff
	sound.PeriodTimer |= uint16(val)
	sound.updateFreq()
}

func (sound *sound) writePeriodHighTimerReg(val byte) {
	sound.PeriodTimer &^= 0x0700
	sound.PeriodTimer |= (uint16(val) & 0x07) << 8
	sound.LengthCounter = val >> 3
	sound.VolumeRestart = true
	sound.T = 0 // nesdev wiki: "sequencer is restarted"
	sound.updateFreq()
}

// square waves only
func (sound *sound) writeVolDutyReg(val byte) {
	sound.DutyCycleSelector = val >> 6
	sound.LengthCounterHalt = val&0x20 == 0x20
	sound.UsesConstantVolume = val&0x10 == 0x10
	sound.InitialVolume = val & 0x0f
}

func (sound *sound) writeSweepReg(val byte) {
	sound.SweepEnable = val&0x80 == 0x80
	sound.SweepDivider = ((val >> 4) & 0x07) + 1
	sound.SweepNegate = val&0x08 == 0x08
	sound.SweepShift = val & 0x07
	sound.SweepReload = true
}

func (apu *apu) writeFrameCounterReg(val byte) {
	apu.FrameCounterInterruptInhibit = val&0x40 != 0
	apu.FrameCounterSequencerMode = val >> 7
}

func (sound *sound) writeDMCSampleLength(val byte) {
	sound.DMCSampleLength = (uint16(val) << 4) + 1
}

func (sound *sound) writeDMCSampleAddr(val byte) {
	sound.DMCSampleAddr = 0xc000 | (uint16(val) << 6)
}

func (sound *sound) writeDMCCurrentValue(val byte) {
	sound.DMCCurrentValue = 0x7f & val
}

func (sound *sound) writeDMCFlagsAndRate(val byte) {
	sound.DMCIRQEnabled = val&0x80 == 0x80
	sound.DMCLoopEnabled = val&0x40 == 0x40
	sound.DMCRateSelector = val & 0x0f
}

func (sound *sound) writeNoiseLength(val byte) {
	sound.LengthCounter = val >> 3
}

func (sound *sound) writeNoiseControlReg(val byte) {
	sound.NoiseLoopEnabled = val&0x80 == 0x80
	sound.NoisePeriod = val & 0x0f
}

func (apu *apu) writeStatusReg(val byte) {
	boolsFromByte(val,
		nil, nil, nil,
		&apu.DMC.On,
		&apu.Noise.On,
		&apu.Triangle.On,
		&apu.Pulse2.On,
		&apu.Pulse1.On,
	)
	apu.DMC.DMCInterruptRequested = false
}
func (apu *apu) readStatusReg() byte {
	result := byteFromBools(
		apu.DMC.DMCInterruptRequested,
		apu.FrameInterruptRequested,
		true,
		apu.DMC.DMCSampleLength > 0, // NOTE: make sure this means "DMC active"
		apu.Noise.LengthCounter > 0,
		apu.Triangle.LengthCounter > 0,
		apu.Pulse2.LengthCounter > 0,
		apu.Pulse1.LengthCounter > 0,
	)
	apu.FrameInterruptRequested = false
	return result
}
