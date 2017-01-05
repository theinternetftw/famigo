package famigo

import (
	"fmt"
	"os"
)

type cpuState struct {
	Mem mem

	PPU ppu
	APU apu

	PC            uint16
	P, A, X, Y, S byte

	IRQ, BRK, NMI, RESET bool

	Steps uint64

	CurrentJoypad1 Joypad
	CurrentJoypad2 Joypad // empty for now

	JoypadReg1          Joypad
	JoypadReg2          Joypad
	ReloadingJoypads    bool
	JoypadReg1ReadCount byte
	JoypadReg2ReadCount byte
}

func (cs *cpuState) writeJoypadReg1(val byte) {
	if val&0x01 != 0 {
		cs.ReloadingJoypads = true
		cs.JoypadReg1ReadCount = 0
	} else if cs.ReloadingJoypads {
		cs.ReloadingJoypads = false
	}
}
func (cs *cpuState) getCurrentButtonState(jp *Joypad, readCount byte) bool {
	tbl := []bool{jp.A, jp.B, jp.Sel, jp.Start, jp.Up, jp.Down, jp.Left, jp.Right}
	return tbl[readCount]
}
func (cs *cpuState) readJoypadReg1() byte {
	jp := &cs.CurrentJoypad1
	if cs.ReloadingJoypads {
		return 0x40 | boolBit(jp.A, 0)
	} else if cs.JoypadReg1ReadCount > 7 {
		return 0x41
	}
	state := cs.getCurrentButtonState(jp, cs.JoypadReg1ReadCount)
	cs.JoypadReg1ReadCount++
	return 0x40 | boolBit(state, 1)
}

// writes for this reg handled by apu.writeFrameCounterReg
func (cs *cpuState) readJoypadReg2() byte {
	jp := &cs.CurrentJoypad2
	if cs.ReloadingJoypads {
		return 0x40 | boolBit(jp.A, 0)
	} else if cs.JoypadReg2ReadCount > 7 {
		return 0x41
	}
	state := cs.getCurrentButtonState(jp, cs.JoypadReg2ReadCount)
	cs.JoypadReg2ReadCount++
	return 0x40 | boolBit(state, 1)
}

func (cs *cpuState) runCycles(cycles uint) {
	for i := uint(0); i < cycles; i++ {
		for j := 0; j < 3; j++ {
			cs.PPU.runCycle(cs) // ppu clock is 3x cpu
		}
	}
}

func (cs *cpuState) debugStatusLine() string {
	fmt.Println()
	opcode := cs.read(cs.PC)
	b2, b3 := cs.read(cs.PC+1), cs.read(cs.PC+2)
	return fmt.Sprintf("Steps: %09d ", cs.Steps) +
		fmt.Sprintf("PC:%04x ", cs.PC) +
		fmt.Sprintf("*PC[0:2]:%02x%02x%02x ", opcode, b2, b3) +
		fmt.Sprintf("opcode:%v ", opcodeNames[opcode]) +
		fmt.Sprintf("S:%02x ", cs.S) +
		fmt.Sprintf("A:%02x ", cs.A) +
		fmt.Sprintf("P:%02x ", cs.P) +
		fmt.Sprintf("X:%02x ", cs.X) +
		fmt.Sprintf("Y:%02x ", cs.Y) +
		fmt.Sprintf("IRQ:%v ", cs.IRQ) +
		fmt.Sprintf("BRK:%v ", cs.BRK) +
		fmt.Sprintf("NMI:%v ", cs.NMI) +
		fmt.Sprintf("RESET:%v", cs.RESET)
}

const (
	flagNeg         = 0x80
	flagOverflow    = 0x40
	flagOnStack     = 0x10
	flagBrk         = 0x10
	flagDecimal     = 0x08 // unused
	flagIrqDisabled = 0x04
	flagZero        = 0x02
	flagCarry       = 0x01
)

func (cs *cpuState) handleInterrupts() {
	if cs.RESET {
		cs.RESET = false
		cs.PC = cs.read16(0xfffc)
	} else if cs.NMI {
		cs.NMI = false
		cs.push16(cs.PC)
		cs.push8(cs.P | flagOnStack)
		cs.P |= flagIrqDisabled
		cs.PC = cs.read16(0xfffa)
	} else if cs.IRQ && cs.interruptsEnabled() {
		cs.IRQ = false
		cs.push16(cs.PC)
		cs.push8(cs.P | flagBrk | flagOnStack)
		cs.P |= flagIrqDisabled
		cs.PC = cs.read16(0xfffe)
	} else if cs.BRK {
		cs.BRK = false
		cs.push16(cs.PC)
		cs.push8(cs.P | flagBrk | flagOnStack)
		cs.P |= flagIrqDisabled
		cs.PC = cs.read16(0xfffe)
	}
}

func (cs *cpuState) push16(val uint16) {
	cs.S -= 2
	cs.write16(0x100+uint16(cs.S), val)
}
func (cs *cpuState) push8(val byte) {
	cs.S--
	cs.write(0x100+uint16(cs.S), val)
}

func (cs *cpuState) pop16() uint16 {
	result := cs.read16(0x100 + uint16(cs.S))
	cs.S += 2
	return result
}
func (cs *cpuState) pop() byte {
	result := cs.read(0x100 + uint16(cs.S))
	cs.S++
	return result
}

func (cs *cpuState) interruptsEnabled() bool {
	return cs.P&flagIrqDisabled == 0
}

// Step steps the emulator one instruction
func (cs *cpuState) step() {

	cs.handleInterrupts()

	cs.Steps++

	cs.stepOpcode()
}

func newState(romBytes []byte) *cpuState {
	cartInfo, _ := ParseCartInfo(romBytes)
	prgStart := cartInfo.GetROMOffsetPrg()
	prgEnd := prgStart + cartInfo.GetROMSizePrg()
	chrStart := cartInfo.GetROMOffsetChr()
	chrEnd := chrStart + cartInfo.GetROMSizeChr()
	cs := cpuState{
		Mem: mem{
			MMC:    makeMMC(cartInfo),
			PrgROM: romBytes[prgStart:prgEnd],
			ChrROM: romBytes[chrStart:chrEnd],
		},
		RESET: true,
	}
	return &cs
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

func stepErr(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(1)
}
