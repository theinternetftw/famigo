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

	OAM            [256]byte
	OAMBeingParsed []oamEntry
	OAMForScanline []oamEntry

	OAMAddrReg byte

	PaletteRAM [32]byte
}

type oamEntry struct {
	X, Y      byte
	TileField byte
	FlipY     bool
	FlipX     bool
	BehindBG  bool
	PaletteID byte
	OAMIndex  byte
}

func (ppu *ppu) xInRange(spriteX, testX byte) bool {
	return testX >= spriteX && testX < spriteX+8
}

func (ppu *ppu) yInRange(spriteY, testY byte) bool {
	height := byte(8)
	if ppu.UseBigSprites {
		height = 16
	}
	return testY >= spriteY && testY < spriteY+height
}

func (ppu *ppu) parseOAM() {
	ppu.OAMBeingParsed = ppu.OAMBeingParsed[:0]
	for i := 0; len(ppu.OAMBeingParsed) < 8 && i < 64; i++ {
		spriteY := ppu.OAM[i*4]
		if ppu.yInRange(spriteY, ppu.getY()+1) {
			tileField := ppu.OAM[i*4+1]
			attrByte := ppu.OAM[i*4+2]
			ppu.OAMBeingParsed = append(ppu.OAMBeingParsed, oamEntry{
				Y:         spriteY,
				TileField: tileField,
				FlipY:     attrByte&0x80 == 0x80,
				FlipX:     attrByte&0x40 == 0x40,
				BehindBG:  attrByte&0x20 == 0x20,
				PaletteID: attrByte & 0x03,
				X:         ppu.OAM[i*4+3],
				OAMIndex:  byte(i),
			})
		}
	}
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
	return ppu.getNametableBase() +
		uint16(ppu.getY()>>3)*32 + uint16(ppu.getX()>>3)
}
func (ppu *ppu) getCurrentNametableAttributeAddr() uint16 {
	return (ppu.getCurrentNametableTileAddr() & 0x3c00) + (0x400 - 64) +
		uint16((ppu.getY()&0xff)>>5)*8 + uint16((ppu.getX()&0xff)>>5)
}
func (ppu *ppu) getBGPatternAddr(tileID byte) uint16 {
	addr := uint16(tileID) << 4
	if ppu.UseUpperBGPatternTable {
		addr |= 0x1000
	}
	return addr
}

func (ppu *ppu) getX() byte { return byte(ppu.LineX) + ppu.ScrollX }
func (ppu *ppu) getY() byte { return byte(ppu.LineY) + ppu.ScrollY }

func (ppu *ppu) getPattern(cs *cpuState, patternAddr uint16, x, y byte) byte {
	patternAddr += uint16(y & 0x07)
	patternPlane0 := ppu.read(cs, patternAddr)
	patternPlane1 := ppu.read(cs, patternAddr+8)
	patternBit0 := (patternPlane0 >> (7 - (x & 0x07))) & 0x01
	patternBit1 := (patternPlane1 >> (7 - (x & 0x07))) & 0x01
	return byte((patternBit1 << 1) | patternBit0)
}

func (ppu *ppu) read(cs *cpuState, addr uint16) byte {
	return cs.Mem.MMC.ReadVRAM(&cs.Mem, addr)
}

var defaultPalette = []byte{
	84, 84, 84, 0, 30, 116, 8, 16, 144, 48, 0, 136, 68, 0, 100, 92, 0, 48, 84, 4, 0, 60, 24, 0,
	32, 42, 0, 8, 58, 0, 0, 64, 0, 0, 60, 0, 0, 50, 60, 0, 0, 0, 152, 150, 152, 8, 76, 196,
	48, 50, 236, 92, 30, 228, 136, 20, 176, 160, 20, 100, 152, 34, 32, 120, 60, 0, 84, 90, 0, 40, 114, 0,
	8, 124, 0, 0, 118, 40, 0, 102, 120, 0, 0, 0, 236, 238, 236, 76, 154, 236, 120, 124, 236, 176, 98, 236,
	228, 84, 236, 236, 88, 180, 236, 106, 100, 212, 136, 32, 160, 170, 0, 116, 196, 0, 76, 208, 32, 56, 204, 108,
	56, 180, 204, 60, 60, 60, 236, 238, 236, 168, 204, 236, 188, 188, 236, 212, 178, 236, 236, 174, 236, 236, 174, 212,
	236, 180, 176, 228, 196, 144, 204, 210, 120, 180, 222, 120, 168, 226, 144, 152, 226, 180, 160, 214, 228, 160, 162, 160,
}

func (ppu *ppu) getPaletteIDFromAttributeByte(attributes byte, x, y byte) byte {
	attrX := (x >> 4) & 0x01
	attrY := (y >> 4) & 0x01
	return (attributes >> (attrX * 2) >> (attrY * 4)) & 0x03
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
			ppu.SpriteZeroHit = false
		} else if ppu.LineY >= 0 && ppu.LineY < 240 {
			ppu.OAMForScanline = ppu.OAMForScanline[:0]
			ppu.OAMForScanline = append(ppu.OAMForScanline, ppu.OAMBeingParsed...)
			ppu.parseOAM()
		}

	case ppu.PPUCyclesSinceYInc >= 1 && ppu.PPUCyclesSinceYInc <= 256:
		if ppu.LineY >= 0 && ppu.LineY < 240 && ppu.PPUCyclesSinceYInc&0x07 == 0 {
			for i := 0; i < 8; i++ {

				x, y := ppu.getX(), ppu.getY()
				r, g, b := byte(0), byte(0), byte(0)
				bgPattern := byte(0)
				if ppu.ShowBG {
					tileID := ppu.read(cs, ppu.getCurrentNametableTileAddr())
					patternAddr := ppu.getBGPatternAddr(tileID)
					bgPattern = ppu.getPattern(cs, patternAddr, x, y)
					var color byte
					if bgPattern == 0 {
						color = ppu.PaletteRAM[0] // universal background color
					} else {
						attributeByte := ppu.read(cs, ppu.getCurrentNametableAttributeAddr())
						paletteID := ppu.getPaletteIDFromAttributeByte(attributeByte, x, y)
						colorAddr := (paletteID << 2) | bgPattern
						color = ppu.PaletteRAM[colorAddr] & 0x3f
					}
					r = defaultPalette[color*3]
					g = defaultPalette[color*3+1]
					b = defaultPalette[color*3+2]
				}

				if ppu.ShowSprites {
					for i := len(ppu.OAMForScanline) - 1; i >= 0; i-- {
						entry := ppu.OAMForScanline[i]
						if ppu.xInRange(entry.X, x) {
							tileID := entry.TileField
							var spriteY, spriteX byte
							height := byte(8)
							if ppu.UseBigSprites {
								height = 16
							}
							if entry.FlipY {
								spriteY = (height - 1) - (y - entry.Y)
							} else {
								spriteY = y - entry.Y
							}
							if entry.FlipX {
								spriteX = 7 - (x - entry.X)
							} else {
								spriteX = x - entry.X
							}
							patternTbl := uint16(0x0000)
							if ppu.UseUpperSpritePatternTable || (ppu.UseBigSprites && (tileID&0x01 == 0x01)) {
								patternTbl = 0x1000
							}
							if ppu.UseBigSprites {
								if spriteY >= 8 {
									tileID |= 0x01
								} else {
									tileID &^= 0x01
								}
							}
							patternAddr := patternTbl | (uint16(tileID) << 4)
							pattern := ppu.getPattern(cs, patternAddr, spriteX, spriteY)
							if pattern != 0 {
								if entry.OAMIndex == 0 && bgPattern != 0 {
									ppu.SpriteZeroHit = true
								}
								colorAddr := 0x10 | (entry.PaletteID << 2) | pattern
								color := ppu.PaletteRAM[colorAddr] & 0x3f
								r = defaultPalette[color*3]
								g = defaultPalette[color*3+1]
								b = defaultPalette[color*3+2]
							}
						}
					}
				}

				ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4] = r
				ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+1] = g
				ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+2] = b
				ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+3] = 0xff

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
