package famigo

import "os"

type errEmu struct {
	terminal      dbgTerminal
	screen        [256 * 240 * 4]byte
	flipRequested bool
}

// NewErrEmu returns an emulator that only shows an error message
func NewErrEmu(msg string) Emulator {
	emu := errEmu{}
	emu.terminal = dbgTerminal{w: 256, h: 240, screen: emu.screen[:]}
	os.Stderr.Write([]byte(msg + "\n"))
	emu.terminal.newline()
	emu.terminal.writeString(msg)
	emu.flipRequested = true
	return &emu
}

func (e *errEmu) ReadSoundBuffer(toFill []byte) []byte { return nil }
func (e *errEmu) UpdateInput(input Input)              {}
func (e *errEmu) Step()                                {}

func (e *errEmu) Framebuffer() []byte { return e.screen[:] }
func (e *errEmu) FlipRequested() bool {
	result := e.flipRequested
	e.flipRequested = false
	return result
}
