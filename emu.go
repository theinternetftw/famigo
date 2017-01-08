package famigo

// Emulator exposes the public facing fns for an emulation session
type Emulator interface {
	Step()

	Framebuffer() []byte
	FlipRequested() bool

	UpdateInput(input Input)
	ReadSoundBuffer([]byte) []byte
}

// Input covers all outside info sent to the Emulator
// TODO: add dt?
type Input struct {
	Joypad Joypad
}

// NewEmulator creates an emulation session
func NewEmulator(cart []byte) Emulator {
	return newState(cart)
}

// GetSoundBuffer returns a 44100hz * 16bit * 2ch sound buffer.
// A pre-sized buffer must be provided, which is returned resized
// if the buffer was less full than the length requested.
func (cs *cpuState) ReadSoundBuffer(toFill []byte) []byte {
	return []byte{}
}

func (cs *cpuState) UpdateInput(input Input) {
	cs.CurrentJoypad1 = input.Joypad
}

// Framebuffer returns the current state of the screen
func (cs *cpuState) Framebuffer() []byte {
	return cs.PPU.FrameBuffer[:]
}

// FlipRequested indicates if a draw request is pending
// and clears it before returning
func (cs *cpuState) FlipRequested() bool {
	result := cs.flipRequested
	cs.flipRequested = false
	return result
}

func (cs *cpuState) Step() {
	cs.step()
}
