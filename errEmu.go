package famigo

import "os"

type errEmu struct {
	cursor        dbgCursor
	screen        [256 * 240 * 4]byte
	flipRequested bool
}

// NewErrEmu returns an emulator that only shows an error message
func NewErrEmu(msg string) Emulator {
	emu := errEmu{
		cursor: dbgCursor{w: 256, h: 240},
	}
	os.Stderr.Write([]byte(msg + "\n"))
	emu.cursor.newline()
	emu.cursor.writeString(emu.screen[:], msg)
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
