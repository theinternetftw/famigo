package famigo

import "fmt"

func makeMMC(cartInfo *CartInfo) mmc {
	mapperNum := cartInfo.GetMapperNumber()
	switch mapperNum {
	case 0:
		return &mapper000{
			VramMirroring: cartInfo.GetMirrorInfo(),
		}
	case 1:
		return &mapper001{
			VramMirroring: cartInfo.GetMirrorInfo(),
			IsChrRAM:      cartInfo.IsChrRAM(),
		}
	default:
		panic(fmt.Sprintf("makeMMC: unimplemented mapper number %v", mapperNum))
	}
}

type mmc interface {
	Init(mem *mem)
	Read(mem *mem, addr uint16) byte
	Write(mem *mem, addr uint16, val byte)
	ReadVRAM(mem *mem, addr uint16) byte
	WriteVRAM(mem *mem, addr uint16, val byte)
}

func vertMirrorVRAMAddr(addr uint16) uint16 {
	return (addr - 0x2000) & 0x07ff
}
func horizMirrorVRAMAddr(addr uint16) uint16 {
	// can surely do this in one shot
	if addr < 0x2800 {
		return (addr & 0x23ff) - 0x2000
	}
	return (addr & 0x2bff) - 0x2400
}
func oneScreenLowerVRAMAddr(addr uint16) uint16 {
	return (addr - 0x2000) & 0x03ff
}
func oneScreenUpperVRAMAddr(addr uint16) uint16 {
	return ((addr - 0x2000) & 0x03ff) + 0x0400
}

type mapper000 struct {
	VramMirroring MirrorInfo
}

func (m *mapper000) Init(mem *mem) {}

func (m *mapper000) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		// will crash if no RAM, but should be fine
		return mem.PrgRAM[(int(addr)-0x6000)&(len(mem.PrgRAM)-1)]
	}
	if addr >= 0x8000 {
		return mem.PrgROM[(int(addr)-0x8000)&(len(mem.PrgROM)-1)]
	}
	return 0xff
}

func (m *mapper000) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		realAddr := (int(addr) - 0x6000) & (len(mem.PrgRAM) - 1)
		if realAddr < len(mem.PrgRAM)-1 {
			mem.PrgRAM[realAddr] = val
		}
	}
	if addr >= 0x8000 {
		// It's ROM: nop
	}
}

func (m *mapper000) ReadVRAM(mem *mem, addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		val = mem.ChrROM[addr]
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			stepErr(fmt.Sprintf("mapper000: unimplemented vram mirroring: %v at read(%04x, %02x)", m.VramMirroring, addr, val))
		}
		val = mem.InternalVRAM[realAddr]
	default:
		stepErr(fmt.Sprintf("mapper000: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper000) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		// nop
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			stepErr(fmt.Sprintf("mapper000: unimplemented vram mirroring: write(%04x, %02x)", addr, val))
		}
		mem.InternalVRAM[realAddr] = val
	default:
		stepErr(fmt.Sprintf("mapper000: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}

type mapper001 struct {
	VramMirroring        MirrorInfo
	ShiftReg             byte
	ShiftRegWriteCounter byte
	PrgBankMode          mapper001PRGBankMode
	ChrBankMode          mapper001CHRBankMode
	PrgBankNumber        int
	ChrBank0Number       int
	ChrBank1Number       int
	RAMEnabled           bool
	IsChrRAM             bool
}

type mapper001PRGBankMode int

const (
	oneBigBank mapper001PRGBankMode = iota
	firstBankFixed
	lastBankFixed
)

type mapper001CHRBankMode int

const (
	oneBank mapper001CHRBankMode = iota
	twoBanks
)

func (m *mapper001) Init(mem *mem) {
	m.PrgBankMode = lastBankFixed
}

func (m *mapper001) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		// FIXME: not complete for SOROM and SXROM
		// will crash if no RAM, but should be fine
		return mem.PrgRAM[(int(addr)-0x6000)&(len(mem.PrgRAM)-1)]
	}
	if addr >= 0x8000 {
		switch m.PrgBankMode {
		case oneBigBank:
			return mem.PrgROM[16*1024*m.PrgBankNumber+int(addr-0x8000)]
		case firstBankFixed:
			if addr < 0xc000 {
				return mem.PrgROM[addr-0x8000]
			}
			return mem.PrgROM[16*1024*m.PrgBankNumber+int(addr-0xc000)]
		case lastBankFixed:
			if addr >= 0xc000 {
				return mem.PrgROM[len(mem.PrgROM)-(16*1024)+int(addr-0xc000)]
			}
			return mem.PrgROM[16*1024*m.PrgBankNumber+int(addr-0x8000)]
		}
	}
	return 0xff
}

func (m *mapper001) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		realAddr := (int(addr) - 0x6000) & (len(mem.PrgRAM) - 1)
		if realAddr < len(mem.PrgRAM)-1 {
			mem.PrgRAM[realAddr] = val
		}
	} else if addr >= 0x8000 {
		if val&0x80 == 0x80 {
			m.ShiftRegWriteCounter = 0
			m.ShiftReg = 0
			m.PrgBankMode = lastBankFixed
		} else {
			m.ShiftReg |= ((val & 0x01) << 4)
			m.ShiftRegWriteCounter++
			if m.ShiftRegWriteCounter == 5 {
				m.writeReg(addr, m.ShiftReg)
				m.ShiftRegWriteCounter = 0
				m.ShiftReg = 0
			} else {
				m.ShiftReg >>= 1
			}
		}
	}
}

func (m *mapper001) writeReg(addr uint16, val byte) {
	switch {
	case addr >= 0x8000 && addr < 0xa000:

		switch val & 0x03 {
		case 0:
			m.VramMirroring = OneScreenLowerMirroring
		case 1:
			m.VramMirroring = OneScreenUpperMirroring
		case 2:
			m.VramMirroring = VerticalMirroring
		case 3:
			m.VramMirroring = HorizontalMirroring
		default:
			stepErr(fmt.Sprintf("mapper001: mirroring style not yet implemented: %v", val&0x03))
		}

		switch (val >> 2) & 0x03 {
		case 0, 1:
			m.PrgBankMode = oneBigBank
		case 2:
			m.PrgBankMode = firstBankFixed
		case 3:
			m.PrgBankMode = lastBankFixed
		}

		switch (val >> 4) & 0x01 {
		case 0:
			m.ChrBankMode = oneBank
		case 1:
			m.ChrBankMode = twoBanks
		}

	case addr >= 0xa000 && addr < 0xc000:
		if m.ChrBankMode == oneBank {
			val &^= 0x01
		}
		m.ChrBank0Number = int(val)

	case addr >= 0xc000 && addr < 0xe000:
		if m.ChrBankMode == twoBanks {
			m.ChrBank1Number = int(val)
		}

	case addr >= 0xe000:
		m.RAMEnabled = val&0x10 == 0x10
		if m.PrgBankMode == oneBigBank {
			val &^= 0x01
		}
		m.PrgBankNumber = int(val & 0x0f)
	}
}

func (m *mapper001) getChrROMAddr(addr uint16) int {
	if m.ChrBankMode == oneBank || addr < 0x1000 {
		return int(m.ChrBank0Number)*1024*4 + int(addr)
	}
	return int(m.ChrBank1Number)*1024*4 + int(addr-0x1000)
}

func (m *mapper001) ReadVRAM(mem *mem, addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		realAddr := m.getChrROMAddr(addr)
		val = mem.ChrROM[realAddr]
	case addr >= 0x2000 && addr < 0x3000:
		switch m.VramMirroring {
		case OneScreenLowerMirroring:
			val = mem.InternalVRAM[oneScreenLowerVRAMAddr(addr)]
		case OneScreenUpperMirroring:
			val = mem.InternalVRAM[oneScreenLowerVRAMAddr(addr)]
		case VerticalMirroring:
			val = mem.InternalVRAM[vertMirrorVRAMAddr(addr)]
		case HorizontalMirroring:
			val = mem.InternalVRAM[horizMirrorVRAMAddr(addr)]
		default:
			stepErr(fmt.Sprintf("mapper001: unimplemented vram mirroring: read(%04x, %02x)", addr, val))
		}
	default:
		stepErr(fmt.Sprintf("mapper001: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper001) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		// NOTE: this ignores the difference between CHR ROM and CHR RAM
		if m.IsChrRAM {
			mem.ChrROM[m.getChrROMAddr(addr)] = val
		}
	case addr >= 0x2000 && addr < 0x3000:
		if m.VramMirroring == VerticalMirroring {
			mem.InternalVRAM[vertMirrorVRAMAddr(addr)] = val
		} else if m.VramMirroring == HorizontalMirroring {
			mem.InternalVRAM[horizMirrorVRAMAddr(addr)] = val
		} else {
			stepErr(fmt.Sprintf("mapper001: unimplemented vram mirroring: write(%04x, %02x)", addr, val))
		}
	default:
		stepErr(fmt.Sprintf("mapper001: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}
