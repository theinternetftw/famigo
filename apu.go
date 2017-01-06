package famigo

type apu struct {
	FrameCounterInterruptInhibit bool
	FrameCounterSequencerMode    byte

	Pulse1   sound
	Pulse2   sound
	Triangle sound
	DMC      sound
	Noise    sound

	DMCInterrupt bool
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

	// triangle wave only
	LinearCounter            byte
	LinearCounterControlFlag bool

	UsesConstantVolume bool
	CurrentVolume      byte
	InitialVolume      byte

	PeriodTimer uint16

	LengthCounter     byte
	LengthCounterHalt bool
}

func (sound *sound) writeLinearCounterReg(val byte) {
	sound.LinearCounterControlFlag = val&0x80 == 0x80
	sound.LinearCounter = val & 0x7f
}
func (sound *sound) readLinearCounterReg() byte { return 0xff } // write only

func (sound *sound) writePeriodLowReg(val byte) {
	sound.PeriodTimer &^= 0x00ff
	sound.PeriodTimer |= uint16(val)
}
func (sound *sound) readPeriodLowReg() byte { return 0xff } // write only
func (sound *sound) writePeriodHighTimerReg(val byte) {
	sound.PeriodTimer &^= 0x0700
	sound.PeriodTimer |= (uint16(val) & 0x07) << 8
	sound.LengthCounter = val >> 3
}
func (sound *sound) readPeriodHighTimerReg() byte { return 0xff } // write only

// square waves only
func (sound *sound) writeVolDutyReg(val byte) {
	sound.DutyCycleSelector = val >> 6
	sound.LengthCounterHalt = val&0x20 == 0x20
	sound.UsesConstantVolume = val&0x10 == 0x10
	sound.InitialVolume = val & 0x0f
}
func (sound *sound) readVolDutyReg() byte { return 0xff } // write only
func (sound *sound) writeSweepReg(val byte) {
	sound.SweepEnable = val&0x80 == 0x80
	sound.SweepDivider = ((val >> 4) & 0x07) + 1
	sound.SweepNegate = val&0x08 == 0x08
	sound.SweepShift = val & 0x07
	sound.SweepReload = true
}
func (sound *sound) readSweepReg() byte { return 0xff } // write only

func (apu *apu) writeFrameCounterReg(val byte) {
	apu.FrameCounterInterruptInhibit = val&0x40 != 0
	apu.FrameCounterSequencerMode = val >> 7
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
	apu.DMCInterrupt = false
}
