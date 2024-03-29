package famigo

import (
	"fmt"
	"os"
)

type errEmu struct {
	textDisplay   textDisplay
	screen        [256 * 240 * 4]byte
	flipRequested bool

	devMode bool
}

func (e *errEmu) InDevMode() bool   { return e.devMode }
func (e *errEmu) SetDevMode(b bool) { e.devMode = b }

// NewErrEmu returns an emulator that only shows an error message
func NewErrEmu(msg string) Emulator {
	emu := errEmu{}
	emu.textDisplay = textDisplay{w: 256, h: 240, screen: emu.screen[:]}
	os.Stderr.Write([]byte(msg + "\n"))
	emu.textDisplay.newline()
	emu.textDisplay.writeString(msg)
	emu.flipRequested = true
	return &emu
}

func (e *errEmu) GetPrgRAM() []byte      { return []byte{} }
func (e *errEmu) SetPrgRAM([]byte) error { return nil }
func (e *errEmu) MakeSnapshot() []byte   { return nil }
func (e *errEmu) LoadSnapshot([]byte) (Emulator, error) {
	return nil, fmt.Errorf("snapshots not implemented for errEmu")
}
func (e *errEmu) ReadSoundBuffer(toFill []byte) []byte { return nil }
func (e *errEmu) GetSoundBufferUsed() int              { return 0 }
func (e *errEmu) UpdateInput(input Input)              {}
func (e *errEmu) Step()                                {}

func (e *errEmu) Framebuffer() []byte { return e.screen[:] }
func (e *errEmu) FlipRequested() bool {
	result := e.flipRequested
	e.flipRequested = false
	return result
}
