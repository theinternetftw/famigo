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
}

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
