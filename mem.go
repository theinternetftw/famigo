package famigo

import "fmt"

type mem struct {
	mmc          mmc
	prgROM       []byte
	chrROM       []byte
	PrgRAM       []byte
	InternalVRAM [0x0800]byte
	InternalRAM  [0x0800]byte
}

func (emu *emuState) read(addr uint16) byte {
	var val byte
	switch {
	case addr < 0x2000:
		val = emu.Mem.InternalRAM[addr&0x07ff]
	case addr >= 0x2000 && addr < 0x4000:
		val = emu.ppuRead(addr)
	case addr >= 0x4000 && addr < 0x4014:
		val = 0xff // apu control regs, write only
	case addr == 0x4014:
		val = 0xff // dma reg - write only
	case addr == 0x4015:
		val = emu.APU.readStatusReg()
	case addr == 0x4016:
		val = emu.readJoypadReg1()
	case addr == 0x4017:
		val = emu.readJoypadReg2()
	case addr >= 0x4000 && addr < 0x4018:
		emuErr(fmt.Sprintf("APU/IO not implemented, read at %04x", addr))
	case addr >= 0x4018 && addr < 0x4020:
		emuErr(fmt.Sprintf("CPU test mode not implemented, read at %04x", addr))
	case addr >= 0x4020:
		val = emu.Mem.mmc.Read(&emu.Mem, addr)
	default:
		emuErr(fmt.Sprintf("unimplemented read: %v", addr))
	}
	if showMemReads {
		fmt.Printf("read(0x%04x) = 0x%02x\n", addr, val)
	}
	return val
}

func (emu *emuState) read16(addr uint16) uint16 {
	low := uint16(emu.read(addr))
	high := uint16(emu.read(addr + 1))
	return (high << 8) | low
}

func (emu *emuState) oamDMA(addrBase byte) {
	addr := uint16(addrBase) << 8
	for i := 0; i < 256; i++ {
		emu.write(0x2004, emu.read(addr))
		addr++
	}
}

func (emu *emuState) write(addr uint16, val byte) {
	switch {
	case addr < 0x2000:
		emu.Mem.InternalRAM[addr&0x07ff] = val
	case addr >= 0x2000 && addr < 0x4000:
		emu.ppuWrite(addr, val)
	case addr == 0x4000:
		emu.APU.Pulse1.writeVolDutyReg(val)
	case addr == 0x4001:
		emu.APU.Pulse1.writeSweepReg(val)
	case addr == 0x4002:
		emu.APU.Pulse1.writePeriodLowReg(val)
	case addr == 0x4003:
		emu.APU.Pulse1.writePeriodHighTimerReg(val)
	case addr == 0x4004:
		emu.APU.Pulse2.writeVolDutyReg(val)
	case addr == 0x4005:
		emu.APU.Pulse2.writeSweepReg(val)
	case addr == 0x4006:
		emu.APU.Pulse2.writePeriodLowReg(val)
	case addr == 0x4007:
		emu.APU.Pulse2.writePeriodHighTimerReg(val)
	case addr == 0x4008:
		emu.APU.Triangle.writeLinearCounterReg(val)
	case addr == 0x4009:
		// nop
	case addr == 0x400a:
		emu.APU.Triangle.writePeriodLowReg(val)
	case addr == 0x400b:
		emu.APU.Triangle.writePeriodHighTimerReg(val)
	case addr == 0x400c:
		emu.APU.Noise.writeVolDutyReg(val) // duty ignored
	case addr == 0x400d:
		// nop
	case addr == 0x400e:
		emu.APU.Noise.writeNoiseControlReg(val)
	case addr == 0x400f:
		emu.APU.Noise.writeNoiseLength(val)
	case addr == 0x4010:
		emu.APU.DMC.writeDMCFlagsAndRate(val)
	case addr == 0x4011:
		emu.APU.DMC.writeDMCCurrentValue(val)
	case addr == 0x4012:
		emu.APU.DMC.writeDMCInitialSampleAddr(val)
	case addr == 0x4013:
		emu.APU.DMC.writeDMCSampleLength(val)
	case addr == 0x4014:
		emu.oamDMA(val)
	case addr == 0x4015:
		emu.APU.writeStatusReg(val)
	case addr == 0x4016:
		emu.writeJoypadReg1(val)
	case addr == 0x4017:
		emu.APU.writeFrameCounterReg(val)
	case addr >= 0x4000 && addr < 0x4018:
		emuErr(fmt.Sprintf("APU/IO not implemented, write(0x%04x, 0x%02x)", addr, val))
	case addr >= 0x4018 && addr < 0x4020:
		emuErr(fmt.Sprintf("CPU test not implemented, write(0x%04x, 0x%02x)", addr, val))
	case addr >= 0x4020:
		emu.Mem.mmc.Write(&emu.Mem, addr, val)
	default:
		emuErr(fmt.Sprintf("unimplemented: write(0x%04x, 0x%02x)", addr, val))
	}
	if showMemWrites {
		fmt.Printf("write(0x%04x, 0x%02x)\n", addr, val)
	}
}

func (emu *emuState) ppuRead(addr uint16) byte {
	realAddr := (addr - 0x2000) & 0x07
	switch realAddr {
	case 0x00:
		return emu.PPU.readControlReg()
	case 0x01:
		return emu.PPU.readMaskReg()
	case 0x02:
		return emu.PPU.readStatusReg()
	case 0x03:
		return emu.PPU.readOAMAddrReg()
	case 0x04:
		return emu.PPU.readOAMDataReg()
	case 0x05:
		return emu.PPU.readScrollReg()
	case 0x06:
		return emu.PPU.readAddrReg()
	case 0x07:
		return emu.PPU.readDataReg(&emu.Mem)
	default:
		emuErr(fmt.Sprintf("PPU not implemented, read at %04x", realAddr))
	}
	panic("never get here")
}
func (emu *emuState) ppuWrite(addr uint16, val byte) {
	realAddr := (addr - 0x2000) & 0x07
	switch realAddr {
	case 0x00:
		emu.PPU.writeControlReg(val)
	case 0x01:
		emu.PPU.writeMaskReg(val)
	case 0x02:
		emu.PPU.writeStatusReg(val)
	case 0x03:
		emu.PPU.writeOAMAddrReg(val)
	case 0x04:
		emu.PPU.writeOAMDataReg(val)
	case 0x05:
		emu.PPU.writeScrollReg(val)
	case 0x06:
		emu.PPU.writeAddrReg(val)
	case 0x07:
		emu.PPU.writeDataReg(&emu.Mem, val)
	default:
		emuErr(fmt.Sprintf("PPU not implemented, write(%04x, %02x)", realAddr, val))
	}
}

func (emu *emuState) write16(addr uint16, val uint16) {
	emu.write(addr, byte(val))
	emu.write(addr+1, byte(val>>8))
}
