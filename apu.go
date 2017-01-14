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
	apu.Pulse1.SweepUsesOnesComplement = true
	apu.Pulse2.SoundType = squareSoundType
	apu.Triangle.SoundType = triangleSoundType
	apu.DMC.SoundType = dmcSoundType
	apu.Noise.SoundType = noiseSoundType
	apu.Noise.NoiseShiftRegister = 1
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
			apu.Triangle.runTriangleLengthCycle()
		}
		if c == 7456 || c == 14914 {
			apu.runLengthCycle()
			apu.runSweepCycle()
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
			apu.Triangle.runTriangleLengthCycle()
		}
		if c == 7456 || c == 18640 {
			apu.runLengthCycle()
			apu.runSweepCycle()
		}
		if c == 18641 {
			apu.FrameCounter = 0
		}
	}
	apu.FrameCounter++

	if !apu.buffer.full() {

		left, right := 0.0, 0.0

		p1 := apu.Pulse1.getSample()
		p2 := apu.Pulse2.getSample()
		tri := apu.Triangle.getSample()

		dmc := apu.DMC.getSample()
		noise := apu.Noise.getSample()

		pSamples := 95.88 / ((8128 / (p1 + p2)) + 100)
		tdnSamples := 159.79 / ((1 / ((tri / 8227) + (noise / 12241) + (dmc / 22638))) + 100)

		sample := pSamples + tdnSamples

		left, right = sample, sample

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

func (apu *apu) runSweepCycle() {
	apu.Pulse1.runSweepCycle()
	apu.Pulse2.runSweepCycle()
}

func (apu *apu) runLengthCycle() {
	apu.Pulse1.runLengthCycle()
	apu.Pulse2.runLengthCycle()
	apu.Triangle.runLengthCycle()
	apu.Noise.runLengthCycle()
}

func (sound *sound) runFreqCycle() {

	sound.T += sound.Freq * timePerSample

	for sound.T > 1.0 {
		sound.T -= 1.0
		if sound.SoundType == noiseSoundType {
			sound.updateNoiseShiftRegister()
		}
	}
}

func (sound *sound) updateNoiseShiftRegister() {
	var feedback uint16
	if sound.NoiseShortLoopFlag {
		feedback = (sound.NoiseShiftRegister >> 6) & 0x01
	} else {
		feedback = (sound.NoiseShiftRegister >> 1) & 0x01
	}
	feedback ^= sound.NoiseShiftRegister & 0x01
	sound.NoiseShiftRegister >>= 1
	sound.NoiseShiftRegister &^= 0x4000
	sound.NoiseShiftRegister |= feedback << 14
}

func (sound *sound) updateFreq() {
	switch sound.SoundType {
	case dmcSoundType:
	case noiseSoundType:
		sound.Freq = 1789773.0 / float64(sound.NoisePeriod)
	case triangleSoundType:
		sound.Freq = 1789773.0 / (32.0 * float64(sound.PeriodTimer+1))
	case squareSoundType:
		sound.Freq = 1789773.0 / (16.0 * float64(sound.PeriodTimer+1))
		sound.updateSweepTargetPeriod()
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

var triangleSampleTable = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
}

func (sound *sound) getTriangleSample() byte {
	val := triangleSampleTable[int(sound.T*32)]
	return val
}

func (sound *sound) getSample() float64 {
	sample := 0.0
	if sound.On {
		switch sound.SoundType {
		case squareSoundType:
			vol := float64(sound.getCurrentVolume())
			if vol > 0 {
				if sound.LengthCounter > 0 {
					if sound.sweepTargetInRange() { // an out-of-range sweep target mutes even if sweep is disabled
						sound.runFreqCycle()
						if sound.inDutyCycle() {
							sample = vol
						} else {
							sample = 0.0
						}
					}
				}
			}
		case triangleSoundType:
			audible := sound.PeriodTimer >= 2 // not accurate, but eliminates annoying clicks
			if audible && sound.LengthCounter > 0 && sound.TriangleLinearCounter > 0 {
				sound.runFreqCycle()
				sample = float64(sound.getTriangleSample())
			}
		case noiseSoundType:
			vol := float64(sound.getCurrentVolume())
			if vol > 0 {
				if sound.LengthCounter > 0 {
					sound.runFreqCycle()
					if sound.NoiseShiftRegister&0x01 == 0x01 {
						sample = vol
					} else {
						sample = 0.0
					}
				}
			}
		case dmcSoundType:
		}
	}
	return sample
}

type sound struct {
	SoundType uint8

	On bool

	T    float64
	Freq float64

	// square waves only
	DutyCycleSelector       byte
	SweepEnable             bool
	SweepNegate             bool
	SweepReload             bool
	SweepDivider            byte
	SweepCounter            byte
	SweepShift              byte
	SweepTargetPeriod       uint16
	SweepUsesOnesComplement bool // hard-wired to pulse1

	TriangleLinearCounter            byte
	TriangleLinearCounterControlFlag bool
	TriangleLinearCounterReloadValue byte
	TriangleLinearCounterReloadFlag  bool

	DMCSampleLength       uint16
	DMCSampleAddr         uint16
	DMCCurrentValue       byte
	DMCIRQEnabled         bool
	DMCLoopEnabled        bool
	DMCRateSelector       byte
	DMCInterruptRequested bool

	NoiseShortLoopFlag bool
	NoisePeriod        uint16
	NoiseShiftRegister uint16

	UsesConstantVolume bool
	InitialVolume      byte
	VolumeDivider      byte
	VolumeDecayCounter byte
	VolumeRestart      bool

	PeriodTimer uint16

	LengthCounter     byte
	LengthCounterHalt bool
}

var lengthCounterTable = []byte{
	10, 254,
	20, 2,
	40, 4,
	80, 6,
	160, 8,
	60, 10,
	14, 12,
	26, 14,
	12, 16,
	24, 18,
	48, 20,
	96, 22,
	192, 24,
	72, 26,
	16, 28,
	32, 30,
}

func (sound *sound) loadLengthCounter(regVal byte) {
	if sound.On {
		sound.LengthCounter = lengthCounterTable[regVal] + 1
	}
}

var noisePeriodTable = []uint16{
	4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 280, 508, 762, 1016, 2034, 4068,
}

func (sound *sound) loadNoisePeriod(regVal byte) {
	if sound.On {
		sound.NoisePeriod = noisePeriodTable[regVal]
	}
}

func (sound *sound) getCurrentVolume() byte {
	if sound.UsesConstantVolume {
		return sound.InitialVolume
	}
	return sound.VolumeDecayCounter
}

func (sound *sound) runLengthCycle() {
	if !sound.LengthCounterHalt {
		if sound.LengthCounter > 0 {
			sound.LengthCounter--
		}
	}
}

func (sound *sound) runSweepCycle() {
	if sound.SweepReload {
		sound.SweepCounter = sound.SweepDivider
		sound.SweepReload = false
	}
	if sound.SweepCounter > 0 {
		sound.SweepCounter--
	} else {
		sound.SweepCounter = sound.SweepDivider
		if sound.SweepEnable && sound.sweepTargetInRange() {
			sound.PeriodTimer = sound.SweepTargetPeriod
			sound.updateFreq()
		}
	}
}

func (sound *sound) sweepTargetInRange() bool {
	return sound.SweepTargetPeriod <= 0x7ff && sound.SweepTargetPeriod >= 8
}

func (sound *sound) updateSweepTargetPeriod() {
	periodDelta := int(sound.PeriodTimer >> sound.SweepShift)
	if sound.SweepNegate {
		if sound.SweepUsesOnesComplement {
			periodDelta = -periodDelta - 1
		} else {
			periodDelta = -periodDelta
		}
	}
	sound.SweepTargetPeriod = uint16(int(sound.PeriodTimer) + periodDelta)
}

func (sound *sound) runTriangleLengthCycle() {
	if sound.TriangleLinearCounterReloadFlag {
		sound.TriangleLinearCounter = sound.TriangleLinearCounterReloadValue
	} else if sound.TriangleLinearCounter > 0 {
		sound.TriangleLinearCounter--
	}
	if !sound.TriangleLinearCounterControlFlag {
		sound.TriangleLinearCounterReloadFlag = false
	}
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
	} else {
		sound.VolumeDivider--
	}
}

func (sound *sound) writeLinearCounterReg(val byte) {
	sound.TriangleLinearCounterControlFlag = val&0x80 == 0x80
	sound.LengthCounterHalt = sound.TriangleLinearCounterControlFlag
	sound.TriangleLinearCounterReloadValue = val & 0x7f
}

func (sound *sound) writePeriodLowReg(val byte) {
	sound.PeriodTimer &^= 0x00ff
	sound.PeriodTimer |= uint16(val)
	sound.updateFreq()
}

func (sound *sound) writePeriodHighTimerReg(val byte) {
	sound.PeriodTimer &^= 0x0700
	sound.PeriodTimer |= (uint16(val) & 0x07) << 8
	sound.loadLengthCounter(val >> 3)
	sound.VolumeRestart = true
	sound.TriangleLinearCounterReloadFlag = true
	// if sound.SoundType == squareSoundType {
	// inaccurate, but do this for triangle too as it removes a lot of clicks...
	sound.T = 0 // nesdev wiki: "sequencer is restarted"
	// }
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
	apu.FrameCounterInterruptInhibit = val&0x40 == 0x40
	apu.FrameCounterSequencerMode = val >> 7
	// NOTE: frame counter should be reset here, but until
	// we have more accurate timing, it makes sweeps sound
	// weird on games (e.g. smbros) that write to this reg
	// for sync
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
	sound.loadLengthCounter(val >> 3)
}

func (sound *sound) writeNoiseControlReg(val byte) {
	sound.NoiseShortLoopFlag = val&0x80 == 0x80
	sound.loadNoisePeriod(val & 0x0f)
	sound.updateFreq()
}

func (sound *sound) setChannelOn(val bool) {
	sound.On = val
	if !sound.On {
		sound.LengthCounter = 0
	}
}

func (apu *apu) writeStatusReg(val byte) {
	apu.DMC.setChannelOn(val&0x10 == 0x10)
	apu.Noise.setChannelOn(val&0x08 == 0x08)
	apu.Triangle.setChannelOn(val&0x04 == 0x04)
	apu.Pulse2.setChannelOn(val&0x02 == 0x02)
	apu.Pulse1.setChannelOn(val&0x01 == 0x01)

	apu.DMC.DMCInterruptRequested = false
}
func (apu *apu) readStatusReg() byte {
	result := byteFromBools(
		apu.DMC.DMCInterruptRequested,
		apu.FrameInterruptRequested,
		true,
		apu.DMC.DMCSampleLength > 0, // FIXME: when dmc is implemented this should be bytesRemaining > 0
		apu.Noise.LengthCounter > 0,
		apu.Triangle.LengthCounter > 0,
		apu.Pulse2.LengthCounter > 0,
		apu.Pulse1.LengthCounter > 0,
	)
	apu.FrameInterruptRequested = false
	return result
}
