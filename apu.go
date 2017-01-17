package famigo

type apu struct {
	FrameCounterInterruptInhibit   bool
	FrameCounterSequencerMode      byte
	FrameCounterInterruptRequested bool
	FrameCounterManualTrigger      bool
	FrameCounter                   uint64

	lastSample            float64
	lastCorrectedSample   float64
	lastCorrectedTriangle float64

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

	apu.Noise.NoisePeriod = noisePeriodTable[0]
	apu.DMC.DMCPeriod = dmcPeriodTable[0]
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

func (apu *apu) runFrameCounterCycle() {
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
				apu.FrameCounterInterruptRequested = true
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
	if apu.FrameCounterManualTrigger {
		apu.FrameCounterManualTrigger = false

		apu.runEnvCycle()
		apu.Triangle.runTriangleLengthCycle()
		apu.runLengthCycle()
		apu.runSweepCycle()
	}
	apu.FrameCounter++
}

func (sound *sound) updateDMCOutput(cs *cpuState) {
	if sound.DMCRestartFlag {
		sound.DMCRestartFlag = false
		sound.DMCCurrentSampleAddr = sound.DMCInitialSampleAddr
		sound.DMCSampleBytesRemaining = sound.DMCSampleLength
	}

	if sound.DMCSampleBitsRemaining > 0 {
		if !sound.DMCSilenceFlag {
			if sound.DMCCurrentSampleByte&0x01 == 1 {
				if sound.DMCCurrentValue <= 125 {
					sound.DMCCurrentValue += 2
				}
			} else {
				if sound.DMCCurrentValue >= 2 {
					sound.DMCCurrentValue -= 2
				}
			}
		}
		sound.DMCCurrentSampleByte >>= 1
		sound.DMCSampleBitsRemaining--
	} else {
		sound.DMCSampleBitsRemaining = 8
		if sound.DMCSampleBytesRemaining > 0 {
			sound.DMCSilenceFlag = false
			sound.DMCCurrentSampleByte = cs.read(sound.DMCCurrentSampleAddr)
			sound.DMCCurrentSampleAddr = (sound.DMCCurrentSampleAddr + 1) | 0x8000
			sound.DMCSampleBytesRemaining--
			if sound.DMCSampleBytesRemaining == 0 {
				if sound.DMCLoopEnabled {
					sound.DMCRestartFlag = true
				}
				if sound.DMCIRQEnabled {
					sound.DMCInterruptRequested = true
				}
			}
		} else {
			sound.DMCSilenceFlag = true
		}
	}
}

func (apu *apu) runCycle(cs *cpuState) {

	apu.runFrameCounterCycle()
	if apu.FrameCounterInterruptRequested {
		cs.IRQ = true
	}

	if !apu.buffer.full() {

		left, right := 0.0, 0.0

		apu.runFreqCycle(cs)

		p1 := apu.Pulse1.getSample()
		p2 := apu.Pulse2.getSample()
		tri := apu.Triangle.getSample()
		dmc := apu.DMC.getSample()
		noise := apu.Noise.getSample()

		apu.lastCorrectedTriangle = apu.lastCorrectedTriangle + 0.07*(float64(tri)-apu.lastCorrectedTriangle)
		tri = byte(apu.lastCorrectedTriangle)

		pSamples := 95.88 / (8128/(float64(p1)+float64(p2)) + 100)
		tdnSamples := 159.79 / (1/(float64(tri)/8227+float64(noise)/12241+float64(dmc)/22638) + 100)
		sample := pSamples + tdnSamples

		// dc blocker to center waveform
		correctedSample := sample - apu.lastSample + 0.995*apu.lastCorrectedSample
		apu.lastCorrectedSample = correctedSample
		apu.lastSample = sample
		sample = correctedSample

		left, right = sample, sample

		sampleL, sampleR := int16(left*32767.0), int16(right*32767.0)
		apu.buffer.write([]byte{
			byte(sampleL & 0xff),
			byte(sampleL >> 8),
			byte(sampleR & 0xff),
			byte(sampleR >> 8),
		})
	}

	if apu.DMC.DMCInterruptRequested {
		cs.IRQ = true
	}
}

func (apu *apu) runFreqCycle(cs *cpuState) {
	apu.Pulse1.runFreqCycle(cs)
	apu.Pulse2.runFreqCycle(cs)
	apu.Triangle.runFreqCycle(cs)
	apu.DMC.runFreqCycle(cs)
	apu.Noise.runFreqCycle(cs)
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

func (sound *sound) runFreqCycle(cs *cpuState) {

	sound.T += sound.Freq * timePerSample

	for sound.T > 1.0 {
		sound.T -= 1.0
		if sound.SoundType == noiseSoundType {
			sound.updateNoiseShiftRegister()
		} else if sound.SoundType == dmcSoundType {
			sound.updateDMCOutput(cs)
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
		sound.Freq = 1789773.0 / float64(sound.DMCPeriod)
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

func (sound *sound) getSample() byte {
	sample := byte(0)
	switch sound.SoundType {
	case squareSoundType:
		vol := sound.getCurrentVolume()
		if sound.On && vol > 0 {
			if sound.LengthCounter > 0 {
				if sound.sweepTargetInRange() { // an out-of-range sweep target mutes even if sweep is disabled
					if sound.inDutyCycle() {
						sample = vol
					}
				}
			}
		}
	case triangleSoundType:
		audible := sound.PeriodTimer >= 2 // not accurate, but eliminates annoying clicks
		if sound.On && audible && sound.LengthCounter > 0 && sound.TriangleLinearCounter > 0 {
			sample = sound.getTriangleSample()
		}
	case noiseSoundType:
		vol := sound.getCurrentVolume()
		if sound.On && vol > 0 {
			if sound.LengthCounter > 0 {
				if sound.NoiseShiftRegister&0x01 == 0x01 {
					sample = vol
				}
			}
		}
	case dmcSoundType:
		sample = sound.DMCCurrentValue
	}
	return sample
}

type sound struct {
	SoundType uint8

	On bool

	T    float64
	Freq float64

	// square waves only
	DutyCycleSelector         byte
	SweepEnable               bool
	SweepNegate               bool
	SweepReload               bool
	SweepDivider              byte
	SweepCounter              byte
	SweepShift                byte
	SweepTargetPeriod         uint16
	SweepTargetPeriodOverflow bool
	SweepUsesOnesComplement   bool // hard-wired to pulse1

	TriangleLinearCounter            byte
	TriangleLinearCounterControlFlag bool
	TriangleLinearCounterReloadValue byte
	TriangleLinearCounterReloadFlag  bool

	DMCSampleLength         uint16
	DMCSampleBytesRemaining uint16
	DMCSampleBitsRemaining  uint16
	DMCCurrentSampleByte    byte
	DMCSilenceFlag          bool
	DMCRestartFlag          bool
	DMCInitialSampleAddr    uint16
	DMCCurrentSampleAddr    uint16
	DMCCurrentValue         byte
	DMCIRQEnabled           bool
	DMCLoopEnabled          bool
	DMCInterruptRequested   bool
	DMCPeriod               uint16

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

var dmcPeriodTable = []uint16{
	428, 380, 340, 320, 286, 254, 226, 214, 190, 160, 142, 128, 106, 84, 72, 54,
}

func (sound *sound) loadDMCPeriod(regVal byte) {
	sound.DMCPeriod = dmcPeriodTable[regVal]
}

var noisePeriodTable = []uint16{
	4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 280, 508, 762, 1016, 2034, 4068,
}

func (sound *sound) loadNoisePeriod(regVal byte) {
	sound.NoisePeriod = noisePeriodTable[regVal]
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
	if sound.SweepCounter > 0 {
		sound.SweepCounter--
	} else {
		sound.SweepCounter = sound.SweepDivider
		if sound.SweepEnable && sound.sweepTargetInRange() {
			sound.PeriodTimer = sound.SweepTargetPeriod
			sound.updateFreq()
		}
	}
	if sound.SweepReload {
		sound.SweepCounter = sound.SweepDivider
		sound.SweepReload = false
	}
}

func (sound *sound) sweepTargetInRange() bool {
	return !sound.SweepTargetPeriodOverflow && sound.PeriodTimer >= 8
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
	sum := int(sound.PeriodTimer) + periodDelta
	sound.SweepTargetPeriodOverflow = sum > 0x7ff
	sound.SweepTargetPeriod = uint16(sum) & 0x7ff
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
	if sound.SoundType == squareSoundType {
		sound.T = 0 // nesdev wiki: "sequencer is restarted"
	}
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
	sound.SweepDivider = (val >> 4) & 0x07
	sound.SweepNegate = val&0x08 == 0x08
	sound.SweepShift = val & 0x07
	sound.SweepReload = true
}

func (apu *apu) writeFrameCounterReg(val byte) {
	apu.FrameCounterInterruptInhibit = val&0x40 == 0x40
	apu.FrameCounterSequencerMode = val >> 7

	apu.FrameCounter = 0
	if apu.FrameCounterSequencerMode == 1 {
		apu.FrameCounterManualTrigger = true
	}
	if apu.FrameCounterInterruptInhibit {
		apu.FrameCounterInterruptRequested = false
	}
}

func (sound *sound) writeDMCSampleLength(val byte) {
	sound.DMCSampleLength = (uint16(val) << 4) + 1
}

func (sound *sound) writeDMCInitialSampleAddr(val byte) {
	sound.DMCInitialSampleAddr = 0xc000 | (uint16(val) << 6)
}

func (sound *sound) writeDMCCurrentValue(val byte) {
	sound.DMCCurrentValue = 0x7f & val
}

func (sound *sound) writeDMCFlagsAndRate(val byte) {
	sound.DMCIRQEnabled = val&0x80 == 0x80
	sound.DMCLoopEnabled = val&0x40 == 0x40
	sound.loadDMCPeriod(val & 0x0f)
	sound.updateFreq()
	if !sound.DMCIRQEnabled {
		sound.DMCInterruptRequested = false
	}
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
		if sound.SoundType == dmcSoundType {
			sound.DMCSampleBytesRemaining = 0
		}
	} else {
		if sound.SoundType == dmcSoundType {
			if sound.DMCSampleBytesRemaining == 0 {
				sound.DMCRestartFlag = true
			}
		}
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
		apu.FrameCounterInterruptRequested,
		true,
		apu.DMC.DMCSampleBytesRemaining > 0,
		apu.Noise.LengthCounter > 0,
		apu.Triangle.LengthCounter > 0,
		apu.Pulse2.LengthCounter > 0,
		apu.Pulse1.LengthCounter > 0,
	)
	apu.FrameCounterInterruptRequested = false
	return result
}
