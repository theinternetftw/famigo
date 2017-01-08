package famigo

type apu struct {
	FrameCounterInterruptInhibit bool
	FrameCounterSequencerMode    byte

	Pulse1   sound
	Pulse2   sound
	Triangle sound
	DMC      sound
	Noise    sound

	FrameInterruptRequested bool
}

const (
	squareSoundType   = 0
	triangleSoundType = 1
	dmcSoundType      = 2
	noiseSoundType    = 3
)

type sound struct {
	SoundType uint8

	On bool

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
	CurrentVolume      byte
	InitialVolume      byte

	PeriodTimer uint16

	LengthCounter     byte
	LengthCounterHalt bool
}

func (sound *sound) writeLinearCounterReg(val byte) {
	sound.TriangleLinearCounterControlFlag = val&0x80 == 0x80
	sound.TriangleLinearCounter = val & 0x7f
}

func (sound *sound) writePeriodLowReg(val byte) {
	sound.PeriodTimer &^= 0x00ff
	sound.PeriodTimer |= uint16(val)
}

func (sound *sound) writePeriodHighTimerReg(val byte) {
	sound.PeriodTimer &^= 0x0700
	sound.PeriodTimer |= (uint16(val) & 0x07) << 8
	sound.LengthCounter = val >> 3
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
