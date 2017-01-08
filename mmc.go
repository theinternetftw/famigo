package famigo

import "fmt"

func makeMMC(cartInfo *CartInfo) mmc {
	mapperNum := cartInfo.GetMapperNumber()
	switch mapperNum {
	case 0:
		return &mapper000{
			vramMirroring: cartInfo.GetMirrorInfo(),
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

type mapper000 struct {
	vramMirroring MirrorInfo
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
		if m.vramMirroring == VerticalMirroring {
			val = mem.InternalVRAM[(addr-0x2000)&0x07ff]
		} else if m.vramMirroring == HorizontalMirroring {
			// can surely do this in one shot
			if addr < 0x2800 {
				val = mem.InternalVRAM[((addr & 0x23ff) - 0x2000)]
			} else {
				val = mem.InternalVRAM[((addr & 0x2bff) - 0x2400)]
			}
		} else {
			stepErr(fmt.Sprintf("mapper000: unimplemented vram mirroring: read(%04x, %02x)", addr, val))
		}
	default:
		stepErr(fmt.Sprintf("mapper000: unimplemented vram access: read(%04x, %02x)", addr, val))
	}
	return val
}

func (m *mapper000) WriteVRAM(mem *mem, addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		// nop
	case addr >= 0x2000 && addr < 0x2fff:
		if m.vramMirroring == VerticalMirroring {
			mem.InternalVRAM[(addr-0x2000)&0x27ff] = val
		} else if m.vramMirroring == HorizontalMirroring {
			// can surely do this in one shot
			if addr < 0x2800 {
				mem.InternalVRAM[((addr & 0x23ff) - 0x2000)] = val
			} else {
				mem.InternalVRAM[((addr & 0x2bff) - 0x2400)] = val
			}
		} else {
			stepErr(fmt.Sprintf("mapper000: unimplemented vram mirroring: write(%04x, %02x)", addr, val))
		}
	default:
		stepErr(fmt.Sprintf("mapper000: unimplemented vram access: write(%04x, %02x)", addr, val))
	}
}
