package famigo

import (
	"encoding/json"
	"fmt"
)

func makeMMC(cartInfo *CartInfo) mmc {
	mapperNum := cartInfo.GetMapperNumber()
	switch mapperNum {
	case 0:
		return &mapper000{
			VramMirroring: cartInfo.GetMirrorInfo(),
			IsChrRAM:      cartInfo.IsChrRAM(),
		}
	case 1:
		return &mapper001{
			VramMirroring: cartInfo.GetMirrorInfo(),
			IsChrRAM:      cartInfo.IsChrRAM(),
		}
	case 2:
		return &mapper002{
			VramMirroring: cartInfo.GetMirrorInfo(),
			IsChrRAM:      cartInfo.IsChrRAM(),
		}
	case 3:
		return &mapper003{
			VramMirroring: cartInfo.GetMirrorInfo(),
			IsChrRAM:      cartInfo.IsChrRAM(),
		}
	case 4:
		return &mapper004{}
	case 7:
		return &mapper007{}
	case 31:
		return &mapper031{
			VramMirroring: cartInfo.GetMirrorInfo(),
			IsChrRAM:      cartInfo.IsChrRAM(),
		}
	default:
		emuErr(fmt.Sprintf("makeMMC: unimplemented mapper number %v", mapperNum))
		panic("unreachable code")
	}
}

type mmc interface {
	Init(mem *mem)
	Read(mem *mem, addr uint16) byte
	Write(mem *mem, addr uint16, val byte)
	ReadVRAM(mem *mem, addr uint16) byte
	WriteVRAM(mem *mem, addr uint16, val byte)
	RunCycle(cpu *cpuState)

	Marshal() marshalledMMC
}

func unmarshalMMC(m marshalledMMC) (mmc, error) {
	var mmc mmc
	switch m.Number {
	case 0:
		mmc = &mapper000{}
	case 1:
		mmc = &mapper001{}
	case 2:
		mmc = &mapper002{}
	case 3:
		mmc = &mapper003{}
	case 4:
		mmc = &mapper004{}
	case 7:
		mmc = &mapper007{}
	case 31:
		mmc = &mapper031{}
	default:
		return nil, fmt.Errorf("state contained unknown mapper number %v", m.Number)
	}
	if err := json.Unmarshal(m.Data, &mmc); err != nil {
		return nil, err
	}
	return mmc, nil
}

type marshalledMMC struct {
	Number uint32
	Data   []byte
}

func marshalMMC(number uint32, mmc mmc) marshalledMMC {
	rawJSON, err := json.Marshal(mmc)
	if err != nil {
		panic(err)
	}
	return marshalledMMC{
		Number: number,
		Data:   rawJSON,
	}
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
	IsChrRAM      bool
}

func (m *mapper000) Init(mem *mem)          {}
func (m *mapper000) RunCycle(cs *cpuState)  {}
func (m *mapper000) Marshal() marshalledMMC { return marshalMMC(0, m) }

func (m *mapper000) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		// will crash if no RAM, but should be fine
		return mem.PrgRAM[(int(addr)-0x6000)&(len(mem.PrgRAM)-1)]
	}
	if addr >= 0x8000 {
		return mem.prgROM[(int(addr)-0x8000)&(len(mem.prgROM)-1)]
	}
	return 0xff
}

func (m *mapper000) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		realAddr := (int(addr) - 0x6000) & (len(mem.PrgRAM) - 1)
		if realAddr < len(mem.PrgRAM) {
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
		val = mem.chrROM[addr]
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper000: unimplemented vram mirroring: %v at read(%04x, %02x)", m.VramMirroring, addr, val))
		}
		val = mem.InternalVRAM[realAddr]
	default:
		emuErr(fmt.Sprintf("mapper000: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper000) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		if m.IsChrRAM {
			mem.chrROM[addr] = val
		}
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper000: unimplemented vram mirroring: write(%04x, %02x)", addr, val))
		}
		mem.InternalVRAM[realAddr] = val
	default:
		emuErr(fmt.Sprintf("mapper000: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}

type mapper001 struct {
	VramMirroring        MirrorInfo
	ShiftReg             byte
	ShiftRegWriteCounter byte
	PrgBankMode          mapper001PRGBankMode
	ChrBankMode          mapper001CHRBankMode
	PrgBankNumber        int
	PrgBankNumber256     int
	ChrBank0Number       int
	ChrBank1Number       int
	PrgRAMBankNumber     int
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
func (m *mapper001) RunCycle(cs *cpuState)  {}
func (m *mapper001) Marshal() marshalledMMC { return marshalMMC(1, m) }

func (m *mapper001) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		// FIXME: not complete for SOROM and SXROM
		// will crash if no RAM, but should be fine
		realAddr := 8*1024*m.PrgRAMBankNumber + int(addr-0x6000)
		return mem.PrgRAM[realAddr&(len(mem.PrgRAM)-1)]
	}
	if addr >= 0x8000 {
		switch m.PrgBankMode {
		case oneBigBank:
			realAddr := 256*1024*m.PrgBankNumber256 + 16*1024*m.PrgBankNumber + int(addr-0x8000)
			return mem.prgROM[realAddr]
		case firstBankFixed:
			if addr < 0xc000 {
				realAddr := 256*1024*m.PrgBankNumber256 + int(addr-0x8000)
				return mem.prgROM[realAddr]
			}
			realAddr := 256*1024*m.PrgBankNumber256 + 16*1024*m.PrgBankNumber + int(addr-0xc000)
			return mem.prgROM[realAddr]
		case lastBankFixed:
			if addr >= 0xc000 {
				var lastBankStart int
				if len(mem.prgROM) > 256*1024 && m.PrgBankNumber256 == 0 {
					lastBankStart = 256*1024 - 16*1024
				} else {
					lastBankStart = len(mem.prgROM) - (16 * 1024)
				}
				return mem.prgROM[lastBankStart+int(addr-0xc000)]
			}
			realAddr := 256*1024*m.PrgBankNumber256 + 16*1024*m.PrgBankNumber + int(addr-0x8000)
			return mem.prgROM[realAddr]
		}
	}
	return 0xff
}

func (m *mapper001) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		// FIXME: not complete for SOROM and SXROM
		// will crash if no RAM, but should be fine
		realAddr := 8*1024*m.PrgRAMBankNumber + int(addr-0x6000)
		realAddr &= len(mem.PrgRAM) - 1
		if realAddr < len(mem.PrgRAM) {
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
				m.writeReg(mem, addr, m.ShiftReg)
				m.ShiftRegWriteCounter = 0
				m.ShiftReg = 0
			} else {
				m.ShiftReg >>= 1
			}
		}
	}
}

func (m *mapper001) writeReg(mem *mem, addr uint16, val byte) {
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
			emuErr(fmt.Sprintf("mapper001: mirroring style not yet implemented: %v", val&0x03))
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
		chrBankNum := int(val)
		if len(mem.prgROM) > 256*1024 {
			m.PrgBankNumber256 = int(val>>4) & 0x01
			chrBankNum &= 0x0f
		}
		if len(mem.chrROM) == 8*1024 {
			chrBankNum &= 0x01
			if len(mem.prgROM) <= 256*1024 {
				// impl ram disable here, if desired
			}
			m.PrgRAMBankNumber = int(val>>2) & 0x03
		}
		m.ChrBank0Number = chrBankNum & (len(mem.chrROM)/(4*1024) - 1)

	case addr >= 0xc000 && addr < 0xe000:
		if m.ChrBankMode != oneBank {
			chrBankNum := int(val)
			if len(mem.prgROM) > 256*1024 {
				m.PrgBankNumber256 = int(val>>4) & 0x01
				chrBankNum &= 0x0f
			}
			if len(mem.chrROM) == 8*1024 {
				chrBankNum &= 0x01
				if len(mem.prgROM) <= 256*1024 {
					// impl ram disable here, if desired
				}
				m.PrgRAMBankNumber = int(val>>2) & 0x03
			}
			m.ChrBank1Number = chrBankNum & (len(mem.chrROM)/(4*1024) - 1)
		}

	case addr >= 0xe000:
		m.RAMEnabled = val&0x10 == 0x10
		if m.PrgBankMode == oneBigBank {
			val &^= 0x01
		}
		m.PrgBankNumber = int(val&0x0f) & (len(mem.prgROM)/(16*1024) - 1)
	}
}

func (m *mapper001) getchrROMAddr(addr uint16) int {
	if m.ChrBankMode == oneBank || addr < 0x1000 {
		return int(m.ChrBank0Number)*1024*4 + int(addr)
	}
	return int(m.ChrBank1Number)*1024*4 + int(addr-0x1000)
}

func (m *mapper001) ReadVRAM(mem *mem, addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		realAddr := m.getchrROMAddr(addr)
		val = mem.chrROM[realAddr]
	case addr >= 0x2000 && addr < 0x3000:
		switch m.VramMirroring {
		case OneScreenLowerMirroring:
			val = mem.InternalVRAM[oneScreenLowerVRAMAddr(addr)]
		case OneScreenUpperMirroring:
			val = mem.InternalVRAM[oneScreenUpperVRAMAddr(addr)]
		case VerticalMirroring:
			val = mem.InternalVRAM[vertMirrorVRAMAddr(addr)]
		case HorizontalMirroring:
			val = mem.InternalVRAM[horizMirrorVRAMAddr(addr)]
		default:
			emuErr(fmt.Sprintf("mapper001: unimplemented vram mirroring %v: read(%04x)", m.VramMirroring, addr))
		}
	default:
		emuErr(fmt.Sprintf("mapper001: unimplemented vram access: read(%04x)", addr))
	}
	return val
}

func (m *mapper001) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		// NOTE: this ignores the difference between CHR ROM and CHR RAM
		if m.IsChrRAM {
			mem.chrROM[m.getchrROMAddr(addr)] = val
		}
	case addr >= 0x2000 && addr < 0x3000:
		switch m.VramMirroring {
		case OneScreenLowerMirroring:
			mem.InternalVRAM[oneScreenLowerVRAMAddr(addr)] = val
		case OneScreenUpperMirroring:
			mem.InternalVRAM[oneScreenUpperVRAMAddr(addr)] = val
		case VerticalMirroring:
			mem.InternalVRAM[vertMirrorVRAMAddr(addr)] = val
		case HorizontalMirroring:
			mem.InternalVRAM[horizMirrorVRAMAddr(addr)] = val
		default:
			emuErr(fmt.Sprintf("mapper001: unimplemented vram mirroring %v: write(%04x, %02x)", m.VramMirroring, addr, val))
		}
	default:
		emuErr(fmt.Sprintf("mapper001: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}

type mapper002 struct {
	VramMirroring MirrorInfo
	PrgBankNumber int
	IsChrRAM      bool
}

func (m *mapper002) Init(mem *mem)          {}
func (m *mapper002) RunCycle(cs *cpuState)  {}
func (m *mapper002) Marshal() marshalledMMC { return marshalMMC(2, m) }

func (m *mapper002) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		// will crash if no RAM, but should be fine
		return mem.PrgRAM[(int(addr)-0x6000)&(len(mem.PrgRAM)-1)]
	}
	if addr >= 0x8000 && addr < 0xc000 {
		return mem.prgROM[m.PrgBankNumber*16*1024+int(addr-0x8000)]
	}
	return mem.prgROM[(len(mem.prgROM)-16*1024)+int(addr-0xc000)]
}

func (m *mapper002) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		realAddr := (int(addr) - 0x6000) & (len(mem.PrgRAM) - 1)
		if realAddr < len(mem.PrgRAM) {
			mem.PrgRAM[realAddr] = val
		}
	}
	if addr >= 0x8000 {
		m.PrgBankNumber = int(val)
		m.PrgBankNumber &= len(mem.prgROM)/(16*1024) - 1
	}
}

func (m *mapper002) ReadVRAM(mem *mem, addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		val = mem.chrROM[addr]
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper002: unimplemented vram mirroring: %v at read(%04x, %02x)", m.VramMirroring, addr, val))
		}
		val = mem.InternalVRAM[realAddr]
	default:
		emuErr(fmt.Sprintf("mapper002: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper002) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		if m.IsChrRAM {
			mem.chrROM[addr] = val
		}
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper002: unimplemented vram mirroring: write(%04x, %02x)", addr, val))
		}
		mem.InternalVRAM[realAddr] = val
	default:
		emuErr(fmt.Sprintf("mapper002: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}

type mapper003 struct {
	VramMirroring MirrorInfo
	IsChrRAM      bool
	ChrBankNumber int
}

func (m *mapper003) Init(mem *mem) {}

func (m *mapper003) RunCycle(cs *cpuState)  {}
func (m *mapper003) Marshal() marshalledMMC { return marshalMMC(3, m) }

func (m *mapper003) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		// will crash if no RAM, but should be fine
		return mem.PrgRAM[(int(addr)-0x6000)&(len(mem.PrgRAM)-1)]
	}
	if addr >= 0x8000 {
		return mem.prgROM[(int(addr)-0x8000)&(len(mem.prgROM)-1)]
	}
	return 0xff
}

func (m *mapper003) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		realAddr := (int(addr) - 0x6000) & (len(mem.PrgRAM) - 1)
		if realAddr < len(mem.PrgRAM) {
			mem.PrgRAM[realAddr] = val
		}
	}
	if addr >= 0x8000 {
		m.ChrBankNumber = int(val)
		m.ChrBankNumber &= len(mem.chrROM)/(8*1024) - 1
	}
}

func (m *mapper003) ReadVRAM(mem *mem, addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		realAddr := int(m.ChrBankNumber)*1024*8 + int(addr)
		val = mem.chrROM[realAddr]
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper003: unimplemented vram mirroring: %v at read(%04x, %02x)", m.VramMirroring, addr, val))
		}
		val = mem.InternalVRAM[realAddr]
	default:
		emuErr(fmt.Sprintf("mapper003: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper003) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		if m.IsChrRAM {
			realAddr := int(m.ChrBankNumber)*1024*8 + int(addr)
			mem.chrROM[realAddr] = val
		}
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper003: unimplemented vram mirroring: write(%04x, %02x)", addr, val))
		}
		mem.InternalVRAM[realAddr] = val
	default:
		emuErr(fmt.Sprintf("mapper003: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}

type mapper004 struct {
	VramMirroring MirrorInfo

	BankWriteSelector byte

	PrgLowerBankIsLocked bool

	PrgBank0Number int
	PrgBank1Number int

	ChrUpperBanksAreBigger bool

	ChrBank0Number int
	ChrBank1Number int
	ChrBank2Number int
	ChrBank3Number int
	ChrBank4Number int
	ChrBank5Number int

	IRQLastPPUCycles          int // NOTE: if accuracy demands, change this to track ppu.AddrReg bit 12
	IRQCounter                byte
	IRQCounterReloadValue     byte
	IRQCounterReloadRequested bool
	IRQRequested              bool
	IRQEnabled                bool
}

func (m *mapper004) Init(mem *mem)          {}
func (m *mapper004) Marshal() marshalledMMC { return marshalMMC(4, m) }

func (m *mapper004) RunCycle(cs *cpuState) {
	endOfScanline := 260
	isRendering := cs.PPU.ShowBG && cs.PPU.LineY >= -1 && cs.PPU.LineY < 240
	if isRendering && m.IRQLastPPUCycles < endOfScanline && cs.PPU.PPUCyclesSinceYInc >= endOfScanline {
		if m.IRQCounterReloadRequested {
			m.IRQCounterReloadRequested = false
			m.IRQCounter = m.IRQCounterReloadValue
		}
		if m.IRQCounter == 0 {
			if m.IRQEnabled {
				cs.IRQ = true
			}
			m.IRQCounter = m.IRQCounterReloadValue
		} else {
			m.IRQCounter--
		}
	}
	m.IRQLastPPUCycles = cs.PPU.PPUCyclesSinceYInc
}

func (m *mapper004) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		return mem.PrgRAM[int(addr-0x6000)&(len(mem.PrgRAM)-1)]
	}
	if addr >= 0x8000 && addr < 0xa000 {
		if m.PrgLowerBankIsLocked {
			offset := len(mem.prgROM) - 2*8*1024 // second to last bank
			return mem.prgROM[offset+int(addr-0x8000)]
		}
		return mem.prgROM[1024*8*m.PrgBank0Number+int(addr-0x8000)]
	}
	if addr >= 0xa000 && addr < 0xc000 {
		return mem.prgROM[1024*8*m.PrgBank1Number+int(addr-0xa000)]
	}
	if addr >= 0xc000 && addr < 0xe000 {
		if m.PrgLowerBankIsLocked {
			return mem.prgROM[1024*8*m.PrgBank0Number+int(addr-0xc000)]
		}
		offset := len(mem.prgROM) - 2*8*1024 // second to last bank
		return mem.prgROM[offset+int(addr-0xc000)]
	}
	// addr > 0xe000
	offset := len(mem.prgROM) - 8*1024 // last bank
	return mem.prgROM[offset+int(addr-0xe000)]
}

func (m *mapper004) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		realAddr := (int(addr) - 0x6000) & (len(mem.PrgRAM) - 1)
		if realAddr < len(mem.PrgRAM) {
			mem.PrgRAM[realAddr] = val
		}
	}
	if addr >= 0x8000 && addr < 0xa000 {
		if addr&0x01 == 0 {
			m.ChrUpperBanksAreBigger = val&0x80 == 0x80
			m.PrgLowerBankIsLocked = val&0x40 == 0x40
			// MM6 ram enable bit here, but let's ignore it for easier compat
			m.BankWriteSelector = val & 0x07
		} else {
			bankNumReg := []*int{
				&m.ChrBank0Number,
				&m.ChrBank1Number,
				&m.ChrBank2Number,
				&m.ChrBank3Number,
				&m.ChrBank4Number,
				&m.ChrBank5Number,
				&m.PrgBank0Number,
				&m.PrgBank1Number,
			}[m.BankWriteSelector]
			*bankNumReg = int(val)
			if m.BankWriteSelector < 6 {
				*bankNumReg &= len(mem.chrROM)/1024 - 1
			} else {
				*bankNumReg &= len(mem.prgROM)/(8*1024) - 1
			}
		}
	}
	if addr >= 0xa000 && addr < 0xc000 {
		if addr&0x01 == 0 {
			if val&0x01 == 0 {
				m.VramMirroring = VerticalMirroring
			} else {
				m.VramMirroring = HorizontalMirroring
			}
		} else {
			// ram protect; do nothing to improve mmc3/mmc6 compat
		}
	}
	if addr >= 0xc000 && addr < 0xe000 {
		if addr&0x01 == 0 {
			m.IRQCounterReloadValue = val
		} else {
			m.IRQCounterReloadRequested = true
		}
	}
	if addr >= 0xe000 {
		if addr&0x01 == 0 {
			m.IRQRequested = false
			m.IRQEnabled = false
		} else {
			m.IRQEnabled = true
		}
	}
}

func (m *mapper004) ReadVRAM(mem *mem, addr uint16) byte {
	var val byte
	switch {
	case addr < 0x0400:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = m.ChrBank2Number * 1024
		} else {
			offset = (m.ChrBank0Number &^ 0x01) * 1024
		}
		val = mem.chrROM[offset+int(addr)]
	case addr >= 0x0400 && addr < 0x0800:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = m.ChrBank3Number * 1024
		} else {
			offset = (m.ChrBank0Number | 0x01) * 1024
		}
		val = mem.chrROM[offset+int(addr-0x0400)]
	case addr >= 0x0800 && addr < 0x0c00:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = m.ChrBank4Number * 1024
		} else {
			offset = (m.ChrBank1Number &^ 0x01) * 1024
		}
		val = mem.chrROM[offset+int(addr-0x0800)]
	case addr >= 0x0c00 && addr < 0x1000:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = m.ChrBank5Number * 1024
		} else {
			offset = (m.ChrBank1Number | 0x01) * 1024
		}
		val = mem.chrROM[offset+int(addr-0x0c00)]
	case addr >= 0x1000 && addr < 0x1400:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = (m.ChrBank0Number &^ 0x01) * 1024
		} else {
			offset = m.ChrBank2Number * 1024
		}
		val = mem.chrROM[offset+int(addr-0x1000)]
	case addr >= 0x1400 && addr < 0x1800:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = (m.ChrBank0Number | 0x01) * 1024
		} else {
			offset = m.ChrBank3Number * 1024
		}
		val = mem.chrROM[offset+int(addr-0x1400)]
	case addr >= 0x1800 && addr < 0x1c00:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = (m.ChrBank1Number &^ 0x01) * 1024
		} else {
			offset = m.ChrBank4Number * 1024
		}
		val = mem.chrROM[offset+int(addr-0x1800)]
	case addr >= 0x1c00 && addr < 0x2000:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = (m.ChrBank1Number | 0x01) * 1024
		} else {
			offset = m.ChrBank5Number * 1024
		}
		val = mem.chrROM[offset+int(addr-0x1c00)]
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper004: unimplemented vram mirroring: %v at read(%04x, %02x)", m.VramMirroring, addr, val))
		}
		val = mem.InternalVRAM[realAddr]
	default:
		emuErr(fmt.Sprintf("mapper004: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper004) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x0400:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = m.ChrBank2Number * 1024
		} else {
			offset = (m.ChrBank0Number &^ 0x01) * 1024
		}
		mem.chrROM[offset+int(addr)] = val
	case addr >= 0x0400 && addr < 0x0800:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = m.ChrBank3Number * 1024
		} else {
			offset = (m.ChrBank0Number | 0x01) * 1024
		}
		mem.chrROM[offset+int(addr-0x0400)] = val
	case addr >= 0x0800 && addr < 0x0c00:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = m.ChrBank4Number * 1024
		} else {
			offset = (m.ChrBank1Number &^ 0x01) * 1024
		}
		mem.chrROM[offset+int(addr-0x0800)] = val
	case addr >= 0x0c00 && addr < 0x1000:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = m.ChrBank5Number * 1024
		} else {
			offset = (m.ChrBank1Number | 0x01) * 1024
		}
		mem.chrROM[offset+int(addr-0x0c00)] = val
	case addr >= 0x1000 && addr < 0x1400:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = (m.ChrBank0Number &^ 0x01) * 1024
		} else {
			offset = m.ChrBank2Number * 1024
		}
		mem.chrROM[offset+int(addr-0x1000)] = val
	case addr >= 0x1400 && addr < 0x1800:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = (m.ChrBank0Number | 0x01) * 1024
		} else {
			offset = m.ChrBank3Number * 1024
		}
		mem.chrROM[offset+int(addr-0x1400)] = val
	case addr >= 0x1800 && addr < 0x1c00:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = (m.ChrBank1Number &^ 0x01) * 1024
		} else {
			offset = m.ChrBank4Number * 1024
		}
		mem.chrROM[offset+int(addr-0x1800)] = val
	case addr >= 0x1c00 && addr < 0x2000:
		var offset int
		if m.ChrUpperBanksAreBigger {
			offset = (m.ChrBank1Number | 0x01) * 1024
		} else {
			offset = m.ChrBank5Number * 1024
		}
		mem.chrROM[offset+int(addr-0x1c00)] = val
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper004: unimplemented vram mirroring: write(%04x, %02x)", addr, val))
		}
		mem.InternalVRAM[realAddr] = val
	default:
		emuErr(fmt.Sprintf("mapper004: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}

type mapper007 struct {
	VramMirroring MirrorInfo
	PrgBankNumber int
}

func (m *mapper007) Init(mem *mem) {
	m.VramMirroring = OneScreenLowerMirroring
}

func (m *mapper007) RunCycle(cs *cpuState)  {}
func (m *mapper007) Marshal() marshalledMMC { return marshalMMC(7, m) }

func (m *mapper007) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		// no RAM in this mapper
		return 0xff
	}
	if addr >= 0x8000 {
		offset := m.PrgBankNumber * 32 * 1024
		return mem.prgROM[offset+int(addr-0x8000)]
	}
	panic("impossible case")
}

func (m *mapper007) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		// no RAM in this mapper
	}
	if addr >= 0x8000 {
		m.PrgBankNumber = int(val & 0x07)
		m.PrgBankNumber &= len(mem.prgROM)/(32*1024) - 1
		if val&0x10 == 0x10 {
			m.VramMirroring = OneScreenUpperMirroring
		} else {
			m.VramMirroring = OneScreenLowerMirroring
		}
	}
}

func (m *mapper007) ReadVRAM(mem *mem, addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		val = mem.chrROM[addr]
	case addr >= 0x2000 && addr < 0x3000:
		switch m.VramMirroring {
		case OneScreenLowerMirroring:
			val = mem.InternalVRAM[oneScreenLowerVRAMAddr(addr)]
		case OneScreenUpperMirroring:
			val = mem.InternalVRAM[oneScreenUpperVRAMAddr(addr)]
		default:
			emuErr(fmt.Sprintf("mapper007: unimplemented vram mirroring %v: read(%04x)", m.VramMirroring, addr))
		}
	default:
		emuErr(fmt.Sprintf("mapper007: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper007) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		mem.chrROM[addr] = val
	case addr >= 0x2000 && addr < 0x3000:
		switch m.VramMirroring {
		case OneScreenLowerMirroring:
			mem.InternalVRAM[oneScreenLowerVRAMAddr(addr)] = val
		case OneScreenUpperMirroring:
			mem.InternalVRAM[oneScreenUpperVRAMAddr(addr)] = val
		default:
			emuErr(fmt.Sprintf("mapper007: unimplemented vram mirroring %v: write(%04x, %02x)", m.VramMirroring, addr, val))
		}
	default:
		emuErr(fmt.Sprintf("mapper007: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}

type mapper031 struct {
	VramMirroring MirrorInfo
	IsChrRAM      bool

	bankNumSlots [8]int
}

func (m *mapper031) Init(mem *mem) {
	m.bankNumSlots[7] = len(mem.prgROM)/(4*1024) - 1
}

func (m *mapper031) RunCycle(cs *cpuState)  {}
func (m *mapper031) Marshal() marshalledMMC { return marshalMMC(31, m) }

func (m *mapper031) Read(mem *mem, addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		// no prg RAM in this mapper
		return 0xff
	}
	if addr >= 0x8000 {
		slotNum := (addr - 0x8000) >> 12
		offset := m.bankNumSlots[slotNum] * (4 * 1024)
		strippedAddr := int(addr - 0x8000 - (0x1000 * slotNum))
		realAddr := offset + strippedAddr
		return mem.prgROM[realAddr]
	}
	return 0xff
}

func (m *mapper031) Write(mem *mem, addr uint16, val byte) {
	if addr >= 0x5000 && addr < 0x6000 {
		m.bankNumSlots[addr&0x07] = int(val)
	}
	if addr >= 0x6000 && addr < 0x8000 {
		// no prg RAM in this mapper
	}
}

func (m *mapper031) ReadVRAM(mem *mem, addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		val = mem.chrROM[addr]
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper031: unimplemented vram mirroring: %v at read(%04x, %02x)", m.VramMirroring, addr, val))
		}
		val = mem.InternalVRAM[realAddr]
	default:
		emuErr(fmt.Sprintf("mapper031: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper031) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		if m.IsChrRAM {
			mem.chrROM[addr] = val
		}
	case addr >= 0x2000 && addr < 0x3000:
		var realAddr uint16
		if m.VramMirroring == VerticalMirroring {
			realAddr = vertMirrorVRAMAddr(addr)
		} else if m.VramMirroring == HorizontalMirroring {
			realAddr = horizMirrorVRAMAddr(addr)
		} else {
			emuErr(fmt.Sprintf("mapper031: unimplemented vram mirroring: write(%04x, %02x)", addr, val))
		}
		mem.InternalVRAM[realAddr] = val
	default:
		emuErr(fmt.Sprintf("mapper031: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}
