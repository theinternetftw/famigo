package famigo

import "fmt"

func (cs *cpuState) opFn(cycles uint, instLen uint16, fn func()) {
	fn()
	cs.PC += instLen
	cs.runCycles(cycles)
}

func (cs *cpuState) setZeroNeg(val byte) {
	if val == 0 {
		cs.P |= flagZero
	} else {
		cs.P &^= flagZero
	}
	if val&0x80 == 0x80 {
		cs.P |= flagNeg
	} else {
		cs.P &^= flagNeg
	}
}

func (cs *cpuState) setRegOp(numCycles uint, instLen uint16, dst *byte, src byte, flagFn func(byte)) {
	*dst = src
	cs.PC += instLen
	cs.runCycles(numCycles)
	flagFn(*dst)
}
func (cs *cpuState) storeOp(numCycles uint, instLen uint16, addr uint16, val byte, flagFn func(byte)) {
	cs.write(addr, val)
	cs.PC += instLen
	cs.runCycles(numCycles)
	flagFn(val)
}

func (cs *cpuState) setNoFlags(val byte) {}

// cs.PC must be in right place, obviously
func (cs *cpuState) getYPostIndexedAddr() (uint16, uint) {
	zPageLowAddr := uint16(cs.read(cs.PC + 1))
	zPageHighAddr := uint16(cs.read(cs.PC+1) + 1) // wraps at 0xff
	baseAddr := (uint16(cs.read(zPageHighAddr)) << 8) | uint16(cs.read(zPageLowAddr))
	addr := baseAddr + uint16(cs.Y)
	if addr&0xff00 != baseAddr&0xff00 { // if not same page, takes extra cycle
		return addr, 1
	}
	return addr, 0
}
func (cs *cpuState) getXPreIndexedAddr() uint16 {
	zPageLowAddr := uint16(cs.read(cs.PC+1) + cs.X)      // wraps at 0xff
	zPageHighAddr := uint16(cs.read(cs.PC+1) + cs.X + 1) // wraps at 0xff
	return (uint16(cs.read(zPageHighAddr)) << 8) | uint16(cs.read(zPageLowAddr))
}
func (cs *cpuState) getZeroPageAddr() uint16 {
	return uint16(cs.read(cs.PC + 1))
}
func (cs *cpuState) getIndexedZeroPageAddr(idx byte) uint16 {
	return uint16(cs.read(cs.PC+1) + idx) // wraps at 0xff
}
func (cs *cpuState) getAbsoluteAddr() uint16 {
	return cs.read16(cs.PC + 1)
}
func (cs *cpuState) getIndexedAbsoluteAddr(idx byte) (uint16, uint) {
	base := cs.read16(cs.PC + 1)
	addr := base + uint16(idx)
	if base&0xff00 != addr&0xff00 { // if not same page, takes extra cycle
		return addr, 1
	}
	return addr, 0
}
func (cs *cpuState) getIndirectJmpAddr() uint16 {
	// hw bug! similar to other indexing wrapping issues...
	operandAddr := cs.getAbsoluteAddr()
	highAddr := (operandAddr & 0xff00) | ((operandAddr + 1) & 0xff) // lo-byte wraps at 0xff
	return (uint16(cs.read(highAddr)) << 8) | uint16(cs.read(operandAddr))
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

func (cs *cpuState) undocumentedOpcode() {
	if crashOnUndocumentOpcode {
		emuErr(fmt.Sprintf("Undocumented opcode 0x%02x at 0x%04x", cs.read(cs.PC), cs.PC))
	}
}

func (cs *cpuState) stepOpcode() {

	opcode := cs.read(cs.PC)
	switch opcode {
	case 0x00: // BRK
		cs.opFn(7, 1, func() { cs.BRK = true })
	case 0x01: // ORA (indirect,x)
		addr := cs.getXPreIndexedAddr()
		cs.setRegOp(6, 2, &cs.A, cs.A|cs.read(addr), cs.setZeroNeg)
	case 0x04: // 2-nop (UNDOCUMENTED)
		cs.opFn(3, 2, cs.undocumentedOpcode)
	case 0x05: // ORA zeropage
		addr := cs.getZeroPageAddr()
		cs.setRegOp(3, 2, &cs.A, cs.A|cs.read(addr), cs.setZeroNeg)
	case 0x06: // ASL zeropage
		addr := cs.getZeroPageAddr()
		cs.storeOp(5, 2, addr, cs.aslAndSetFlags(cs.read(addr)), cs.setNoFlags)
	case 0x08: // PHP
		cs.opFn(3, 1, func() { cs.push(cs.P | flagOnStack | flagBrk) })
	case 0x09: // ORA imm
		addr := cs.PC + 1
		cs.setRegOp(2, 2, &cs.A, cs.A|cs.read(addr), cs.setZeroNeg)
	case 0x0a: // ASL A
		cs.opFn(2, 1, func() { cs.A = cs.aslAndSetFlags(cs.A) })
	case 0x0c: // 3-nop (UNDOCUMENTED)
		cs.opFn(4, 3, cs.undocumentedOpcode)
	case 0x0d: // ORA absolute
		addr := cs.getAbsoluteAddr()
		cs.setRegOp(4, 3, &cs.A, cs.A|cs.read(addr), cs.setZeroNeg)
	case 0x0e: // ASL absolute
		addr := cs.getAbsoluteAddr()
		cs.storeOp(6, 3, addr, cs.aslAndSetFlags(cs.read(addr)), cs.setNoFlags)

	case 0x10: // BPL
		cs.branchOpRel(cs.P&flagNeg == 0)
	case 0x11: // ORA (indirect),y
		addr, cycles := cs.getYPostIndexedAddr()
		cs.setRegOp(5+cycles, 2, &cs.A, cs.A|cs.read(addr), cs.setZeroNeg)
	case 0x14: // 2-nop (UNDOCUMENTED)
		cs.opFn(4, 2, cs.undocumentedOpcode)
	case 0x15: // ORA zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.setRegOp(4, 2, &cs.A, cs.A|cs.read(addr), cs.setZeroNeg)
	case 0x16: // ASL zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(6, 2, addr, cs.aslAndSetFlags(cs.read(addr)), cs.setNoFlags)
	case 0x18: // CLC
		cs.opFn(2, 1, func() { cs.P &^= flagCarry })
	case 0x19: // ORA absolute,y
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.setRegOp(4+cycles, 3, &cs.A, cs.A|cs.read(addr), cs.setZeroNeg)
	case 0x1a: // 1-nop (UNDOCUMENTED)
		cs.opFn(2, 1, cs.undocumentedOpcode)
	case 0x1c: // 3-nop (UNDOCUMENTED)
		_, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.opFn(4+cycles, 3, cs.undocumentedOpcode)
	case 0x1d: // ORA absolute,x
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.setRegOp(4+cycles, 3, &cs.A, cs.A|cs.read(addr), cs.setZeroNeg)
	case 0x1e: // ASL absolute,x
		addr, _ := cs.getIndexedAbsoluteAddr(cs.X)
		cs.storeOp(7, 3, addr, cs.aslAndSetFlags(cs.read(addr)), cs.setNoFlags)

	case 0x20: // JSR (jump and store return addr)
		cs.push16(cs.PC + 2)
		cs.jmpOp(6, 3, cs.getAbsoluteAddr())
	case 0x21: // AND (indirect,x)
		addr := cs.getXPreIndexedAddr()
		cs.setRegOp(6, 2, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x24: // BIT zeropage
		addr := cs.getZeroPageAddr()
		cs.opFn(3, 2, func() { cs.bitAndSetFlags(cs.read(addr)) })
	case 0x25: // AND zeropage
		addr := cs.getZeroPageAddr()
		cs.setRegOp(3, 2, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x26: // ROL zeropage
		addr := cs.getZeroPageAddr()
		cs.storeOp(5, 2, addr, cs.rolAndSetFlags(cs.read(addr)), cs.setNoFlags)
	case 0x28: // PLP
		flags := cs.pop() &^ (flagBrk | flagOnStack)
		cs.setRegOp(4, 1, &cs.P, flags, cs.setNoFlags)
	case 0x29: // AND imm
		addr := cs.PC + 1
		cs.setRegOp(2, 2, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x2a: // ROL A
		cs.opFn(2, 1, func() { cs.A = cs.rolAndSetFlags(cs.A) })
	case 0x2c: // BIT absolute
		addr := cs.getAbsoluteAddr()
		cs.opFn(4, 3, func() { cs.bitAndSetFlags(cs.read(addr)) })
	case 0x2d: // AND absolute
		addr := cs.getAbsoluteAddr()
		cs.setRegOp(4, 3, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x2e: // ROL absolute
		addr := cs.getAbsoluteAddr()
		cs.storeOp(6, 3, addr, cs.rolAndSetFlags(cs.read(addr)), cs.setNoFlags)

	case 0x30: // BMI
		cs.branchOpRel(cs.P&flagNeg == flagNeg)
	case 0x31: // AND (indirect),y
		addr, cycles := cs.getYPostIndexedAddr()
		cs.setRegOp(5+cycles, 2, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x34: // 2-nop (UNDOCUMENTED)
		cs.opFn(4, 2, cs.undocumentedOpcode)
	case 0x35: // AND zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.setRegOp(4, 2, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x36: // ROL zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(6, 2, addr, cs.rolAndSetFlags(cs.read(addr)), cs.setNoFlags)
	case 0x38: // SEC
		cs.opFn(2, 1, func() { cs.P |= flagCarry })
	case 0x39: // AND absolute,y
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.setRegOp(4+cycles, 3, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x3a: // 1-nop (UNDOCUMENTED)
		cs.opFn(2, 1, cs.undocumentedOpcode)
	case 0x3c: // 3-nop (UNDOCUMENTED)
		_, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.opFn(4+cycles, 3, cs.undocumentedOpcode)
	case 0x3d: // AND absolute,x
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.setRegOp(4+cycles, 3, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x3e: // ROL absolute,x
		addr, _ := cs.getIndexedAbsoluteAddr(cs.X)
		cs.storeOp(7, 3, addr, cs.rolAndSetFlags(cs.read(addr)), cs.setNoFlags)

	case 0x40: // RTI (return from interrupt)
		cs.P = cs.pop() &^ (flagBrk | flagOnStack)
		cs.opFn(6, 0, func() { cs.PC = cs.pop16() }) // real instLen 1, but we don't want to step past newPC (unlike RTS)
	case 0x41: // EOR (indirect,x)
		addr := cs.getXPreIndexedAddr()
		cs.setRegOp(6, 2, &cs.A, cs.A^cs.read(addr), cs.setZeroNeg)
	case 0x44: // 2-nop (UNDOCUMENTED)
		cs.opFn(3, 2, cs.undocumentedOpcode)
	case 0x45: // EOR zeropage
		addr := cs.getZeroPageAddr()
		cs.setRegOp(3, 2, &cs.A, cs.A^cs.read(addr), cs.setZeroNeg)
	case 0x46: // LSR zeropage
		addr := cs.getZeroPageAddr()
		cs.storeOp(5, 2, addr, cs.lsrAndSetFlags(cs.read(addr)), cs.setNoFlags)
	case 0x48: // PHA
		cs.opFn(3, 1, func() { cs.push(cs.A) })
	case 0x49: // EOR imm
		addr := cs.PC + 1
		cs.setRegOp(2, 2, &cs.A, cs.A^cs.read(addr), cs.setZeroNeg)
	case 0x4a: // LSR A
		cs.opFn(2, 1, func() { cs.A = cs.lsrAndSetFlags(cs.A) })
	case 0x4c: // JMP absolute
		cs.jmpOp(3, 3, cs.getAbsoluteAddr())
	case 0x4d: // EOR absolute
		addr := cs.getAbsoluteAddr()
		cs.setRegOp(4, 3, &cs.A, cs.A^cs.read(addr), cs.setZeroNeg)
	case 0x4e: // LSR absolute
		addr := cs.getAbsoluteAddr()
		cs.storeOp(6, 3, addr, cs.lsrAndSetFlags(cs.read(addr)), cs.setNoFlags)

	case 0x50: // BVC
		cs.branchOpRel(cs.P&flagOverflow == 0)
	case 0x51: // EOR (indirect),y
		addr, cycles := cs.getYPostIndexedAddr()
		cs.setRegOp(5+cycles, 2, &cs.A, cs.A^cs.read(addr), cs.setZeroNeg)
	case 0x54: // 2-nop (UNDOCUMENTED)
		cs.opFn(4, 2, cs.undocumentedOpcode)
	case 0x55: // EOR zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.setRegOp(4, 2, &cs.A, cs.A^cs.read(addr), cs.setZeroNeg)
	case 0x56: // LSR zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(6, 2, addr, cs.lsrAndSetFlags(cs.read(addr)), cs.setNoFlags)
	case 0x58: // CLI
		cs.opFn(2, 1, func() { cs.P &^= flagIrqDisabled })
	case 0x59: // EOR absolute,y
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.setRegOp(4+cycles, 3, &cs.A, cs.A^cs.read(addr), cs.setZeroNeg)
	case 0x5a: // 1-nop (UNDOCUMENTED)
		cs.opFn(2, 1, cs.undocumentedOpcode)
	case 0x5c: // 3-nop (UNDOCUMENTED)
		_, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.opFn(4+cycles, 3, cs.undocumentedOpcode)
	case 0x5d: // EOR absolute,x
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.setRegOp(4+cycles, 3, &cs.A, cs.A^cs.read(addr), cs.setZeroNeg)
	case 0x5e: // LSR absolute,x
		addr, _ := cs.getIndexedAbsoluteAddr(cs.X)
		cs.storeOp(7, 3, addr, cs.lsrAndSetFlags(cs.read(addr)), cs.setNoFlags)

	case 0x60: // RTS (return from subroutine)
		cs.opFn(6, 1, func() { cs.PC = cs.pop16() }) // opFn adds 1 to PC, so does real 6502
	case 0x61: // ADC (indirect,x)
		addr := cs.getXPreIndexedAddr()
		cs.opFn(6, 2, func() { cs.A = cs.adcAndSetFlags(cs.read(addr)) })
	case 0x64: // 2-nop (UNDOCUMENTED)
		cs.opFn(3, 2, cs.undocumentedOpcode)
	case 0x65: // ADC zeropage
		addr := cs.getZeroPageAddr()
		cs.opFn(3, 2, func() { cs.A = cs.adcAndSetFlags(cs.read(addr)) })
	case 0x66: // ROR zeropage
		addr := cs.getZeroPageAddr()
		cs.storeOp(5, 2, addr, cs.rorAndSetFlags(cs.read(addr)), cs.setNoFlags)
	case 0x68: // PLA
		cs.setRegOp(4, 1, &cs.A, cs.pop(), cs.setZeroNeg)
	case 0x69: // ADC imm
		addr := cs.PC + 1
		cs.opFn(2, 2, func() { cs.A = cs.adcAndSetFlags(cs.read(addr)) })
	case 0x6a: // ROR A
		cs.opFn(2, 1, func() { cs.A = cs.rorAndSetFlags(cs.A) })
	case 0x6c: // JMP (indirect)
		cs.jmpOp(5, 3, cs.getIndirectJmpAddr())
	case 0x6d: // ADC absolute
		addr := cs.getAbsoluteAddr()
		cs.opFn(4, 3, func() { cs.A = cs.adcAndSetFlags(cs.read(addr)) })
	case 0x6e: // ROR absolute
		addr := cs.getAbsoluteAddr()
		cs.storeOp(6, 3, addr, cs.rorAndSetFlags(cs.read(addr)), cs.setNoFlags)

	case 0x70: // BVS
		cs.branchOpRel(cs.P&flagOverflow == flagOverflow)
	case 0x71: // ADC (indirect),y
		addr, cycles := cs.getYPostIndexedAddr()
		cs.opFn(4+cycles, 2, func() { cs.A = cs.adcAndSetFlags(cs.read(addr)) })
	case 0x74: // 2-nop (UNDOCUMENTED)
		cs.opFn(4, 2, cs.undocumentedOpcode)
	case 0x75: // ADC zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.opFn(4, 2, func() { cs.A = cs.adcAndSetFlags(cs.read(addr)) })
	case 0x76: // ROR zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(6, 2, addr, cs.rorAndSetFlags(cs.read(addr)), cs.setNoFlags)
	case 0x78: // SEI
		cs.opFn(2, 1, func() { cs.P |= flagIrqDisabled })
	case 0x79: // ADC absolute,y
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.opFn(4+cycles, 3, func() { cs.A = cs.adcAndSetFlags(cs.read(addr)) })
	case 0x7a: // 1-nop (UNDOCUMENTED)
		cs.opFn(2, 1, cs.undocumentedOpcode)
	case 0x7c: // 3-nop (UNDOCUMENTED)
		_, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.opFn(4+cycles, 3, cs.undocumentedOpcode)
	case 0x7d: // ADC absolute,x
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.opFn(4+cycles, 3, func() { cs.A = cs.adcAndSetFlags(cs.read(addr)) })
	case 0x7e: // ROR absolute,x
		addr, _ := cs.getIndexedAbsoluteAddr(cs.X)
		cs.storeOp(7, 3, addr, cs.rorAndSetFlags(cs.read(addr)), cs.setNoFlags)

	case 0x80: // 2-nop (UNDOCUMENTED)
		cs.opFn(2, 2, cs.undocumentedOpcode)
	case 0x81: // STA (indirect,x)
		cs.storeOp(6, 2, cs.getXPreIndexedAddr(), cs.A, cs.setNoFlags)
	case 0x82: // 2-nop (UNDOCUMENTED)
		cs.opFn(2, 2, cs.undocumentedOpcode)
	case 0x84: // STY zeropage
		cs.storeOp(3, 2, cs.getZeroPageAddr(), cs.Y, cs.setNoFlags)
	case 0x85: // STA zeropage
		cs.storeOp(3, 2, cs.getZeroPageAddr(), cs.A, cs.setNoFlags)
	case 0x86: // STX zeropage
		cs.storeOp(3, 2, cs.getZeroPageAddr(), cs.X, cs.setNoFlags)
	case 0x88: // DEY
		cs.setRegOp(2, 1, &cs.Y, cs.Y-1, cs.setZeroNeg)
	case 0x89: // 2-nop (UNDOCUMENTED)
		cs.opFn(2, 2, cs.undocumentedOpcode)
	case 0x8a: // TXA
		cs.setRegOp(2, 1, &cs.A, cs.X, cs.setZeroNeg)
	case 0x8c: // STY absolute
		cs.storeOp(4, 3, cs.getAbsoluteAddr(), cs.Y, cs.setNoFlags)
	case 0x8d: // STA absolute
		cs.storeOp(4, 3, cs.getAbsoluteAddr(), cs.A, cs.setNoFlags)
	case 0x8e: // STX absolute
		cs.storeOp(4, 3, cs.getAbsoluteAddr(), cs.X, cs.setNoFlags)

	case 0x90: // BCC
		cs.branchOpRel(cs.P&flagCarry == 0)
	case 0x91: // STA (indirect),y
		addr, _ := cs.getYPostIndexedAddr()
		cs.storeOp(6, 2, addr, cs.A, cs.setNoFlags)
	case 0x94: // STY zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(4, 2, addr, cs.Y, cs.setNoFlags)
	case 0x95: // STA zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(4, 2, addr, cs.A, cs.setNoFlags)
	case 0x96: // STX zeropage,y
		addr := cs.getIndexedZeroPageAddr(cs.Y)
		cs.storeOp(4, 2, addr, cs.X, cs.setNoFlags)
	case 0x98: // TYA
		cs.setRegOp(2, 1, &cs.A, cs.Y, cs.setZeroNeg)
	case 0x99: // STA absolute,y
		addr, _ := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.storeOp(5, 3, addr, cs.A, cs.setNoFlags)
	case 0x9a: // TXS
		cs.setRegOp(2, 1, &cs.S, cs.X, cs.setNoFlags)
	case 0x9d: // STA absolute,x
		addr, _ := cs.getIndexedAbsoluteAddr(cs.X)
		cs.storeOp(5, 3, addr, cs.A, cs.setNoFlags)

	case 0xa0: // LDY imm
		addr := cs.PC + 1
		cs.setRegOp(2, 2, &cs.Y, cs.read(addr), cs.setZeroNeg)
	case 0xa1: // LDA (indirect,x)
		addr := cs.getXPreIndexedAddr()
		cs.setRegOp(6, 2, &cs.A, cs.read(addr), cs.setZeroNeg)
	case 0xa2: // LDX imm
		addr := cs.PC + 1
		cs.setRegOp(2, 2, &cs.X, cs.read(addr), cs.setZeroNeg)
	case 0xa4: // LDY zeropage
		addr := cs.getZeroPageAddr()
		cs.setRegOp(3, 2, &cs.Y, cs.read(addr), cs.setZeroNeg)
	case 0xa5: // LDA zeropage
		addr := cs.getZeroPageAddr()
		cs.setRegOp(3, 2, &cs.A, cs.read(addr), cs.setZeroNeg)
	case 0xa6: // LDX zeropage
		addr := cs.getZeroPageAddr()
		cs.setRegOp(3, 2, &cs.X, cs.read(addr), cs.setZeroNeg)
	case 0xa8: // TAY
		cs.setRegOp(2, 1, &cs.Y, cs.A, cs.setZeroNeg)
	case 0xa9: // LDA imm
		addr := cs.PC + 1
		cs.setRegOp(2, 2, &cs.A, cs.read(addr), cs.setZeroNeg)
	case 0xaa: // TAX
		cs.setRegOp(2, 1, &cs.X, cs.A, cs.setZeroNeg)
	case 0xac: // LDY absolute
		addr := cs.getAbsoluteAddr()
		cs.setRegOp(4, 3, &cs.Y, cs.read(addr), cs.setZeroNeg)
	case 0xad: // LDA absolute
		addr := cs.getAbsoluteAddr()
		cs.setRegOp(4, 3, &cs.A, cs.read(addr), cs.setZeroNeg)
	case 0xae: // LDX absolute
		addr := cs.getAbsoluteAddr()
		cs.setRegOp(4, 3, &cs.X, cs.read(addr), cs.setZeroNeg)

	case 0xb0: // BCS
		cs.branchOpRel(cs.P&flagCarry == flagCarry)
	case 0xb1: // LDA (indirect),y
		addr, cycles := cs.getYPostIndexedAddr()
		cs.setRegOp(5+cycles, 2, &cs.A, cs.read(addr), cs.setZeroNeg)
	case 0xb4: // LDY zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.setRegOp(4, 2, &cs.Y, cs.read(addr), cs.setZeroNeg)
	case 0xb5: // LDA zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.setRegOp(4, 2, &cs.A, cs.read(addr), cs.setZeroNeg)
	case 0xb6: // LDX zeropage,y
		addr := cs.getIndexedZeroPageAddr(cs.Y)
		cs.setRegOp(4, 2, &cs.X, cs.read(addr), cs.setZeroNeg)
	case 0xb8: // CLV
		cs.opFn(2, 1, func() { cs.P &^= flagOverflow })
	case 0xb9: // LDA absolute, y
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.setRegOp(4+cycles, 3, &cs.A, cs.read(addr), cs.setZeroNeg)
	case 0xba: // TSX
		cs.setRegOp(2, 1, &cs.X, cs.S, cs.setZeroNeg)
	case 0xbc: // LDY absolute,x
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.setRegOp(4+cycles, 3, &cs.Y, cs.read(addr), cs.setZeroNeg)
	case 0xbd: // LDA absolute,x
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.setRegOp(4+cycles, 3, &cs.A, cs.read(addr), cs.setZeroNeg)
	case 0xbe: // LDX absolute,y
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.setRegOp(4+cycles, 3, &cs.X, cs.read(addr), cs.setZeroNeg)

	case 0xc0: // CPY imm
		addr := cs.PC + 1
		cs.cmpOp(2, 2, cs.Y, cs.read(addr))
	case 0xc1: // CMP (indirect,x)
		addr := cs.getXPreIndexedAddr()
		cs.cmpOp(6, 2, cs.A, cs.read(addr))
	case 0xc2: // 2-nop (UNDOCUMENTED)
		cs.opFn(2, 2, cs.undocumentedOpcode)
	case 0xc4: // CPY zeropage
		addr := cs.getZeroPageAddr()
		cs.cmpOp(3, 2, cs.Y, cs.read(addr))
	case 0xc5: // CMP zeropage
		addr := cs.getZeroPageAddr()
		cs.cmpOp(3, 2, cs.A, cs.read(addr))
	case 0xc6: // DEC zeropage
		addr := cs.getZeroPageAddr()
		cs.storeOp(5, 2, addr, cs.read(addr)-1, cs.setZeroNeg)
	case 0xc8: // INY
		cs.setRegOp(2, 1, &cs.Y, cs.Y+1, cs.setZeroNeg)
	case 0xc9: // CMP imm
		addr := cs.PC + 1
		cs.cmpOp(2, 2, cs.A, cs.read(addr))
	case 0xca: // DEX
		cs.setRegOp(2, 1, &cs.X, cs.X-1, cs.setZeroNeg)
	case 0xcc: // CPY imm
		addr := cs.getAbsoluteAddr()
		cs.cmpOp(4, 3, cs.Y, cs.read(addr))
	case 0xcd: // CMP abosolute
		addr := cs.getAbsoluteAddr()
		cs.cmpOp(4, 3, cs.A, cs.read(addr))
	case 0xce: // DEC absolute
		addr := cs.getAbsoluteAddr()
		cs.storeOp(6, 3, addr, cs.read(addr)-1, cs.setZeroNeg)

	case 0xd0: // BNE
		cs.branchOpRel(cs.P&flagZero == 0)
	case 0xd1: // CMP (indirect),y
		addr, cycles := cs.getYPostIndexedAddr()
		cs.cmpOp(5+cycles, 2, cs.A, cs.read(addr))
	case 0xd4: // 2-nop (UNDOCUMENTED)
		cs.opFn(4, 2, cs.undocumentedOpcode)
	case 0xd5: // CMP zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.cmpOp(4, 2, cs.A, cs.read(addr))
	case 0xd6: // DEC zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(6, 2, addr, cs.read(addr)-1, cs.setZeroNeg)
	case 0xd8: // CLD
		cs.opFn(2, 1, func() { cs.P &^= flagDecimal })
	case 0xd9: // CMP absolute,y
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.cmpOp(4+cycles, 3, cs.A, cs.read(addr))
	case 0xda: // 1-nop (UNDOCUMENTED)
		cs.opFn(2, 1, cs.undocumentedOpcode)
	case 0xdc: // 3-nop (UNDOCUMENTED)
		_, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.opFn(4+cycles, 3, cs.undocumentedOpcode)
	case 0xdd: // CMP absolute,x
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.cmpOp(4+cycles, 3, cs.A, cs.read(addr))
	case 0xde: // DEC absolute,x
		addr, _ := cs.getIndexedAbsoluteAddr(cs.X)
		cs.storeOp(7, 3, addr, cs.read(addr)-1, cs.setZeroNeg)

	case 0xe0: // CPX imm
		addr := cs.PC + 1
		cs.cmpOp(2, 2, cs.X, cs.read(addr))
	case 0xe1: // SBC (indirect,x)
		addr := cs.getXPreIndexedAddr()
		cs.opFn(6, 2, func() { cs.A = cs.sbcAndSetFlags(cs.read(addr)) })
	case 0xe2: // 2-nop (UNDOCUMENTED)
		cs.opFn(2, 2, cs.undocumentedOpcode)
	case 0xe5: // SBC zeropage
		addr := cs.getZeroPageAddr()
		cs.opFn(3, 2, func() { cs.A = cs.sbcAndSetFlags(cs.read(addr)) })
	case 0xe4: // CPX zeropage
		addr := cs.getZeroPageAddr()
		cs.cmpOp(2, 2, cs.X, cs.read(addr))
	case 0xe6: // INC zeropage
		addr := cs.getZeroPageAddr()
		cs.storeOp(5, 2, addr, cs.read(addr)+1, cs.setZeroNeg)
	case 0xe9: // SBC imm
		val := cs.read(cs.PC + 1)
		cs.opFn(2, 2, func() { cs.A = cs.sbcAndSetFlags(val) })
	case 0xe8: // INX
		cs.setRegOp(2, 1, &cs.X, cs.X+1, cs.setZeroNeg)
	case 0xea: // NOP
		cs.opFn(2, 1, func() {})
	case 0xec: // CPX absolute
		addr := cs.getAbsoluteAddr()
		cs.cmpOp(2, 3, cs.X, cs.read(addr))
	case 0xed: // SBC absolute
		addr := cs.getAbsoluteAddr()
		cs.opFn(4, 3, func() { cs.A = cs.sbcAndSetFlags(cs.read(addr)) })
	case 0xee: // INC absolute
		addr := cs.getAbsoluteAddr()
		cs.storeOp(6, 3, addr, cs.read(addr)+1, cs.setZeroNeg)

	case 0xf0: // BEQ
		cs.branchOpRel(cs.P&flagZero == flagZero)
	case 0xf1: // SBC (indirect),y
		addr, cycles := cs.getYPostIndexedAddr()
		cs.opFn(5+cycles, 2, func() { cs.A = cs.sbcAndSetFlags(cs.read(addr)) })
	case 0xf4: // 2-nop (UNDOCUMENTED)
		cs.opFn(4, 2, cs.undocumentedOpcode)
	case 0xf5: // SBC zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.opFn(4, 2, func() { cs.A = cs.sbcAndSetFlags(cs.read(addr)) })
	case 0xf6: // INC zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(6, 2, addr, cs.read(addr)+1, cs.setZeroNeg)
	case 0xf8: // SED
		cs.opFn(2, 1, func() { cs.P |= flagDecimal })
	case 0xf9: // SBC absolute,y
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.Y)
		cs.opFn(4+cycles, 3, func() { cs.A = cs.sbcAndSetFlags(cs.read(addr)) })
	case 0xfa: // 1-nop (UNDOCUMENTED)
		cs.opFn(2, 1, cs.undocumentedOpcode)
	case 0xfc: // 3-nop (UNDOCUMENTED)
		_, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.opFn(4+cycles, 3, cs.undocumentedOpcode)
	case 0xfd: // SBC absolute,x
		addr, cycles := cs.getIndexedAbsoluteAddr(cs.X)
		cs.opFn(4+cycles, 3, func() { cs.A = cs.sbcAndSetFlags(cs.read(addr)) })
	case 0xfe: // INC absolute,x
		addr, _ := cs.getIndexedAbsoluteAddr(cs.X)
		cs.storeOp(7, 3, addr, cs.read(addr)+1, cs.setZeroNeg)

	default:
		emuErr(fmt.Sprintf("unimplemented opcode 0x%02x", opcode))
	}
}

func (cs *cpuState) adcAndSetFlags(val byte) byte {
	var bigResult, i8Result int
	if cs.P&flagCarry == flagCarry {
		bigResult = int(cs.A) + int(val) + 1
		i8Result = int(int8(cs.A)) + int(int8(val)) + 1
	} else {
		bigResult = int(cs.A) + int(val)
		i8Result = int(int8(cs.A)) + int(int8(val))
	}
	result := byte(bigResult)
	cs.setOverflowFlag(i8Result < -128 || i8Result > 127)
	cs.setCarryFlag(bigResult > 0xff)
	cs.setZeroNeg(result)
	return result
}

func (cs *cpuState) sbcAndSetFlags(val byte) byte {
	var bigResult, i8Result int
	if cs.P&flagCarry == 0 { // sbc uses carry's complement
		bigResult = int(cs.A) - int(val) - 1
		i8Result = int(int8(cs.A)) - int(int8(val)) - 1
	} else {
		bigResult = int(cs.A) - int(val)
		i8Result = int(int8(cs.A)) - int(int8(val))
	}
	result := byte(bigResult)
	cs.setOverflowFlag(i8Result < -128 || i8Result > 127)
	cs.setCarryFlag(bigResult >= 0) // once again, set to "add carry"'s complement
	cs.setZeroNeg(result)
	return result
}

func (cs *cpuState) setOverflowFlag(test bool) {
	if test {
		cs.P |= flagOverflow
	} else {
		cs.P &^= flagOverflow
	}
}

func (cs *cpuState) setCarryFlag(test bool) {
	if test {
		cs.P |= flagCarry
	} else {
		cs.P &^= flagCarry
	}
}

func (cs *cpuState) setZeroFlag(test bool) {
	if test {
		cs.P |= flagZero
	} else {
		cs.P &^= flagZero
	}
}

func (cs *cpuState) aslAndSetFlags(val byte) byte {
	result := val << 1
	cs.setCarryFlag(val&0x80 == 0x80)
	cs.setZeroNeg(result)
	return result
}

func (cs *cpuState) lsrAndSetFlags(val byte) byte {
	result := val >> 1
	cs.setCarryFlag(val&0x01 == 0x01)
	cs.setZeroNeg(result)
	return result
}

func (cs *cpuState) rorAndSetFlags(val byte) byte {
	result := val >> 1
	if cs.P&flagCarry == flagCarry {
		result |= 0x80
	}
	cs.setCarryFlag(val&0x01 == 0x01)
	cs.setZeroNeg(result)
	return result
}

func (cs *cpuState) rolAndSetFlags(val byte) byte {
	result := val << 1
	if cs.P&flagCarry == flagCarry {
		result |= 0x01
	}
	cs.setCarryFlag(val&0x80 == 0x80)
	cs.setZeroNeg(result)
	return result
}

func (cs *cpuState) bitAndSetFlags(val byte) {
	cs.P &^= 0xC0
	cs.P |= val & 0xC0
	cs.setZeroFlag(cs.A&val == 0)
}

func (cs *cpuState) cmpOp(nCycles uint, instLen uint16, reg byte, val byte) {
	cs.runCycles(nCycles)
	cs.PC += instLen
	cs.setZeroNeg(reg - val)
	cs.setCarryFlag(reg >= val)
}

func (cs *cpuState) jmpOp(nCycles uint, instLen uint16, newPC uint16) {
	cs.runCycles(nCycles)
	cs.PC = newPC
}

func (cs *cpuState) branchOpRel(test bool) {
	if test {
		offs := int8(cs.read(cs.PC + 1))
		newPC := uint16(int(cs.PC+2) + int(offs))
		if newPC&0xff00 != cs.PC&0xff00 {
			cs.runCycles(4)
		} else {
			cs.runCycles(3)
		}
		cs.PC = newPC
	} else {
		cs.opFn(2, 2, func() {})
	}
}
