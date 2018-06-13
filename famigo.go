package famigo

import (
	"fmt"
	"os"
)

type emuState struct {
	Mem mem

	flipRequested bool
	PPU           ppu
	APU           apu

	PC            uint16
	P, A, X, Y, S byte

	IRQ, BRK, NMI, RESET bool
	LastStepsP           byte

	Steps    uint64
	Cycles   uint64
	CartInfo *CartInfo

	CurrentJoypad1 Joypad
	CurrentJoypad2 Joypad // empty for now

	JoypadReg1          Joypad
	JoypadReg2          Joypad
	ReloadingJoypads    bool
	JoypadReg1ReadCount byte
	JoypadReg2ReadCount byte
}

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

func (emu *emuState) debugStatusLine() string {
	if showMemReads {
		fmt.Println()
	}
	opcode := emu.read(emu.PC)
	b2, b3 := emu.read(emu.PC+1), emu.read(emu.PC+2)
	sp := 0x100 + uint16(emu.S)
	s1, s2, s3 := emu.read(sp), emu.read(sp+1), emu.read(sp+2)
	return fmt.Sprintf("Steps: %09d ", emu.Steps) +
		fmt.Sprintf("PC:%04x ", emu.PC) +
		fmt.Sprintf("*PC[:3]:%02x%02x%02x ", opcode, b2, b3) +
		fmt.Sprintf("*S[:3]:%02x%02x%02x ", s1, s2, s3) +
		fmt.Sprintf("opcode:%v ", opcodeNames[opcode]) +
		fmt.Sprintf("A:%02x ", emu.A) +
		fmt.Sprintf("X:%02x ", emu.X) +
		fmt.Sprintf("Y:%02x ", emu.Y) +
		fmt.Sprintf("P:%02x ", emu.P) +
		fmt.Sprintf("S:%02x ", emu.S)
	/*
		return fmt.Sprintf("%04X  ", emu.PC) +
			fmt.Sprintf("%02X %02X %02X  ", opcode, b2, b3) +
			fmt.Sprintf("%v                             ", opcodeNames[opcode]) +
			fmt.Sprintf("A:%02X ", emu.A) +
			fmt.Sprintf("X:%02X ", emu.X) +
			fmt.Sprintf("Y:%02X ", emu.Y) +
			fmt.Sprintf("P:%02X ", emu.P) +
			fmt.Sprintf("SP:%02X", emu.S)
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

func (emu *emuState) handleInterrupts() {
	if emu.RESET {
		emu.RESET = false
		emu.PC = emu.read16(0xfffc)
		emu.S -= 3
		emu.P |= flagIrqDisabled
		emu.write(0x4015, 0x00) // all channels off
	} else if emu.BRK {
		emu.BRK = false
		emu.push16(emu.PC + 1)
		emu.push(emu.P | flagBrk | flagOnStack)
		emu.P |= flagIrqDisabled
		emu.PC = emu.read16(0xfffe)
	} else if emu.NMI {
		emu.NMI = false
		emu.push16(emu.PC)
		emu.push(emu.P | flagOnStack)
		emu.P |= flagIrqDisabled
		emu.PC = emu.read16(0xfffa)
	} else if emu.IRQ {
		emu.IRQ = false
		if emu.interruptsEnabled() {
			emu.push16(emu.PC)
			emu.push(emu.P | flagOnStack)
			emu.P |= flagIrqDisabled
			emu.PC = emu.read16(0xfffe)
		}
	}
	emu.LastStepsP = emu.P
}

func (emu *emuState) push16(val uint16) {
	emu.push(byte(val >> 8))
	emu.push(byte(val))
}
func (emu *emuState) push(val byte) {
	emu.write(0x100+uint16(emu.S), val)
	emu.S--
}

func (emu *emuState) pop16() uint16 {
	val := uint16(emu.pop())
	val |= uint16(emu.pop()) << 8
	return val
}
func (emu *emuState) pop() byte {
	emu.S++
	result := emu.read(0x100 + uint16(emu.S))
	return result
}

func (emu *emuState) interruptsEnabled() bool {
	return emu.LastStepsP&flagIrqDisabled == 0
}

const (
	showMemReads  = false
	showMemWrites = false
)

func (emu *emuState) step() {

	emu.handleInterrupts()

	emu.Steps++

	// fmt.Println(emu.debugStatusLine())

	emu.stepOpcode()
}

func newState(romBytes []byte) *emuState {
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
		RESET:    true,
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
