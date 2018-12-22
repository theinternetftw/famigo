package famigo

import (
	"fmt"
	"os"

	"github.com/theinternetftw/cpugo/virt6502"
)

const (
	showMemReads  = false
	showMemWrites = false
)

type emuState struct {
	Mem mem

	flipRequested bool
	PPU           ppu
	APU           apu

	CPU virt6502.Virt6502

	Cycles   uint64
	CartInfo *CartInfo

	CurrentJoypad1 Joypad
	CurrentJoypad2 Joypad // empty for now

	JoypadReg1          Joypad
	JoypadReg2          Joypad
	ReloadingJoypads    bool
	JoypadReg1ReadCount byte
	JoypadReg2ReadCount byte

	devMode bool
}

func (emu *emuState) InDevMode() bool   { return emu.devMode }
func (emu *emuState) SetDevMode(b bool) { emu.devMode = b }

func (emu *emuState) writeJoypadReg1(val byte) {
	if val&0x01 != 0 {
		emu.ReloadingJoypads = true
		emu.JoypadReg1ReadCount = 0
		emu.JoypadReg2ReadCount = 0
	} else if emu.ReloadingJoypads {
		emu.ReloadingJoypads = false
	}
}
func (emu *emuState) getCurrentButtonState(jp *Joypad, readCount byte) bool {
	tbl := []bool{jp.A, jp.B, jp.Sel, jp.Start, jp.Up, jp.Down, jp.Left, jp.Right}
	return tbl[readCount]
}
func (emu *emuState) readJoypadReg1() byte {
	jp := &emu.CurrentJoypad1
	if emu.ReloadingJoypads {
		return 0x40 | boolBit(jp.A, 0)
	} else if emu.JoypadReg1ReadCount > 7 {
		return 0x41
	}
	state := emu.getCurrentButtonState(jp, emu.JoypadReg1ReadCount)
	emu.JoypadReg1ReadCount++
	return 0x40 | boolBit(state, 0)
}

// writes for this reg handled by apu.writeFrameCounterReg
func (emu *emuState) readJoypadReg2() byte {
	jp := &emu.CurrentJoypad2
	if emu.ReloadingJoypads {
		return 0x40 | boolBit(jp.A, 0)
	} else if emu.JoypadReg2ReadCount > 7 {
		return 0x41
	}
	state := emu.getCurrentButtonState(jp, emu.JoypadReg2ReadCount)
	emu.JoypadReg2ReadCount++
	return 0x40 | boolBit(state, 0)
}

func (emu *emuState) runCycles(cycles uint) {
	for i := uint(0); i < cycles; i++ {
		for j := 0; j < 3; j++ {
			emu.PPU.runCycle(emu) // ppu clock is 3x cpu
		}
		emu.APU.runCycle(emu)
		emu.Mem.mmc.RunCycle(emu)
		emu.Cycles++
	}
}

func (emu *emuState) step() {

	// fmt.Println(emu.CPU.DebugStatusLine())

	if emu.CPU.RESET {
		emu.write(0x4015, 0x00) // all channels off
	}
	emu.CPU.Step()
}

func newState(romBytes []byte, devMode bool) *emuState {
	cartInfo, _ := ParseCartInfo(romBytes)
	prgStart := cartInfo.GetROMOffsetPrg()
	prgEnd := prgStart + cartInfo.GetROMSizePrg()
	chrStart := cartInfo.GetROMOffsetChr()
	chrEnd := chrStart + cartInfo.GetROMSizeChr()
	emu := emuState{
		Mem: mem{
			mmc:    makeMMC(cartInfo),
			prgROM: romBytes[prgStart:prgEnd],
			chrROM: romBytes[chrStart:chrEnd],
			PrgRAM: make([]byte, cartInfo.GetRAMSizePrg()),
		},
		CartInfo: cartInfo,
		devMode:  devMode,
	}
	emu.CPU = virt6502.Virt6502{
		RESET:             true,
		IgnoreDecimalMode: true,
		RunCycles:         emu.runCycles,
		Write:             emu.write,
		Read:              emu.read,
		Err:               func(e error) { emuErr(e) },
	}
	if cartInfo.IsChrRAM() {
		emu.Mem.chrROM = make([]byte, cartInfo.GetRAMSizeChr())
	}

	emu.init()

	return &emu
}

func (emu *emuState) init() {
	emu.Mem.mmc.Init(&emu.Mem)
	emu.APU.init()
}

// Joypad represents the buttons on a gamepad
type Joypad struct {
	Sel   bool
	Start bool
	Up    bool
	Down  bool
	Left  bool
	Right bool
	A     bool
	B     bool
}

func emuErr(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(1)
}
