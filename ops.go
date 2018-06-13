package famigo

import "fmt"

func (emu *emuState) opFn(cycles uint, instLen uint16, fn func()) {
	fn()
	emu.PC += instLen
	emu.runCycles(cycles)
}

func (emu *emuState) setZeroNeg(val byte) {
	if val == 0 {
		emu.P |= flagZero
	} else {
		emu.P &^= flagZero
	}
	if val&0x80 == 0x80 {
		emu.P |= flagNeg
	} else {
		emu.P &^= flagNeg
	}
}

func (emu *emuState) setRegOp(numCycles uint, instLen uint16, dst *byte, src byte, flagFn func(byte)) {
	*dst = src
	emu.PC += instLen
	emu.runCycles(numCycles)
	flagFn(*dst)
}
func (emu *emuState) storeOp(numCycles uint, instLen uint16, addr uint16, val byte, flagFn func(byte)) {
	emu.write(addr, val)
	emu.PC += instLen
	emu.runCycles(numCycles)
	flagFn(val)
}

func (emu *emuState) setNoFlags(val byte) {}

// emu.PC must be in right place, obviously
func (emu *emuState) getYPostIndexedAddr() (uint16, uint) {
	zPageLowAddr := uint16(emu.read(emu.PC + 1))
	zPageHighAddr := uint16(emu.read(emu.PC+1) + 1) // wraps at 0xff
	baseAddr := (uint16(emu.read(zPageHighAddr)) << 8) | uint16(emu.read(zPageLowAddr))
	addr := baseAddr + uint16(emu.Y)
	if addr&0xff00 != baseAddr&0xff00 { // if not same page, takes extra cycle
		return addr, 1
	}
	return addr, 0
}
func (emu *emuState) getXPreIndexedAddr() uint16 {
	zPageLowAddr := uint16(emu.read(emu.PC+1) + emu.X)      // wraps at 0xff
	zPageHighAddr := uint16(emu.read(emu.PC+1) + emu.X + 1) // wraps at 0xff
	return (uint16(emu.read(zPageHighAddr)) << 8) | uint16(emu.read(zPageLowAddr))
}
func (emu *emuState) getZeroPageAddr() uint16 {
	return uint16(emu.read(emu.PC + 1))
}
func (emu *emuState) getIndexedZeroPageAddr(idx byte) uint16 {
	return uint16(emu.read(emu.PC+1) + idx) // wraps at 0xff
}
func (emu *emuState) getAbsoluteAddr() uint16 {
	return emu.read16(emu.PC + 1)
}
func (emu *emuState) getIndexedAbsoluteAddr(idx byte) (uint16, uint) {
	base := emu.read16(emu.PC + 1)
	addr := base + uint16(idx)
	if base&0xff00 != addr&0xff00 { // if not same page, takes extra cycle
		return addr, 1
	}
	return addr, 0
}
func (emu *emuState) getIndirectJmpAddr() uint16 {
	// hw bug! similar to other indexing wrapping issues...
	operandAddr := emu.getAbsoluteAddr()
	highAddr := (operandAddr & 0xff00) | ((operandAddr + 1) & 0xff) // lo-byte wraps at 0xff
	return (uint16(emu.read(highAddr)) << 8) | uint16(emu.read(operandAddr))
}

var opcodeNames = []string{
	"BRK", "ORA", "XXX", "XXX", "XXX", "ORA", "ASL", "XXX", "PHP", "ORA", "ASL", "XXX", "XXX", "ORA", "ASL", "XXX",
	"BPL", "ORA", "XXX", "XXX", "XXX", "ORA", "ASL", "XXX", "CLC", "ORA", "XXX", "XXX", "XXX", "ORA", "ASL", "XXX",
	"JSR", "AND", "XXX", "XXX", "BIT", "AND", "ROL", "XXX", "PLP", "AND", "ROL", "XXX", "BIT", "AND", "ROL", "XXX",
	"BMI", "AND", "XXX", "XXX", "XXX", "AND", "ROL", "XXX", "SEC", "AND", "XXX", "XXX", "XXX", "AND", "ROL", "XXX",
	"RTI", "EOR", "XXX", "XXX", "XXX", "EOR", "LSR", "XXX", "PHA", "EOR", "LSR", "XXX", "JMP", "EOR", "LSR", "XXX",
	"BVC", "EOR", "XXX", "XXX", "XXX", "EOR", "LSR", "XXX", "CLI", "EOR", "XXX", "XXX", "XXX", "EOR", "LSR", "XXX",
	"RTS", "ADC", "XXX", "XXX", "XXX", "ADC", "ROR", "XXX", "PLA", "ADC", "ROR", "XXX", "JMP", "ADC", "ROR", "XXX",
	"BVS", "ADC", "XXX", "XXX", "XXX", "ADC", "ROR", "XXX", "SEI", "ADC", "XXX", "XXX", "XXX", "ADC", "ROR", "XXX",
	"XXX", "STA", "XXX", "XXX", "STY", "STA", "STX", "XXX", "DEY", "XXX", "TXA", "XXX", "STY", "STA", "STX", "XXX",
	"BCC", "STA", "XXX", "XXX", "STY", "STA", "STX", "XXX", "TYA", "STA", "TXS", "XXX", "XXX", "STA", "XXX", "XXX",
	"LDY", "LDA", "LDX", "XXX", "LDY", "LDA", "LDX", "XXX", "TAY", "LDA", "TAX", "XXX", "LDY", "LDA", "LDX", "XXX",
	"BCS", "LDA", "XXX", "XXX", "LDY", "LDA", "LDX", "XXX", "CLV", "LDA", "TSX", "XXX", "LDY", "LDA", "LDX", "XXX",
	"CPY", "CMP", "XXX", "XXX", "CPY", "CMP", "DEC", "XXX", "INY", "CMP", "DEX", "XXX", "CPY", "CMP", "DEC", "XXX",
	"BNE", "CMP", "XXX", "XXX", "XXX", "CMP", "DEC", "XXX", "CLD", "CMP", "XXX", "XXX", "XXX", "CMP", "DEC", "XXX",
	"CPX", "SBC", "XXX", "XXX", "CPX", "SBC", "INC", "XXX", "INX", "SBC", "NOP", "XXX", "CPX", "SBC", "INC", "XXX",
	"BEQ", "SBC", "XXX", "XXX", "XXX", "SBC", "INC", "XXX", "SED", "SBC", "XXX", "XXX", "XXX", "SBC", "INC", "XXX",
}

const crashOnUndocumentOpcode = false

func (emu *emuState) undocumentedOpcode() {
	if crashOnUndocumentOpcode {
		emuErr(fmt.Sprintf("Undocumented opcode 0x%02x at 0x%04x", emu.read(emu.PC), emu.PC))
	}
}

func (emu *emuState) stepOpcode() {

	opcode := emu.read(emu.PC)
	switch opcode {
	case 0x00: // BRK
		emu.opFn(7, 1, func() { emu.BRK = true })
	case 0x01: // ORA (indirect,x)
		addr := emu.getXPreIndexedAddr()
		emu.setRegOp(6, 2, &emu.A, emu.A|emu.read(addr), emu.setZeroNeg)
	case 0x04: // 2-nop (UNDOCUMENTED)
		emu.opFn(3, 2, emu.undocumentedOpcode)
	case 0x05: // ORA zeropage
		addr := emu.getZeroPageAddr()
		emu.setRegOp(3, 2, &emu.A, emu.A|emu.read(addr), emu.setZeroNeg)
	case 0x06: // ASL zeropage
		addr := emu.getZeroPageAddr()
		emu.storeOp(5, 2, addr, emu.aslAndSetFlags(emu.read(addr)), emu.setNoFlags)
	case 0x08: // PHP
		emu.opFn(3, 1, func() { emu.push(emu.P | flagOnStack | flagBrk) })
	case 0x09: // ORA imm
		addr := emu.PC + 1
		emu.setRegOp(2, 2, &emu.A, emu.A|emu.read(addr), emu.setZeroNeg)
	case 0x0a: // ASL A
		emu.opFn(2, 1, func() { emu.A = emu.aslAndSetFlags(emu.A) })
	case 0x0c: // 3-nop (UNDOCUMENTED)
		emu.opFn(4, 3, emu.undocumentedOpcode)
	case 0x0d: // ORA absolute
		addr := emu.getAbsoluteAddr()
		emu.setRegOp(4, 3, &emu.A, emu.A|emu.read(addr), emu.setZeroNeg)
	case 0x0e: // ASL absolute
		addr := emu.getAbsoluteAddr()
		emu.storeOp(6, 3, addr, emu.aslAndSetFlags(emu.read(addr)), emu.setNoFlags)

	case 0x10: // BPL
		emu.branchOpRel(emu.P&flagNeg == 0)
	case 0x11: // ORA (indirect),y
		addr, cycles := emu.getYPostIndexedAddr()
		emu.setRegOp(5+cycles, 2, &emu.A, emu.A|emu.read(addr), emu.setZeroNeg)
	case 0x14: // 2-nop (UNDOCUMENTED)
		emu.opFn(4, 2, emu.undocumentedOpcode)
	case 0x15: // ORA zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.setRegOp(4, 2, &emu.A, emu.A|emu.read(addr), emu.setZeroNeg)
	case 0x16: // ASL zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.storeOp(6, 2, addr, emu.aslAndSetFlags(emu.read(addr)), emu.setNoFlags)
	case 0x18: // CLC
		emu.opFn(2, 1, func() { emu.P &^= flagCarry })
	case 0x19: // ORA absolute,y
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.setRegOp(4+cycles, 3, &emu.A, emu.A|emu.read(addr), emu.setZeroNeg)
	case 0x1a: // 1-nop (UNDOCUMENTED)
		emu.opFn(2, 1, emu.undocumentedOpcode)
	case 0x1c: // 3-nop (UNDOCUMENTED)
		_, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.opFn(4+cycles, 3, emu.undocumentedOpcode)
	case 0x1d: // ORA absolute,x
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.setRegOp(4+cycles, 3, &emu.A, emu.A|emu.read(addr), emu.setZeroNeg)
	case 0x1e: // ASL absolute,x
		addr, _ := emu.getIndexedAbsoluteAddr(emu.X)
		emu.storeOp(7, 3, addr, emu.aslAndSetFlags(emu.read(addr)), emu.setNoFlags)

	case 0x20: // JSR (jump and store return addr)
		emu.push16(emu.PC + 2)
		emu.jmpOp(6, 3, emu.getAbsoluteAddr())
	case 0x21: // AND (indirect,x)
		addr := emu.getXPreIndexedAddr()
		emu.setRegOp(6, 2, &emu.A, emu.A&emu.read(addr), emu.setZeroNeg)
	case 0x24: // BIT zeropage
		addr := emu.getZeroPageAddr()
		emu.opFn(3, 2, func() { emu.bitAndSetFlags(emu.read(addr)) })
	case 0x25: // AND zeropage
		addr := emu.getZeroPageAddr()
		emu.setRegOp(3, 2, &emu.A, emu.A&emu.read(addr), emu.setZeroNeg)
	case 0x26: // ROL zeropage
		addr := emu.getZeroPageAddr()
		emu.storeOp(5, 2, addr, emu.rolAndSetFlags(emu.read(addr)), emu.setNoFlags)
	case 0x28: // PLP
		flags := emu.pop() &^ (flagBrk | flagOnStack)
		emu.setRegOp(4, 1, &emu.P, flags, emu.setNoFlags)
	case 0x29: // AND imm
		addr := emu.PC + 1
		emu.setRegOp(2, 2, &emu.A, emu.A&emu.read(addr), emu.setZeroNeg)
	case 0x2a: // ROL A
		emu.opFn(2, 1, func() { emu.A = emu.rolAndSetFlags(emu.A) })
	case 0x2c: // BIT absolute
		addr := emu.getAbsoluteAddr()
		emu.opFn(4, 3, func() { emu.bitAndSetFlags(emu.read(addr)) })
	case 0x2d: // AND absolute
		addr := emu.getAbsoluteAddr()
		emu.setRegOp(4, 3, &emu.A, emu.A&emu.read(addr), emu.setZeroNeg)
	case 0x2e: // ROL absolute
		addr := emu.getAbsoluteAddr()
		emu.storeOp(6, 3, addr, emu.rolAndSetFlags(emu.read(addr)), emu.setNoFlags)

	case 0x30: // BMI
		emu.branchOpRel(emu.P&flagNeg == flagNeg)
	case 0x31: // AND (indirect),y
		addr, cycles := emu.getYPostIndexedAddr()
		emu.setRegOp(5+cycles, 2, &emu.A, emu.A&emu.read(addr), emu.setZeroNeg)
	case 0x34: // 2-nop (UNDOCUMENTED)
		emu.opFn(4, 2, emu.undocumentedOpcode)
	case 0x35: // AND zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.setRegOp(4, 2, &emu.A, emu.A&emu.read(addr), emu.setZeroNeg)
	case 0x36: // ROL zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.storeOp(6, 2, addr, emu.rolAndSetFlags(emu.read(addr)), emu.setNoFlags)
	case 0x38: // SEC
		emu.opFn(2, 1, func() { emu.P |= flagCarry })
	case 0x39: // AND absolute,y
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.setRegOp(4+cycles, 3, &emu.A, emu.A&emu.read(addr), emu.setZeroNeg)
	case 0x3a: // 1-nop (UNDOCUMENTED)
		emu.opFn(2, 1, emu.undocumentedOpcode)
	case 0x3c: // 3-nop (UNDOCUMENTED)
		_, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.opFn(4+cycles, 3, emu.undocumentedOpcode)
	case 0x3d: // AND absolute,x
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.setRegOp(4+cycles, 3, &emu.A, emu.A&emu.read(addr), emu.setZeroNeg)
	case 0x3e: // ROL absolute,x
		addr, _ := emu.getIndexedAbsoluteAddr(emu.X)
		emu.storeOp(7, 3, addr, emu.rolAndSetFlags(emu.read(addr)), emu.setNoFlags)

	case 0x40: // RTI (return from interrupt)
		emu.P = emu.pop() &^ (flagBrk | flagOnStack)
		emu.LastStepsP = emu.P                          // no lag from RTI
		emu.opFn(6, 0, func() { emu.PC = emu.pop16() }) // real instLen 1, but we don't want to step past newPC (unlike RTS)
	case 0x41: // EOR (indirect,x)
		addr := emu.getXPreIndexedAddr()
		emu.setRegOp(6, 2, &emu.A, emu.A^emu.read(addr), emu.setZeroNeg)
	case 0x44: // 2-nop (UNDOCUMENTED)
		emu.opFn(3, 2, emu.undocumentedOpcode)
	case 0x45: // EOR zeropage
		addr := emu.getZeroPageAddr()
		emu.setRegOp(3, 2, &emu.A, emu.A^emu.read(addr), emu.setZeroNeg)
	case 0x46: // LSR zeropage
		addr := emu.getZeroPageAddr()
		emu.storeOp(5, 2, addr, emu.lsrAndSetFlags(emu.read(addr)), emu.setNoFlags)
	case 0x48: // PHA
		emu.opFn(3, 1, func() { emu.push(emu.A) })
	case 0x49: // EOR imm
		addr := emu.PC + 1
		emu.setRegOp(2, 2, &emu.A, emu.A^emu.read(addr), emu.setZeroNeg)
	case 0x4a: // LSR A
		emu.opFn(2, 1, func() { emu.A = emu.lsrAndSetFlags(emu.A) })
	case 0x4c: // JMP absolute
		emu.jmpOp(3, 3, emu.getAbsoluteAddr())
	case 0x4d: // EOR absolute
		addr := emu.getAbsoluteAddr()
		emu.setRegOp(4, 3, &emu.A, emu.A^emu.read(addr), emu.setZeroNeg)
	case 0x4e: // LSR absolute
		addr := emu.getAbsoluteAddr()
		emu.storeOp(6, 3, addr, emu.lsrAndSetFlags(emu.read(addr)), emu.setNoFlags)

	case 0x50: // BVC
		emu.branchOpRel(emu.P&flagOverflow == 0)
	case 0x51: // EOR (indirect),y
		addr, cycles := emu.getYPostIndexedAddr()
		emu.setRegOp(5+cycles, 2, &emu.A, emu.A^emu.read(addr), emu.setZeroNeg)
	case 0x54: // 2-nop (UNDOCUMENTED)
		emu.opFn(4, 2, emu.undocumentedOpcode)
	case 0x55: // EOR zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.setRegOp(4, 2, &emu.A, emu.A^emu.read(addr), emu.setZeroNeg)
	case 0x56: // LSR zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.storeOp(6, 2, addr, emu.lsrAndSetFlags(emu.read(addr)), emu.setNoFlags)
	case 0x58: // CLI
		emu.opFn(2, 1, func() { emu.P &^= flagIrqDisabled })
	case 0x59: // EOR absolute,y
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.setRegOp(4+cycles, 3, &emu.A, emu.A^emu.read(addr), emu.setZeroNeg)
	case 0x5a: // 1-nop (UNDOCUMENTED)
		emu.opFn(2, 1, emu.undocumentedOpcode)
	case 0x5c: // 3-nop (UNDOCUMENTED)
		_, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.opFn(4+cycles, 3, emu.undocumentedOpcode)
	case 0x5d: // EOR absolute,x
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.setRegOp(4+cycles, 3, &emu.A, emu.A^emu.read(addr), emu.setZeroNeg)
	case 0x5e: // LSR absolute,x
		addr, _ := emu.getIndexedAbsoluteAddr(emu.X)
		emu.storeOp(7, 3, addr, emu.lsrAndSetFlags(emu.read(addr)), emu.setNoFlags)

	case 0x60: // RTS (return from subroutine)
		emu.opFn(6, 1, func() { emu.PC = emu.pop16() }) // opFn adds 1 to PC, so does real 6502
	case 0x61: // ADC (indirect,x)
		addr := emu.getXPreIndexedAddr()
		emu.opFn(6, 2, func() { emu.A = emu.adcAndSetFlags(emu.read(addr)) })
	case 0x64: // 2-nop (UNDOCUMENTED)
		emu.opFn(3, 2, emu.undocumentedOpcode)
	case 0x65: // ADC zeropage
		addr := emu.getZeroPageAddr()
		emu.opFn(3, 2, func() { emu.A = emu.adcAndSetFlags(emu.read(addr)) })
	case 0x66: // ROR zeropage
		addr := emu.getZeroPageAddr()
		emu.storeOp(5, 2, addr, emu.rorAndSetFlags(emu.read(addr)), emu.setNoFlags)
	case 0x68: // PLA
		emu.setRegOp(4, 1, &emu.A, emu.pop(), emu.setZeroNeg)
	case 0x69: // ADC imm
		addr := emu.PC + 1
		emu.opFn(2, 2, func() { emu.A = emu.adcAndSetFlags(emu.read(addr)) })
	case 0x6a: // ROR A
		emu.opFn(2, 1, func() { emu.A = emu.rorAndSetFlags(emu.A) })
	case 0x6c: // JMP (indirect)
		emu.jmpOp(5, 3, emu.getIndirectJmpAddr())
	case 0x6d: // ADC absolute
		addr := emu.getAbsoluteAddr()
		emu.opFn(4, 3, func() { emu.A = emu.adcAndSetFlags(emu.read(addr)) })
	case 0x6e: // ROR absolute
		addr := emu.getAbsoluteAddr()
		emu.storeOp(6, 3, addr, emu.rorAndSetFlags(emu.read(addr)), emu.setNoFlags)

	case 0x70: // BVS
		emu.branchOpRel(emu.P&flagOverflow == flagOverflow)
	case 0x71: // ADC (indirect),y
		addr, cycles := emu.getYPostIndexedAddr()
		emu.opFn(5+cycles, 2, func() { emu.A = emu.adcAndSetFlags(emu.read(addr)) })
	case 0x74: // 2-nop (UNDOCUMENTED)
		emu.opFn(4, 2, emu.undocumentedOpcode)
	case 0x75: // ADC zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.opFn(4, 2, func() { emu.A = emu.adcAndSetFlags(emu.read(addr)) })
	case 0x76: // ROR zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.storeOp(6, 2, addr, emu.rorAndSetFlags(emu.read(addr)), emu.setNoFlags)
	case 0x78: // SEI
		emu.opFn(2, 1, func() { emu.P |= flagIrqDisabled })
	case 0x79: // ADC absolute,y
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.opFn(4+cycles, 3, func() { emu.A = emu.adcAndSetFlags(emu.read(addr)) })
	case 0x7a: // 1-nop (UNDOCUMENTED)
		emu.opFn(2, 1, emu.undocumentedOpcode)
	case 0x7c: // 3-nop (UNDOCUMENTED)
		_, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.opFn(4+cycles, 3, emu.undocumentedOpcode)
	case 0x7d: // ADC absolute,x
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.opFn(4+cycles, 3, func() { emu.A = emu.adcAndSetFlags(emu.read(addr)) })
	case 0x7e: // ROR absolute,x
		addr, _ := emu.getIndexedAbsoluteAddr(emu.X)
		emu.storeOp(7, 3, addr, emu.rorAndSetFlags(emu.read(addr)), emu.setNoFlags)

	case 0x80: // 2-nop (UNDOCUMENTED)
		emu.opFn(2, 2, emu.undocumentedOpcode)
	case 0x81: // STA (indirect,x)
		emu.storeOp(6, 2, emu.getXPreIndexedAddr(), emu.A, emu.setNoFlags)
	case 0x82: // 2-nop (UNDOCUMENTED)
		emu.opFn(2, 2, emu.undocumentedOpcode)
	case 0x84: // STY zeropage
		emu.storeOp(3, 2, emu.getZeroPageAddr(), emu.Y, emu.setNoFlags)
	case 0x85: // STA zeropage
		emu.storeOp(3, 2, emu.getZeroPageAddr(), emu.A, emu.setNoFlags)
	case 0x86: // STX zeropage
		emu.storeOp(3, 2, emu.getZeroPageAddr(), emu.X, emu.setNoFlags)
	case 0x88: // DEY
		emu.setRegOp(2, 1, &emu.Y, emu.Y-1, emu.setZeroNeg)
	case 0x89: // 2-nop (UNDOCUMENTED)
		emu.opFn(2, 2, emu.undocumentedOpcode)
	case 0x8a: // TXA
		emu.setRegOp(2, 1, &emu.A, emu.X, emu.setZeroNeg)
	case 0x8c: // STY absolute
		emu.storeOp(4, 3, emu.getAbsoluteAddr(), emu.Y, emu.setNoFlags)
	case 0x8d: // STA absolute
		emu.storeOp(4, 3, emu.getAbsoluteAddr(), emu.A, emu.setNoFlags)
	case 0x8e: // STX absolute
		emu.storeOp(4, 3, emu.getAbsoluteAddr(), emu.X, emu.setNoFlags)

	case 0x90: // BCC
		emu.branchOpRel(emu.P&flagCarry == 0)
	case 0x91: // STA (indirect),y
		addr, _ := emu.getYPostIndexedAddr()
		emu.storeOp(6, 2, addr, emu.A, emu.setNoFlags)
	case 0x94: // STY zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.storeOp(4, 2, addr, emu.Y, emu.setNoFlags)
	case 0x95: // STA zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.storeOp(4, 2, addr, emu.A, emu.setNoFlags)
	case 0x96: // STX zeropage,y
		addr := emu.getIndexedZeroPageAddr(emu.Y)
		emu.storeOp(4, 2, addr, emu.X, emu.setNoFlags)
	case 0x98: // TYA
		emu.setRegOp(2, 1, &emu.A, emu.Y, emu.setZeroNeg)
	case 0x99: // STA absolute,y
		addr, _ := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.storeOp(5, 3, addr, emu.A, emu.setNoFlags)
	case 0x9a: // TXS
		emu.setRegOp(2, 1, &emu.S, emu.X, emu.setNoFlags)
	case 0x9d: // STA absolute,x
		addr, _ := emu.getIndexedAbsoluteAddr(emu.X)
		emu.storeOp(5, 3, addr, emu.A, emu.setNoFlags)

	case 0xa0: // LDY imm
		addr := emu.PC + 1
		emu.setRegOp(2, 2, &emu.Y, emu.read(addr), emu.setZeroNeg)
	case 0xa1: // LDA (indirect,x)
		addr := emu.getXPreIndexedAddr()
		emu.setRegOp(6, 2, &emu.A, emu.read(addr), emu.setZeroNeg)
	case 0xa2: // LDX imm
		addr := emu.PC + 1
		emu.setRegOp(2, 2, &emu.X, emu.read(addr), emu.setZeroNeg)
	case 0xa4: // LDY zeropage
		addr := emu.getZeroPageAddr()
		emu.setRegOp(3, 2, &emu.Y, emu.read(addr), emu.setZeroNeg)
	case 0xa5: // LDA zeropage
		addr := emu.getZeroPageAddr()
		emu.setRegOp(3, 2, &emu.A, emu.read(addr), emu.setZeroNeg)
	case 0xa6: // LDX zeropage
		addr := emu.getZeroPageAddr()
		emu.setRegOp(3, 2, &emu.X, emu.read(addr), emu.setZeroNeg)
	case 0xa8: // TAY
		emu.setRegOp(2, 1, &emu.Y, emu.A, emu.setZeroNeg)
	case 0xa9: // LDA imm
		addr := emu.PC + 1
		emu.setRegOp(2, 2, &emu.A, emu.read(addr), emu.setZeroNeg)
	case 0xaa: // TAX
		emu.setRegOp(2, 1, &emu.X, emu.A, emu.setZeroNeg)
	case 0xac: // LDY absolute
		addr := emu.getAbsoluteAddr()
		emu.setRegOp(4, 3, &emu.Y, emu.read(addr), emu.setZeroNeg)
	case 0xad: // LDA absolute
		addr := emu.getAbsoluteAddr()
		emu.setRegOp(4, 3, &emu.A, emu.read(addr), emu.setZeroNeg)
	case 0xae: // LDX absolute
		addr := emu.getAbsoluteAddr()
		emu.setRegOp(4, 3, &emu.X, emu.read(addr), emu.setZeroNeg)

	case 0xb0: // BCS
		emu.branchOpRel(emu.P&flagCarry == flagCarry)
	case 0xb1: // LDA (indirect),y
		addr, cycles := emu.getYPostIndexedAddr()
		emu.setRegOp(5+cycles, 2, &emu.A, emu.read(addr), emu.setZeroNeg)
	case 0xb4: // LDY zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.setRegOp(4, 2, &emu.Y, emu.read(addr), emu.setZeroNeg)
	case 0xb5: // LDA zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.setRegOp(4, 2, &emu.A, emu.read(addr), emu.setZeroNeg)
	case 0xb6: // LDX zeropage,y
		addr := emu.getIndexedZeroPageAddr(emu.Y)
		emu.setRegOp(4, 2, &emu.X, emu.read(addr), emu.setZeroNeg)
	case 0xb8: // CLV
		emu.opFn(2, 1, func() { emu.P &^= flagOverflow })
	case 0xb9: // LDA absolute, y
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.setRegOp(4+cycles, 3, &emu.A, emu.read(addr), emu.setZeroNeg)
	case 0xba: // TSX
		emu.setRegOp(2, 1, &emu.X, emu.S, emu.setZeroNeg)
	case 0xbc: // LDY absolute,x
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.setRegOp(4+cycles, 3, &emu.Y, emu.read(addr), emu.setZeroNeg)
	case 0xbd: // LDA absolute,x
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.setRegOp(4+cycles, 3, &emu.A, emu.read(addr), emu.setZeroNeg)
	case 0xbe: // LDX absolute,y
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.setRegOp(4+cycles, 3, &emu.X, emu.read(addr), emu.setZeroNeg)

	case 0xc0: // CPY imm
		addr := emu.PC + 1
		emu.cmpOp(2, 2, emu.Y, emu.read(addr))
	case 0xc1: // CMP (indirect,x)
		addr := emu.getXPreIndexedAddr()
		emu.cmpOp(6, 2, emu.A, emu.read(addr))
	case 0xc2: // 2-nop (UNDOCUMENTED)
		emu.opFn(2, 2, emu.undocumentedOpcode)
	case 0xc4: // CPY zeropage
		addr := emu.getZeroPageAddr()
		emu.cmpOp(3, 2, emu.Y, emu.read(addr))
	case 0xc5: // CMP zeropage
		addr := emu.getZeroPageAddr()
		emu.cmpOp(3, 2, emu.A, emu.read(addr))
	case 0xc6: // DEC zeropage
		addr := emu.getZeroPageAddr()
		emu.storeOp(5, 2, addr, emu.read(addr)-1, emu.setZeroNeg)
	case 0xc8: // INY
		emu.setRegOp(2, 1, &emu.Y, emu.Y+1, emu.setZeroNeg)
	case 0xc9: // CMP imm
		addr := emu.PC + 1
		emu.cmpOp(2, 2, emu.A, emu.read(addr))
	case 0xca: // DEX
		emu.setRegOp(2, 1, &emu.X, emu.X-1, emu.setZeroNeg)
	case 0xcc: // CPY imm
		addr := emu.getAbsoluteAddr()
		emu.cmpOp(4, 3, emu.Y, emu.read(addr))
	case 0xcd: // CMP abosolute
		addr := emu.getAbsoluteAddr()
		emu.cmpOp(4, 3, emu.A, emu.read(addr))
	case 0xce: // DEC absolute
		addr := emu.getAbsoluteAddr()
		emu.storeOp(6, 3, addr, emu.read(addr)-1, emu.setZeroNeg)

	case 0xd0: // BNE
		emu.branchOpRel(emu.P&flagZero == 0)
	case 0xd1: // CMP (indirect),y
		addr, cycles := emu.getYPostIndexedAddr()
		emu.cmpOp(5+cycles, 2, emu.A, emu.read(addr))
	case 0xd4: // 2-nop (UNDOCUMENTED)
		emu.opFn(4, 2, emu.undocumentedOpcode)
	case 0xd5: // CMP zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.cmpOp(4, 2, emu.A, emu.read(addr))
	case 0xd6: // DEC zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.storeOp(6, 2, addr, emu.read(addr)-1, emu.setZeroNeg)
	case 0xd8: // CLD
		emu.opFn(2, 1, func() { emu.P &^= flagDecimal })
	case 0xd9: // CMP absolute,y
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.cmpOp(4+cycles, 3, emu.A, emu.read(addr))
	case 0xda: // 1-nop (UNDOCUMENTED)
		emu.opFn(2, 1, emu.undocumentedOpcode)
	case 0xdc: // 3-nop (UNDOCUMENTED)
		_, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.opFn(4+cycles, 3, emu.undocumentedOpcode)
	case 0xdd: // CMP absolute,x
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.cmpOp(4+cycles, 3, emu.A, emu.read(addr))
	case 0xde: // DEC absolute,x
		addr, _ := emu.getIndexedAbsoluteAddr(emu.X)
		emu.storeOp(7, 3, addr, emu.read(addr)-1, emu.setZeroNeg)

	case 0xe0: // CPX imm
		addr := emu.PC + 1
		emu.cmpOp(2, 2, emu.X, emu.read(addr))
	case 0xe1: // SBC (indirect,x)
		addr := emu.getXPreIndexedAddr()
		emu.opFn(6, 2, func() { emu.A = emu.sbcAndSetFlags(emu.read(addr)) })
	case 0xe2: // 2-nop (UNDOCUMENTED)
		emu.opFn(2, 2, emu.undocumentedOpcode)
	case 0xe5: // SBC zeropage
		addr := emu.getZeroPageAddr()
		emu.opFn(3, 2, func() { emu.A = emu.sbcAndSetFlags(emu.read(addr)) })
	case 0xe4: // CPX zeropage
		addr := emu.getZeroPageAddr()
		emu.cmpOp(3, 2, emu.X, emu.read(addr))
	case 0xe6: // INC zeropage
		addr := emu.getZeroPageAddr()
		emu.storeOp(5, 2, addr, emu.read(addr)+1, emu.setZeroNeg)
	case 0xe9: // SBC imm
		val := emu.read(emu.PC + 1)
		emu.opFn(2, 2, func() { emu.A = emu.sbcAndSetFlags(val) })
	case 0xe8: // INX
		emu.setRegOp(2, 1, &emu.X, emu.X+1, emu.setZeroNeg)
	case 0xea: // NOP
		emu.opFn(2, 1, func() {})
	case 0xeb: // sbc-alt imm (UNDOCUMENTED)
		val := emu.read(emu.PC + 1)
		emu.opFn(2, 2, func() { emu.A = emu.sbcAndSetFlags(val) })
	case 0xec: // CPX absolute
		addr := emu.getAbsoluteAddr()
		emu.cmpOp(4, 3, emu.X, emu.read(addr))
	case 0xed: // SBC absolute
		addr := emu.getAbsoluteAddr()
		emu.opFn(4, 3, func() { emu.A = emu.sbcAndSetFlags(emu.read(addr)) })
	case 0xee: // INC absolute
		addr := emu.getAbsoluteAddr()
		emu.storeOp(6, 3, addr, emu.read(addr)+1, emu.setZeroNeg)

	case 0xf0: // BEQ
		emu.branchOpRel(emu.P&flagZero == flagZero)
	case 0xf1: // SBC (indirect),y
		addr, cycles := emu.getYPostIndexedAddr()
		emu.opFn(5+cycles, 2, func() { emu.A = emu.sbcAndSetFlags(emu.read(addr)) })
	case 0xf4: // 2-nop (UNDOCUMENTED)
		emu.opFn(4, 2, emu.undocumentedOpcode)
	case 0xf5: // SBC zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.opFn(4, 2, func() { emu.A = emu.sbcAndSetFlags(emu.read(addr)) })
	case 0xf6: // INC zeropage,x
		addr := emu.getIndexedZeroPageAddr(emu.X)
		emu.storeOp(6, 2, addr, emu.read(addr)+1, emu.setZeroNeg)
	case 0xf8: // SED
		emu.opFn(2, 1, func() { emu.P |= flagDecimal })
	case 0xf9: // SBC absolute,y
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.Y)
		emu.opFn(4+cycles, 3, func() { emu.A = emu.sbcAndSetFlags(emu.read(addr)) })
	case 0xfa: // 1-nop (UNDOCUMENTED)
		emu.opFn(2, 1, emu.undocumentedOpcode)
	case 0xfc: // 3-nop (UNDOCUMENTED)
		_, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.opFn(4+cycles, 3, emu.undocumentedOpcode)
	case 0xfd: // SBC absolute,x
		addr, cycles := emu.getIndexedAbsoluteAddr(emu.X)
		emu.opFn(4+cycles, 3, func() { emu.A = emu.sbcAndSetFlags(emu.read(addr)) })
	case 0xfe: // INC absolute,x
		addr, _ := emu.getIndexedAbsoluteAddr(emu.X)
		emu.storeOp(7, 3, addr, emu.read(addr)+1, emu.setZeroNeg)

	default:
		emuErr(fmt.Sprintf("unimplemented opcode 0x%02x", opcode))
	}
}

func (emu *emuState) adcAndSetFlags(val byte) byte {
	var bigResult, i8Result int
	if emu.P&flagCarry == flagCarry {
		bigResult = int(emu.A) + int(val) + 1
		i8Result = int(int8(emu.A)) + int(int8(val)) + 1
	} else {
		bigResult = int(emu.A) + int(val)
		i8Result = int(int8(emu.A)) + int(int8(val))
	}
	result := byte(bigResult)
	emu.setOverflowFlag(i8Result < -128 || i8Result > 127)
	emu.setCarryFlag(bigResult > 0xff)
	emu.setZeroNeg(result)
	return result
}

func (emu *emuState) sbcAndSetFlags(val byte) byte {
	var bigResult, i8Result int
	if emu.P&flagCarry == 0 { // sbc uses carry's complement
		bigResult = int(emu.A) - int(val) - 1
		i8Result = int(int8(emu.A)) - int(int8(val)) - 1
	} else {
		bigResult = int(emu.A) - int(val)
		i8Result = int(int8(emu.A)) - int(int8(val))
	}
	result := byte(bigResult)
	emu.setOverflowFlag(i8Result < -128 || i8Result > 127)
	emu.setCarryFlag(bigResult >= 0) // once again, set to "add carry"'s complement
	emu.setZeroNeg(result)
	return result
}

func (emu *emuState) setOverflowFlag(test bool) {
	if test {
		emu.P |= flagOverflow
	} else {
		emu.P &^= flagOverflow
	}
}

func (emu *emuState) setCarryFlag(test bool) {
	if test {
		emu.P |= flagCarry
	} else {
		emu.P &^= flagCarry
	}
}

func (emu *emuState) setZeroFlag(test bool) {
	if test {
		emu.P |= flagZero
	} else {
		emu.P &^= flagZero
	}
}

func (emu *emuState) aslAndSetFlags(val byte) byte {
	result := val << 1
	emu.setCarryFlag(val&0x80 == 0x80)
	emu.setZeroNeg(result)
	return result
}

func (emu *emuState) lsrAndSetFlags(val byte) byte {
	result := val >> 1
	emu.setCarryFlag(val&0x01 == 0x01)
	emu.setZeroNeg(result)
	return result
}

func (emu *emuState) rorAndSetFlags(val byte) byte {
	result := val >> 1
	if emu.P&flagCarry == flagCarry {
		result |= 0x80
	}
	emu.setCarryFlag(val&0x01 == 0x01)
	emu.setZeroNeg(result)
	return result
}

func (emu *emuState) rolAndSetFlags(val byte) byte {
	result := val << 1
	if emu.P&flagCarry == flagCarry {
		result |= 0x01
	}
	emu.setCarryFlag(val&0x80 == 0x80)
	emu.setZeroNeg(result)
	return result
}

func (emu *emuState) bitAndSetFlags(val byte) {
	emu.P &^= 0xC0
	emu.P |= val & 0xC0
	emu.setZeroFlag(emu.A&val == 0)
}

func (emu *emuState) cmpOp(nCycles uint, instLen uint16, reg byte, val byte) {
	emu.runCycles(nCycles)
	emu.PC += instLen
	emu.setZeroNeg(reg - val)
	emu.setCarryFlag(reg >= val)
}

func (emu *emuState) jmpOp(nCycles uint, instLen uint16, newPC uint16) {
	emu.runCycles(nCycles)
	emu.PC = newPC
}

func (emu *emuState) branchOpRel(test bool) {
	if test {
		offs := int8(emu.read(emu.PC + 1))
		newPC := uint16(int(emu.PC+2) + int(offs))
		if newPC&0xff00 != emu.PC&0xff00 {
			emu.runCycles(4)
		} else {
			emu.runCycles(3)
		}
		emu.PC = newPC
	} else {
		emu.opFn(2, 2, func() {})
	}
}
