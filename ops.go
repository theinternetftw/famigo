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
func (cs *cpuState) getYPostIndexedAddr() uint16 {
	zPageLowAddr := uint16(cs.read(cs.PC + 1))
	zPageHighAddr := uint16(cs.read(cs.PC+1) + 1) // wraps at 0xff
	baseAddr := (uint16(cs.read(zPageHighAddr)) << 8) | uint16(cs.read(zPageLowAddr))
	return baseAddr + uint16(cs.Y)
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
func (cs *cpuState) getIndexedAbsoluteAddr(idx byte) uint16 {
	return uint16(cs.read16(cs.PC+1)) + uint16(idx)
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

const showMemAccesses = false

func (cs *cpuState) stepOpcode() {

	fmt.Println(cs.debugStatusLine())

	opcode := cs.read(cs.PC)
	switch opcode {
	case 0x00: // BRK
		cs.opFn(7, 1, func() { cs.BRK = true })
	case 0x05: // ORA (zeropage)
		result := cs.A | cs.read(cs.getZeroPageAddr())
		cs.setRegOp(3, 2, &cs.A, result, cs.setZeroNeg)
	case 0x06: // ASL (zeropage)
		addr := cs.getZeroPageAddr()
		result := cs.aslAndSetFlags(cs.read(addr))
		cs.storeOp(5, 2, addr, result, cs.setNoFlags)
	case 0x09: // ORA imm
		result := cs.A | cs.read(cs.PC+1)
		cs.setRegOp(2, 2, &cs.A, result, cs.setZeroNeg)
	case 0x0a: // ASL A
		cs.opFn(2, 1, func() { cs.A = cs.aslAndSetFlags(cs.A) })

	case 0x10: // BPL (branch if positive)
		cs.branchOpRel(cs.P&flagNeg == 0)
	case 0x18: // CLC (clear carry flag)
		cs.opFn(2, 1, func() { cs.P &^= flagCarry })
	case 0x19: // ORA absolute,y
		// TODO: add cycle when crossing page boundary
		result := cs.A | cs.read(cs.getIndexedAbsoluteAddr(cs.Y))
		cs.setRegOp(4, 3, &cs.A, result, cs.setZeroNeg)

	case 0x20: // JSR (jump and store return addr)
		cs.push16(cs.PC + 2)
		cs.jmpOp(6, 3, cs.getAbsoluteAddr())
	case 0x24: // BIT (zeropage)
		cs.opFn(3, 2, func() {
			val := cs.read(cs.getZeroPageAddr())
			cs.P &^= 0xC0
			cs.P |= val & 0xC0
			cs.setZeroFlag(cs.A^val == 0)
		})
	case 0x25: // AND (zeropage)
		result := cs.A & cs.read(cs.getZeroPageAddr())
		cs.setRegOp(3, 2, &cs.A, result, cs.setZeroNeg)
	case 0x28: // PLP (pull P from stack)
		flags := cs.pop() &^ (flagBrk | flagOnStack)
		cs.setRegOp(4, 1, &cs.P, flags, cs.setNoFlags)
	case 0x29: // AND imm
		result := cs.A & cs.read(cs.PC+1)
		cs.setRegOp(2, 2, &cs.A, result, cs.setZeroNeg)
	case 0x2a: // ROL A
		cs.opFn(2, 1, func() { cs.A = cs.rolAndSetFlags(cs.A) })

	case 0x30: // BMI (branch if negative)
		cs.branchOpRel(cs.P&flagNeg == flagNeg)
	case 0x35: // AND zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.setRegOp(4, 2, &cs.A, cs.A&cs.read(addr), cs.setZeroNeg)
	case 0x38: // SEC (set carry flag)
		cs.opFn(2, 1, func() { cs.P |= flagCarry })

	case 0x40: // RTI (return from interrupt)
		cs.P = cs.pop() &^ (flagBrk | flagOnStack)
		cs.opFn(6, 0, func() { cs.PC = cs.pop16() }) // real instLen 1, but we don't want to step past newPC (unlike RTS)
	case 0x45: // EOR (zeropage)
		result := cs.A ^ cs.read(cs.getZeroPageAddr())
		cs.setRegOp(3, 2, &cs.A, result, cs.setZeroNeg)
	case 0x46: // LSR (zeropage)
		result := cs.lsrAndSetFlags(cs.read(cs.getZeroPageAddr()))
		cs.storeOp(5, 2, cs.getZeroPageAddr(), result, cs.setNoFlags)
	case 0x48: // PHA (push A onto stack)
		cs.opFn(3, 1, func() { cs.push(cs.A) })
	case 0x49: // EOR imm (exclusive or)
		result := cs.A ^ cs.read(cs.PC+1)
		cs.setRegOp(2, 2, &cs.A, result, cs.setZeroNeg)
	case 0x4a: // LSR A
		cs.opFn(2, 1, func() { cs.A = cs.lsrAndSetFlags(cs.A) })
	case 0x4c: // JMP imm (absolute)
		cs.jmpOp(3, 3, cs.getAbsoluteAddr())

	case 0x60: // RTS (return from subroutine)
		cs.opFn(6, 1, func() { cs.PC = cs.pop16() }) // opFn adds 1 to PC, so does real 6502
	case 0x65: // ADC (zeropage)
		val := cs.read(cs.getZeroPageAddr())
		cs.opFn(3, 2, func() { cs.A = cs.adcAndSetFlags(val) })
	case 0x66: // ROR (zeropage)
		val := cs.read(cs.getZeroPageAddr())
		result := cs.rorAndSetFlags(val)
		cs.storeOp(5, 2, cs.getZeroPageAddr(), result, cs.setNoFlags)
	case 0x68: // PLA (pull A from stack)
		cs.setRegOp(4, 1, &cs.A, cs.pop(), cs.setNoFlags)
	case 0x69: // ADC imm
		val := cs.read(cs.PC + 1)
		cs.opFn(2, 2, func() { cs.A = cs.adcAndSetFlags(val) })
	case 0x6a: // ROR A
		cs.opFn(2, 1, func() { cs.A = cs.rorAndSetFlags(cs.A) })
	case 0x6d: // ADC imm (absolute)
		val := cs.read(cs.getAbsoluteAddr())
		cs.opFn(4, 3, func() { cs.A = cs.adcAndSetFlags(val) })

	case 0x78: // SEI (set disable interrupts flag)
		cs.opFn(2, 1, func() { cs.P |= flagIrqDisabled })

	case 0x84: // STY (zeropage)
		cs.storeOp(3, 2, cs.getZeroPageAddr(), cs.Y, cs.setNoFlags)
	case 0x85: // STA (zeropage)
		cs.storeOp(3, 2, cs.getZeroPageAddr(), cs.A, cs.setNoFlags)
	case 0x86: // STX (zeropage)
		cs.storeOp(3, 2, cs.getZeroPageAddr(), cs.X, cs.setNoFlags)
	case 0x88: // DEY (decrement y)
		cs.setRegOp(2, 1, &cs.Y, cs.Y-1, cs.setZeroNeg)
	case 0x8a: // TXA (transfer x to a)
		cs.setRegOp(2, 1, &cs.A, cs.X, cs.setZeroNeg)
	case 0x8c: // STY absolute
		cs.storeOp(4, 3, cs.getAbsoluteAddr(), cs.Y, cs.setNoFlags)
	case 0x8d: // STA absolute
		cs.storeOp(4, 3, cs.getAbsoluteAddr(), cs.A, cs.setNoFlags)
	case 0x8e: // STX absolute
		cs.storeOp(4, 3, cs.getAbsoluteAddr(), cs.X, cs.setNoFlags)

	case 0x90: // BCC (branch on no carry)
		cs.branchOpRel(cs.P&flagCarry == 0)
	case 0x91: // STA (indirect),y
		cs.storeOp(6, 2, cs.getYPostIndexedAddr(), cs.A, cs.setNoFlags)
	case 0x95: // STA zeropage, x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(4, 2, addr, cs.A, cs.setNoFlags)
	case 0x98: // TYA (transfer y to a)
		cs.setRegOp(2, 1, &cs.A, cs.Y, cs.setZeroNeg)
	case 0x9a: // TXS (transfer x to s)
		cs.setRegOp(2, 1, &cs.S, cs.X, cs.setNoFlags)
	case 0x9d: // STA absolute,x
		cs.storeOp(5, 3, cs.getIndexedAbsoluteAddr(cs.X), cs.A, cs.setNoFlags)

	case 0xa0: // LDY imm
		cs.setRegOp(2, 2, &cs.Y, cs.read(cs.PC+1), cs.setZeroNeg)
	case 0xa2: // LDX imm
		cs.setRegOp(2, 2, &cs.X, cs.read(cs.PC+1), cs.setZeroNeg)
	case 0xa4: // LDY zeropage
		cs.setRegOp(3, 2, &cs.Y, cs.read(cs.getZeroPageAddr()), cs.setZeroNeg)
	case 0xa5: // LDA zeropage
		cs.setRegOp(3, 2, &cs.A, cs.read(cs.getZeroPageAddr()), cs.setZeroNeg)
	case 0xa6: // LDX zeropage
		cs.setRegOp(3, 2, &cs.X, cs.read(cs.getZeroPageAddr()), cs.setZeroNeg)
	case 0xa8: // TAY (transfer a to y)
		cs.setRegOp(2, 1, &cs.Y, cs.A, cs.setZeroNeg)
	case 0xa9: // LDA imm
		cs.setRegOp(2, 2, &cs.A, cs.read(cs.PC+1), cs.setZeroNeg)
	case 0xaa: // TAX (transfer a to x)
		cs.setRegOp(2, 1, &cs.X, cs.A, cs.setZeroNeg)
	case 0xad: // LDA absolute
		cs.setRegOp(4, 3, &cs.A, cs.read(cs.getAbsoluteAddr()), cs.setZeroNeg)
	case 0xae: // LDX absolute
		cs.setRegOp(4, 3, &cs.X, cs.read(cs.getAbsoluteAddr()), cs.setZeroNeg)

	case 0xb0: // BCS (branch on carry)
		cs.branchOpRel(cs.P&flagCarry == flagCarry)
	case 0xb1: // LDA (indirect), y
		// TODO: adjust cycles if page boundary crossed
		cs.setRegOp(5, 2, &cs.A, cs.read(cs.getYPostIndexedAddr()), cs.setZeroNeg)
	case 0xb4: // LDY zeropage, x
		cs.setRegOp(4, 2, &cs.Y, cs.read(cs.getIndexedZeroPageAddr(cs.X)), cs.setZeroNeg)
	case 0xb5: // LDA zeropage, x
		cs.setRegOp(4, 2, &cs.A, cs.read(cs.getIndexedZeroPageAddr(cs.X)), cs.setZeroNeg)
	case 0xb9: // LDA absolute, y
		cs.setRegOp(4, 3, &cs.A, cs.read(cs.getIndexedAbsoluteAddr(cs.Y)), cs.setZeroNeg)
	case 0xbd: // LDA absolute, x
		cs.setRegOp(4, 3, &cs.A, cs.read(cs.getIndexedAbsoluteAddr(cs.X)), cs.setZeroNeg)

	case 0xc0: // CPY imm
		cs.cmpOp(2, 2, cs.Y, cs.read(cs.PC+1))
	case 0xc6: // DEC zeropage
		addr := cs.getZeroPageAddr()
		cs.storeOp(5, 2, addr, cs.read(addr)-1, cs.setZeroNeg)
	case 0xce: // DEC absolute
		addr := cs.getAbsoluteAddr()
		cs.storeOp(6, 3, addr, cs.read(addr)-1, cs.setZeroNeg)
	case 0xc4: // CPY (zeropage)
		cs.cmpOp(3, 2, cs.Y, cs.read(cs.getZeroPageAddr()))
	case 0xc8: // INY (increment y)
		cs.setRegOp(2, 1, &cs.Y, cs.Y+1, cs.setZeroNeg)
	case 0xc9: // CMP imm
		cs.cmpOp(2, 2, cs.A, cs.read(cs.PC+1))
	case 0xca: // DEX (decrement x)
		cs.setRegOp(2, 1, &cs.X, cs.X-1, cs.setZeroNeg)

	case 0xd0: // BNE (branch on not zero)
		cs.branchOpRel(cs.P&flagZero == 0)
	case 0xd6: // DEC zeropage,x
		addr := cs.getIndexedZeroPageAddr(cs.X)
		cs.storeOp(6, 2, addr, cs.read(addr)-1, cs.setZeroNeg)
	case 0xd8: // CLD (clear decimal mode)
		cs.opFn(2, 1, func() { cs.P &^= flagDecimal })

	case 0xe0: // CPX imm
		cs.cmpOp(2, 2, cs.X, cs.read(cs.PC+1))
	case 0xe5: // SBC (zeropage)
		val := cs.read(cs.getZeroPageAddr())
		cs.opFn(3, 2, func() { cs.A = cs.sbcAndSetFlags(val) })
	case 0xe6: // INC zeropage
		addr := cs.getZeroPageAddr()
		cs.storeOp(5, 2, addr, cs.read(addr)+1, cs.setZeroNeg)
	case 0xe9: // SBC imm
		val := cs.read(cs.PC + 1)
		cs.opFn(2, 2, func() { cs.A = cs.sbcAndSetFlags(val) })
	case 0xe8: // INX (increment x)
		cs.setRegOp(2, 1, &cs.X, cs.X+1, cs.setZeroNeg)
	case 0xee: // INC absolute
		addr := cs.getAbsoluteAddr()
		cs.storeOp(6, 3, addr, cs.read(addr)+1, cs.setZeroNeg)

	case 0xf0: // BEQ (branch on zero)
		cs.branchOpRel(cs.P&flagZero == flagZero)

	default:
		stepErr(fmt.Sprintf("unimplemented opcode 0x%02x", opcode))
	}
}

func (cs *cpuState) adcAndSetFlags(val byte) byte {
	var bigResult int
	if cs.P&flagCarry == flagCarry {
		bigResult = int(val) + int(cs.A) + 1
	} else {
		bigResult = int(val) + int(cs.A)
	}
	result := byte(bigResult)
	vSign, aSign, rSign := int8(result) < 0, int8(val) < 0, int8(cs.A) < 0
	cs.setOverflowFlag(aSign == vSign && aSign != rSign)
	cs.setCarryFlag(bigResult > 0xff)
	cs.setZeroNeg(result)
	return result
}

func (cs *cpuState) sbcAndSetFlags(val byte) byte {
	var bigResult int
	if cs.P&flagCarry == flagCarry {
		bigResult = int(val) - int(cs.A) - 1
	} else {
		bigResult = int(val) - int(cs.A)
	}
	result := byte(bigResult)
	vSign, aSign, rSign := int8(result) < 0, int8(val) < 0, int8(cs.A) < 0
	cs.setOverflowFlag(aSign == vSign && aSign != rSign)
	cs.setCarryFlag(bigResult < 0)
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

func (cs *cpuState) setCarryFlagAdd(val, addend byte) {
	if int(val)-int(addend) > 0xff {
		cs.P |= flagCarry
	} else {
		cs.P &^= flagCarry
	}
}

func (cs *cpuState) setCarryFlagSub(val, subtrahend byte) {
	if int(val)-int(subtrahend) < 0 {
		cs.P |= flagCarry
	} else {
		cs.P &^= flagCarry
	}
}

func (cs *cpuState) cmpOp(nCycles uint, instLen uint16, reg byte, val byte) {
	cs.runCycles(nCycles)
	cs.PC += instLen
	cs.setZeroNeg(reg - val)
	cs.setCarryFlagSub(reg, val)
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
