package famigo

type ppu struct {
	FrameBuffer [256 * 240 * 4]byte

	GenerateVBlankNMIs         bool
	MasterSlaveExtSelector     bool
	UseBigSprites              bool
	UseUpperBGPatternTable     bool
	UseUpperSpritePatternTable bool
	IncrementStyleSelector     bool
	NametableBaseSelector      byte

	InVBlank       bool
	VBlankAlert    bool
	SpriteZeroHit  bool
	SpriteOverflow bool // misnomer: complicated role, due to hw bugs

	SharedReg byte

	PPUCyclesSinceYInc int
	LineY              int
	LineX              int

	EmphasizeBlue           bool
	EmphasizeGreen          bool
	EmphasizeRed            bool
	ShowSprites             bool
	ShowBG                  bool
	ShowSpritesInLeftBorder bool
	ShowBGInLeftBorder      bool
	UseGreyscale            bool

	ScrollX           byte
	ScrollY           byte
	RequestedScrollY  byte
	ScrollRegSelector byte

	AddrReg         uint16
	AddrRegSelector byte

	OAM [256]byte

	OAMAddrReg byte

	PaletteRAM [32]byte
}

const (
	incrementBigStride   = true
	incrementSmallStride = false
)

func (ppu *ppu) writeOAMDataReg(val byte) {
	ppu.OAM[ppu.OAMAddrReg] = val
	ppu.OAMAddrReg++
}
func (ppu *ppu) readOAMDataReg() byte {
	// TODO: complicated if game reads during rendering
	return ppu.OAM[ppu.OAMAddrReg]
}

func (ppu *ppu) writeOAMAddrReg(val byte) {
	ppu.OAMAddrReg = val
}
func (ppu *ppu) readOAMAddrReg() byte {
	return 0xff // write-only
}

func (ppu *ppu) writeDataReg(mem *mem, val byte) {
	if ppu.AddrReg >= 0x3f00 && ppu.AddrReg < 0x4000 {
		addr := (ppu.AddrReg - 0x3f00) & 0x1f
		ppu.PaletteRAM[addr] = val
	} else if ppu.AddrReg >= 0x2000 && ppu.AddrReg < 0x3f00 {
		addr := ppu.AddrReg & 0x2fff
		mem.MMC.WriteVRAM(mem, addr, val)
	} else {
		mem.MMC.WriteVRAM(mem, ppu.AddrReg, val)
	}
	if ppu.IncrementStyleSelector == incrementBigStride {
		ppu.AddrReg += 0x20
	} else {
		ppu.AddrReg++
	}
	ppu.AddrReg &= 0x3fff
}

func (ppu *ppu) readDataReg(mem *mem) byte {
	var val byte
	if ppu.AddrReg >= 0x3f00 && ppu.AddrReg < 0x4000 {
		addr := (ppu.AddrReg - 0x3f00) & 0x1f
		val = ppu.PaletteRAM[addr]
	} else if ppu.AddrReg >= 0x2000 && ppu.AddrReg < 0x3f00 {
		addr := ppu.AddrReg & 0x2fff
		val = mem.MMC.ReadVRAM(mem, addr)
	} else {
		val = mem.MMC.ReadVRAM(mem, ppu.AddrReg)
	}
	if ppu.IncrementStyleSelector == incrementBigStride {
		ppu.AddrReg += 0x20
	} else {
		ppu.AddrReg++
	}
	ppu.AddrReg &= 0x3fff
	return val
}

func (ppu *ppu) writeAddrReg(val byte) {
	if ppu.AddrRegSelector == 0 {
		ppu.AddrReg &^= 0xff00
		ppu.AddrReg |= uint16(val) << 8
		// NOTE: take this and out if
		// nobody really uses this and
		// it's just hiding bugs...
		ppu.AddrReg &= 0x3fff
		ppu.AddrRegSelector = 1
	} else {
		ppu.AddrReg &^= 0x00ff
		ppu.AddrReg |= uint16(val)
		ppu.AddrRegSelector = 0
	}
}
func (ppu *ppu) readAddrReg() byte {
	return 0xff // write only
}

func (ppu *ppu) writeScrollReg(val byte) {
	if ppu.ScrollRegSelector == 0 {
		ppu.ScrollX = val
		ppu.ScrollRegSelector = 1
	} else {
		ppu.RequestedScrollY = val
		ppu.ScrollRegSelector = 0
	}
}
func (ppu *ppu) readScrollReg() byte {
	return 0xff // write only
}

func (ppu *ppu) writeMaskReg(val byte) {
	boolsFromByte(val,
		&ppu.EmphasizeBlue,
		&ppu.EmphasizeGreen,
		&ppu.EmphasizeRed,
		&ppu.ShowSprites,
		&ppu.ShowBG,
		&ppu.ShowSpritesInLeftBorder,
		&ppu.ShowBGInLeftBorder,
		&ppu.UseGreyscale,
	)
}
func (ppu *ppu) readMaskReg() byte {
	return 0xff // write-only
}

const (
	incrementStyleAcross = false
	incrementStyleDown   = true

	masterSlavePPUReads  = false
	masterSlavePPUWrites = true
)

func (ppu *ppu) getNametableBase() uint16 {
	return 0x2000 + 0x400*uint16(ppu.NametableBaseSelector)
}
func (ppu *ppu) getCurrentNametableTileAddr() uint16 {
	return ppu.getNametableBase() + uint16(ppu.getBGY()>>3)*32 + uint16(ppu.getBGX()>>3)
}
func (ppu *ppu) getCurrentNametableAttributeAddr() uint16 {
	return ppu.getNametableBase() + (0x400 - 64) + uint16(ppu.getBGY()>>5)*8 + uint16(ppu.getBGX()>>5)
}
func (ppu *ppu) getPatternAddr(tileID byte) uint16 {
	addr := uint16(tileID) << 4
	if ppu.UseUpperBGPatternTable {
		addr |= 0x1000
	}
	return addr
}
func (ppu *ppu) getBGX() byte { return byte(ppu.LineX) + ppu.ScrollX }
func (ppu *ppu) getBGY() byte { return byte(ppu.LineY) + ppu.ScrollY }

func (ppu *ppu) read(cs *cpuState, addr uint16) byte {
	return cs.Mem.MMC.ReadVRAM(&cs.Mem, addr)
}

func (ppu *ppu) runCycle(cs *cpuState) {
	switch {
	case ppu.PPUCyclesSinceYInc == 1:
		if ppu.LineY == 241 {
			ppu.InVBlank = true
			ppu.VBlankAlert = true
			cs.flipRequested = true
			if ppu.GenerateVBlankNMIs {
				cs.NMI = true
			}
		} else if ppu.LineY == -1 {
			ppu.ScrollY = ppu.RequestedScrollY
		}
	case ppu.PPUCyclesSinceYInc >= 1 && ppu.PPUCyclesSinceYInc <= 256:
		if ppu.LineY >= 0 && ppu.LineY < 240 && ppu.PPUCyclesSinceYInc&0x07 == 0 {
			for i := 0; i < 8; i++ {
				tileID := ppu.read(cs, ppu.getCurrentNametableTileAddr())
				patternAddr := ppu.getPatternAddr(tileID) + uint16(ppu.getBGY()&0x07)
				patternPlane0 := ppu.read(cs, patternAddr)
				patternPlane1 := ppu.read(cs, patternAddr+8)
				patternBit0 := (patternPlane0 >> (7 - (ppu.getBGX() & 0x07))) & 0x01
				patternBit1 := (patternPlane1 >> (7 - (ppu.getBGX() & 0x07))) & 0x01
				pattern := (patternBit1 << 1) | patternBit0

				ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4] = pattern * 0x40
				ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+1] = pattern * 0x40
				ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+2] = pattern * 0x40
				ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+3] = pattern * 0x40
				ppu.LineX++
			}
		}
	case ppu.PPUCyclesSinceYInc == 340:
		ppu.PPUCyclesSinceYInc = 0
		ppu.LineX = 0
		ppu.LineY++
		if ppu.LineY == 260 {
			ppu.InVBlank = false
			ppu.VBlankAlert = false
			ppu.LineY = -1
		}
	}

	ppu.PPUCyclesSinceYInc++
}

func (ppu *ppu) writeControlReg(val byte) {
	boolsFromByte(val,
		&ppu.GenerateVBlankNMIs,
		&ppu.MasterSlaveExtSelector,
		&ppu.UseBigSprites,
		&ppu.UseUpperBGPatternTable,
		&ppu.UseUpperSpritePatternTable,
		&ppu.IncrementStyleSelector,
		nil, nil,
	)
	ppu.NametableBaseSelector = val & 0x03
	ppu.SharedReg = val
}
func (ppu *ppu) readControlReg() byte { return 0xff } // write only

func (ppu *ppu) writeStatusReg(val byte) {} // read only
func (ppu *ppu) readStatusReg() byte {
	result := byteFromBools(
		ppu.VBlankAlert,
		ppu.SpriteZeroHit,
		ppu.SpriteOverflow,
		false, false, false, false, false,
	)
	ppu.VBlankAlert = false
	ppu.ScrollRegSelector = 0
	ppu.AddrRegSelector = 0
	return result | (ppu.SharedReg & 0x1f)
}
