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
	GetSoundBufferUsed() int

	InDevMode() bool
	SetDevMode(b bool)
}

// Input covers all outside info sent to the Emulator
type Input struct {
	Joypad Joypad
}

// NewEmulator creates an emulation session
func NewEmulator(cart []byte, devMode bool) Emulator {
	return newState(cart, devMode)
}

func (emu *emuState) MakeSnapshot() []byte {
	return emu.makeSnapshot()
}

func (emu *emuState) LoadSnapshot(snapBytes []byte) (Emulator, error) {
	return emu.loadSnapshot(snapBytes)
}

// GetSoundBuffer returns a 44100hz * 16bit * 2ch sound buffer.
// A pre-sized buffer must be provided, which is returned resized
// if the buffer was less full than the length requested.
func (emu *emuState) ReadSoundBuffer(toFill []byte) []byte {
	return emu.APU.readSoundBuffer(emu, toFill)
}

func (emu *emuState) GetSoundBufferUsed() int {
	return int(emu.APU.buffer.size())
}

func (emu *emuState) UpdateInput(input Input) {

	// prevent impossible inputs on original dpad
	if input.Joypad.Up {
		input.Joypad.Down = false
	}
	if input.Joypad.Left {
		input.Joypad.Right = false
	}

	emu.CurrentJoypad1 = input.Joypad
}

// Framebuffer returns the current state of the screen
func (emu *emuState) Framebuffer() []byte {
	return emu.PPU.FrameBuffer[:]
}

// FlipRequested indicates if a draw request is pending
// and clears it before returning
func (emu *emuState) FlipRequested() bool {
	result := emu.flipRequested
	emu.flipRequested = false
	return result
}

func (emu *emuState) GetPrgRAM() []byte {
	if emu.CartInfo.HasBatteryBackedRAM() {
		return emu.Mem.PrgRAM
	}
	return nil
}

func (emu *emuState) SetPrgRAM(ram []byte) error {
	if len(emu.Mem.PrgRAM) == len(ram) {
		copy(emu.Mem.PrgRAM, ram)
		return nil
	}
	// TODO: better checks if possible (e.g. checksums, etc)
	return fmt.Errorf("ram size mismatch")
}

func (emu *emuState) Step() {
	emu.step()
}
