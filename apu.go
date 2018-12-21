package famigo

type apu struct {
	FrameCounterInterruptInhibit   bool
	FrameCounterSequencerMode      byte
	FrameCounterInterruptRequested bool
	FrameCounterManualTrigger      bool
	FrameCounter                   uint64

	lastSample          float64
	lastCorrectedSample float64

	buffer apuCircleBuf

	SampleP1    uint32
	SampleP2    uint32
	SampleTri   uint32
	SampleDMC   uint32
	SampleNoise uint32
	NumSamples  uint32

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
	apuCircleBufSize = 16 * 512 * 4 // must be power of 2
	samplesPerSecond = 44100
)

const cyclesPerSecond = 1789773
const cyclesPerSample = cyclesPerSecond / samplesPerSecond

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
		if c == 2*3728+1 || c == 2*7456+1 || c == 2*11185+1 || c == 2*14914+1 {
			apu.runEnvCycle()
			apu.Triangle.runTriangleLengthCycle()
		}
		if c == 2*7456+1 || c == 2*14914+1 {
			apu.runLengthCycle()
			apu.runSweepCycle()
		}
		if c == 2*14914 || c == 2*14914+1 || c == 2*14915 {
			if !apu.FrameCounterInterruptInhibit {
				apu.FrameCounterInterruptRequested = true
			}
		}
		if c == 2*14915 {
			apu.FrameCounter = 0
		}
	} else {
		c := apu.FrameCounter
		if c == 2*3728+1 || c == 2*7456+1 || c == 2*11185+1 || c == 2*18640+1 {
			apu.runEnvCycle()
			apu.Triangle.runTriangleLengthCycle()
		}
		if c == 2*7456+1 || c == 2*18640+1 {
			apu.runLengthCycle()
			apu.runSweepCycle()
		}
		if c == 2*18641 {
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

func (sound *sound) updateDMCOutput(emu *emuState) {
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
			sound.DMCCurrentSampleByte = emu.read(sound.DMCCurrentSampleAddr)
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

func (apu *apu) genSample(emu *emuState) {
	apu.runFrameCounterCycle()
	if apu.FrameCounterInterruptRequested {
		emu.CPU.IRQ = true
	}

	apu.runFreqCycle(emu)

	apu.SampleP1 += uint32(apu.Pulse1.getSample(emu))
	apu.SampleP2 += uint32(apu.Pulse2.getSample(emu))
	apu.SampleTri += uint32(apu.Triangle.getSample(emu))
	apu.SampleDMC += uint32(apu.DMC.getSample(emu))
	apu.SampleNoise += uint32(apu.Noise.getSample(emu))
	apu.NumSamples++

	if apu.NumSamples >= cyclesPerSample {

		p1 := float64(apu.SampleP1) / float64(apu.NumSamples)
		p2 := float64(apu.SampleP2) / float64(apu.NumSamples)
		tri := float64(apu.SampleTri) / float64(apu.NumSamples)
		dmc := float64(apu.SampleDMC) / float64(apu.NumSamples)
		noise := float64(apu.SampleNoise) / float64(apu.NumSamples)

		pSamples := 95.88 / (8128/(p1+p2) + 100)
		tdnSamples := 159.79 / (1/(tri/8227+noise/12241+dmc/22638) + 100)
		sample := pSamples + tdnSamples

		// dc blocker to center waveform
		correctedSample := sample - apu.lastSample + 0.995*apu.lastCorrectedSample
		apu.lastCorrectedSample = correctedSample
		apu.lastSample = sample
		sample = correctedSample

		left, right := sample, sample

		sampleL, sampleR := int16(left*32767.0), int16(right*32767.0)
		apu.buffer.write([]byte{
			byte(sampleL & 0xff),
			byte(sampleL >> 8),
			byte(sampleR & 0xff),
			byte(sampleR >> 8),
		})

		apu.SampleP1 = 0
		apu.SampleP2 = 0
		apu.SampleTri = 0
		apu.SampleDMC = 0
		apu.SampleNoise = 0
		apu.NumSamples = 0
	}

	if apu.DMC.DMCInterruptRequested {
		emu.CPU.IRQ = true
	}
}

func (apu *apu) readSoundBuffer(emu *emuState, toFill []byte) []byte {
	if int(apu.buffer.size()) < len(toFill) {
		//fmt.Println("audSize:", apu.buffer.size(), "len(toFill)", len(toFill), "buf[0]", apu.buffer.buf[0])
	}
	for int(apu.buffer.size()) < len(toFill) {
		// stretch sound to fill buffer to avoid click
		apu.genSample(emu)
	}
	return apu.buffer.read(toFill)
}

func (apu *apu) runCycle(emu *emuState) {
	if !apu.buffer.full() {
		apu.genSample(emu)
	}
}

func (apu *apu) runFreqCycle(emu *emuState) {
	apu.Pulse1.runFreqCycle(emu)
	apu.Pulse2.runFreqCycle(emu)
	apu.Triangle.runFreqCycle(emu)
	apu.DMC.runFreqCycle(emu)
	apu.Noise.runFreqCycle(emu)
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

func (sound *sound) runFreqCycle(emu *emuState) {

	sound.T++

	if sound.T >= sound.FreqDivider {
		sound.T = 0
		switch sound.SoundType {
		case squareSoundType:
			sound.DutyCycleSeqCounter++
			sound.DutyCycleSeqCounter &= 7
		case triangleSoundType:
			audible := sound.PeriodTimer >= 2 // not accurate, but eliminates annoying clicks
			if sound.On && audible && sound.LengthCounter > 0 && sound.TriangleLinearCounter > 0 {
				sound.TriangleSeqCounter++
				sound.TriangleSeqCounter &= 31
			}
		case noiseSoundType:
			sound.updateNoiseShiftRegister()
		case dmcSoundType:
			sound.updateDMCOutput(emu)
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
		sound.FreqDivider = uint32(sound.DMCPeriod)
	case noiseSoundType:
		sound.FreqDivider = uint32(sound.NoisePeriod)
	case triangleSoundType:
		sound.FreqDivider = uint32(sound.PeriodTimer) + 1
	case squareSoundType:
		// is 16*t+1 for total freq, 2*t+1 for duty cycle sequence
		sound.FreqDivider = 2 * (uint32(sound.PeriodTimer) + 1)
		sound.updateSweepTargetPeriod()
	default:
		panic("unexpected sound type")
	}
}

var dutyCycleTable = [4][8]byte{
	{0, 1, 0, 0, 0, 0, 0, 0},
	{0, 1, 1, 0, 0, 0, 0, 0},
	{0, 1, 1, 1, 1, 0, 0, 0},
	{1, 0, 0, 1, 1, 1, 1, 1},
}

func (sound *sound) inDutyCycle() bool {
	sel := sound.DutyCycleSelector
	counter := sound.DutyCycleSeqCounter
	return dutyCycleTable[sel][counter] == 1
}

var triangleSampleTable = []byte{
	15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

func (sound *sound) getSample(emu *emuState) byte {
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
		sample = triangleSampleTable[sound.TriangleSeqCounter]
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

	T           uint32
	FreqDivider uint32

	// square waves only
	DutyCycleSelector         byte
	DutyCycleSeqCounter       byte
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
	TriangleSeqCounter               byte
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
		sound.LengthCounter = lengthCounterTable[regVal]
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
	sound.VolumeRestart = true
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
