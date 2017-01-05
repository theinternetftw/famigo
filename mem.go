package famigo

import "fmt"

type mem struct {
	MMC          mmc
	PrgROM       []byte
	PrgRAM       []byte
	ChrROM       []byte
	InternalVRAM [0x0800]byte
	InternalRAM  [0x0800]byte
}

func (cs *cpuState) read(addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		val = cs.Mem.InternalRAM[addr&0x07ff]
	case addr >= 0x2000 && addr < 0x4000:
		val = cs.ppuRead(addr)
	case addr == 0x4014:
		val = 0xff // dma reg - write only
	case addr == 0x4016:
		val = cs.readJoypadReg1()
	case addr == 0x4017:
		val = cs.readJoypadReg2()
	case addr >= 0x4000 && addr < 0x4018:
		stepErr(fmt.Sprintf("APU/IO not implemented, read at %04x", addr))
	case addr >= 0x4018 && addr < 0x4020:
		stepErr(fmt.Sprintf("CPU test mode not implemented, read at %04x", addr))
	case addr >= 0x4020:
		val = cs.Mem.MMC.Read(&cs.Mem, addr)
	default:
		stepErr(fmt.Sprintf("unimplemented read: %v", addr))
	}
	fmt.Printf("read(0x%04x) = 0x%02x\n", addr, val)
	return val
}

func (cs *cpuState) read16(addr uint16) uint16 {
	low := uint16(cs.read(addr))
	high := uint16(cs.read(addr + 1))
	return (high << 8) | low
}

func (cs *cpuState) oamDMA(addrBase byte) {
	addr := uint16(addrBase) << 8
	for i := 0; i < 256; i++ {
		cs.write(0x2004, cs.read(addr))
		addr++
	}
}

func (cs *cpuState) write(addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		cs.Mem.InternalRAM[addr&0x07ff] = val
	case addr >= 0x2000 && addr < 0x4000:
		cs.ppuWrite(addr, val)
	case addr == 0x4014:
		cs.oamDMA(val)
	case addr == 0x4015:
		cs.APU.writeStatusReg(val)
	case addr == 0x4016:
		cs.writeJoypadReg1(val)
	case addr == 0x4017:
		cs.APU.writeFrameCounterReg(val)
	case addr >= 0x4000 && addr < 0x4018:
		stepErr(fmt.Sprintf("APU/IO not implemented, write(0x%04x, 0x%02x)", addr, val))
	case addr >= 0x4018 && addr < 0x4020:
		stepErr(fmt.Sprintf("CPU test not implemented, write(0x%04x, 0x%02x)", addr, val))
	case addr >= 0x4020:
		cs.Mem.MMC.Write(&cs.Mem, addr, val)
	default:
		stepErr(fmt.Sprintf("unimplemented: write(0x%04x, 0x%02x)", addr, val))
	}
	fmt.Printf("write(0x%04x, 0x%02x)\n", addr, val)
}

func (cs *cpuState) ppuRead(addr uint16) byte {
	realAddr := (addr - 0x2000) & 0x07
	switch realAddr {
	case 0x00:
		return cs.PPU.readControlReg()
	case 0x01:
		return cs.PPU.readMaskReg()
	case 0x02:
		return cs.PPU.readStatusReg()
	case 0x03:
		return cs.PPU.readOAMAddrReg()
	case 0x04:
		return cs.PPU.readOAMDataReg()
	case 0x05:
		return cs.PPU.readScrollReg()
	case 0x06:
		return cs.PPU.readAddrReg()
	case 0x07:
		return cs.PPU.readDataReg(&cs.Mem)
	default:
		stepErr(fmt.Sprintf("PPU not implemented, read at %04x", realAddr))
	}
	panic("never get here")
}
func (cs *cpuState) ppuWrite(addr uint16, val byte) {
	realAddr := (addr - 0x2000) & 0x07
	switch realAddr {
	case 0x00:
		cs.PPU.writeControlReg(val)
	case 0x01:
		cs.PPU.writeMaskReg(val)
	case 0x02:
		cs.PPU.writeStatusReg(val)
	case 0x03:
		cs.PPU.writeOAMAddrReg(val)
	case 0x04:
		cs.PPU.writeOAMDataReg(val)
	case 0x05:
		cs.PPU.writeScrollReg(val)
	case 0x06:
		cs.PPU.writeAddrReg(val)
	case 0x07:
		cs.PPU.writeDataReg(&cs.Mem, val)
	default:
		stepErr(fmt.Sprintf("PPU not implemented, write(%04x, %02x)", realAddr, val))
	}
}

func (cs *cpuState) write16(addr uint16, val uint16) {
	cs.write(addr, byte(val))
	cs.write(addr+1, byte(val>>8))
}
