package famigo

import (
	"fmt"
	"os"
)

type cpuState struct {
	Mem mem

	flipRequested bool
	PPU           ppu
	APU           apu

	PC            uint16
	P, A, X, Y, S byte

	IRQ, BRK, NMI, RESET bool

	Steps  uint64
	Cycles uint64

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
		cs.JoypadReg2ReadCount = 0
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
	return 0x40 | boolBit(state, 0)
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
	return 0x40 | boolBit(state, 0)
}

func (cs *cpuState) runCycles(cycles uint) {
	for i := uint(0); i < cycles; i++ {
		for j := 0; j < 3; j++ {
			cs.PPU.runCycle(cs) // ppu clock is 3x cpu
		}
		if cs.Cycles&0x01 == 0x01 {
			cs.APU.runCycle(cs)
		}
		cs.Cycles++
	}
}

func (cs *cpuState) debugStatusLine() string {
	if showMemAccesses {
		fmt.Println()
	}
	opcode := cs.read(cs.PC)
	b2, b3 := cs.read(cs.PC+1), cs.read(cs.PC+2)
	sp := 0x100 + uint16(cs.S)
	s1, s2, s3 := cs.read(sp), cs.read(sp+1), cs.read(sp+2)
	return fmt.Sprintf("Steps: %09d ", cs.Steps) +
		fmt.Sprintf("PC:%04x ", cs.PC) +
		fmt.Sprintf("*PC[:3]:%02x%02x%02x ", opcode, b2, b3) +
		fmt.Sprintf("*S[:3]:%02x%02x%02x ", s1, s2, s3) +
		fmt.Sprintf("opcode:%v ", opcodeNames[opcode]) +
		fmt.Sprintf("A:%02x ", cs.A) +
		fmt.Sprintf("X:%02x ", cs.X) +
		fmt.Sprintf("Y:%02x ", cs.Y) +
		fmt.Sprintf("P:%02x ", cs.P) +
		fmt.Sprintf("S:%02x ", cs.S)
	/*
		return fmt.Sprintf("%04X  ", cs.PC) +
			fmt.Sprintf("%02X %02X %02X  ", opcode, b2, b3) +
			fmt.Sprintf("%v                             ", opcodeNames[opcode]) +
			fmt.Sprintf("A:%02X ", cs.A) +
			fmt.Sprintf("X:%02X ", cs.X) +
			fmt.Sprintf("Y:%02X ", cs.Y) +
			fmt.Sprintf("P:%02X ", cs.P) +
			fmt.Sprintf("SP:%02X", cs.S)
	*/
}

const (
	flagNeg         = 0x80
	flagOverflow    = 0x40
	flagOnStack     = 0x20
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
		cs.S -= 3
		cs.P |= flagIrqDisabled
	} else if cs.NMI {
		cs.NMI = false
		cs.push16(cs.PC)
		cs.push(cs.P | flagOnStack)
		cs.P |= flagIrqDisabled
		cs.PC = cs.read16(0xfffa)
	} else if cs.IRQ && cs.interruptsEnabled() {
		cs.IRQ = false
		cs.push16(cs.PC)
		cs.push(cs.P | flagBrk | flagOnStack)
		cs.P |= flagIrqDisabled
		cs.PC = cs.read16(0xfffe)
	} else if cs.BRK {
		cs.BRK = false
		cs.push16(cs.PC + 1)
		cs.push(cs.P | flagBrk | flagOnStack)
		cs.P |= flagIrqDisabled
		cs.PC = cs.read16(0xfffe)
	}
}

func (cs *cpuState) push16(val uint16) {
	cs.push(byte(val >> 8))
	cs.push(byte(val))
}
func (cs *cpuState) push(val byte) {
	cs.write(0x100+uint16(cs.S), val)
	cs.S--
}

func (cs *cpuState) pop16() uint16 {
	val := uint16(cs.pop())
	val |= uint16(cs.pop()) << 8
	return val
}
func (cs *cpuState) pop() byte {
	cs.S++
	result := cs.read(0x100 + uint16(cs.S))
	return result
}

func (cs *cpuState) interruptsEnabled() bool {
	return cs.P&flagIrqDisabled == 0
}

// Step steps the emulator one instruction
func (cs *cpuState) step() {

	cs.handleInterrupts()

	cs.Steps++

	// fmt.Println(cs.debugStatusLine())

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
			PrgRAM: make([]byte, cartInfo.GetRAMSizePrg()),
		},
		RESET: true,
	}

	cs.APU.init()

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
