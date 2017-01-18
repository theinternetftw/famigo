package famigo

import "fmt"

// Emulator exposes the public facing fns for an emulation session
type Emulator interface {
	Step()

	MakeSnapshot() []byte
	LoadSnapshot([]byte) (Emulator, error)

	SetPrgRAM([]byte) error
	GetPrgRAM() []byte

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

func (cs *cpuState) MakeSnapshot() []byte {
	return cs.makeSnapshot()
}

func (cs *cpuState) LoadSnapshot(snapBytes []byte) (Emulator, error) {
	return cs.loadSnapshot(snapBytes)
}

// GetSoundBuffer returns a 44100hz * 16bit * 2ch sound buffer.
// A pre-sized buffer must be provided, which is returned resized
// if the buffer was less full than the length requested.
func (cs *cpuState) ReadSoundBuffer(toFill []byte) []byte {
	return cs.APU.buffer.read(toFill)
}

func (cs *cpuState) UpdateInput(input Input) {

	// prevent impossible inputs on original dpad
	if input.Joypad.Up {
		input.Joypad.Down = false
	}
	if input.Joypad.Left {
		input.Joypad.Right = false
	}

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

func (cs *cpuState) GetPrgRAM() []byte {
	if cs.CartInfo.HasBatteryBackedRAM() {
		return cs.Mem.PrgRAM
	}
	return nil
}

func (cs *cpuState) SetPrgRAM(ram []byte) error {
	if len(cs.Mem.PrgRAM) == len(ram) {
		copy(cs.Mem.PrgRAM, ram)
		return nil
	}
	// TODO: better checks if possible (e.g. checksums, etc)
	return fmt.Errorf("ram size mismatch")
}

func (cs *cpuState) Step() {
	cs.step()
}
