package famigo

type ppu struct {
	FrameBuffer [256 * 240 * 4]byte

	GenerateVBlankNMIs         bool
	MasterSlaveExtSelector     bool
	UseBigSprites              bool
	UseUpperBGPatternTable     bool
	UseUpperSpritePatternTable bool
	IncrementStyleSelector     bool

	ManuallyGenerateNMI               bool
	ManuallyGenerateNMIWaitingForStep bool
	ManuallyGenerateNMIStepRequested  uint64

	TempAddrReg uint16 // handles scroll, nametables... see ppu docs
	AddrReg     uint16
	FineScrollX byte

	AddrRegSelector byte
	DataReadBuffer  byte

	VBlankAlert     bool
	LastVBlankReset uint64
	SpriteZeroHit   bool
	SpriteOverflow  bool // misnomer: complicated role, due to hw bugs

	SharedReg byte

	PPUCycles          uint64
	PPUCyclesSinceYInc int
	LineY              int
	LineX              int

	CurrentNametableByte byte
	CurrentAttributeByte byte
	CurrentTileLowByte   byte
	CurrentTileHighByte  byte

	EmphasizeBlue           bool
	EmphasizeGreen          bool
	EmphasizeRed            bool
	ShowSprites             bool
	ShowBG                  bool
	ShowSpritesInLeftBorder bool
	ShowBGInLeftBorder      bool
	UseGreyscale            bool

	OAM            [256]byte
	OAMBeingParsed []oamEntry
	OAMForScanline []oamEntry

	OAMAddrReg byte

	PaletteRAM [32]byte

	FrameCounter uint
}

type oamEntry struct {
	X, Y                byte
	TileField           byte
	FlipY               bool
	FlipX               bool
	BehindBG            bool
	PaletteID           byte
	OAMIndex            byte
	PatternsForScanline [8]byte
}

func (ppu *ppu) xInRange(spriteX, testX byte) bool {
	return testX >= spriteX && int(testX) < int(spriteX)+8
}

func (ppu *ppu) yInRange(spriteY, testY int) bool {
	height := 8
	if ppu.UseBigSprites {
		height = 16
	}
	return testY >= spriteY && testY < spriteY+height
}

func (ppu *ppu) parseOAM() {
	ppu.SpriteOverflow = false
	ppu.OAMBeingParsed = ppu.OAMBeingParsed[:0]
	for i := 0; len(ppu.OAMBeingParsed) < 9 && i < 64; i++ {
		spriteY := ppu.OAM[i*4]
		if ppu.yInRange(int(spriteY)+1, ppu.LineY+1) {
			if len(ppu.OAMBeingParsed) < 8 {
				tileField := ppu.OAM[i*4+1]
				attrByte := ppu.OAM[i*4+2]
				ppu.OAMBeingParsed = append(ppu.OAMBeingParsed, oamEntry{
					Y:         spriteY + 1,
					TileField: tileField,
					FlipY:     attrByte&0x80 == 0x80,
					FlipX:     attrByte&0x40 == 0x40,
					BehindBG:  attrByte&0x20 == 0x20,
					PaletteID: attrByte & 0x03,
					X:         ppu.OAM[i*4+3],
					OAMIndex:  byte(i),
				})
			} else {
				// NOTE: overflow hw bug not yet implemented!
				ppu.SpriteOverflow = true
				break
			}
		}
	}
}

func (ppu *ppu) getPatternDataForParsedOAM(cs *cpuState, y byte) {
	for i := range ppu.OAMBeingParsed {
		entry := &ppu.OAMBeingParsed[i]
		tileID := entry.TileField
		var spriteY byte
		height := byte(8)
		if ppu.UseBigSprites {
			height = 16
		}
		if entry.FlipY {
			spriteY = (height - 1) - (y - entry.Y)
		} else {
			spriteY = y - entry.Y
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
		entry.PatternsForScanline = ppu.getPatternsForSpriteAtY(cs, patternAddr, spriteY)
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

func getPaletteRAMAddr(addr uint16) uint16 {
	switch addr {
	case 0x3f10, 0x3f14, 0x3f18, 0x3f1c:
		return (addr - 0x3f00) & 0x0f
	}
	return (addr - 0x3f00) & 0x1f
}

func (ppu *ppu) writeDataReg(mem *mem, val byte) {

	addr := ppu.AddrReg & 0x3fff // NOTE: make sure this mask isn't hiding bugs!
	if addr >= 0x3f00 && addr < 0x4000 {
		addr = getPaletteRAMAddr(addr)
		ppu.PaletteRAM[addr] = val
	} else if addr >= 0x2000 && addr < 0x3f00 {
		addr = addr & 0x2fff
		mem.mmc.WriteVRAM(mem, addr, val)
	} else {
		mem.mmc.WriteVRAM(mem, addr, val)
	}
	if ppu.IncrementStyleSelector == incrementBigStride {
		ppu.AddrReg += 0x20
	} else {
		ppu.AddrReg++
	}
	ppu.AddrReg &= 0x7fff // only a 15 bit reg
}

func (ppu *ppu) readDataReg(mem *mem) byte {
	var val byte
	addr := ppu.AddrReg & 0x3fff // NOTE: make sure this mask isn't hiding bugs!
	if addr >= 0x3f00 && addr < 0x4000 {
		addr = getPaletteRAMAddr(addr)
		// palette data is returned, but data buffer is updated to nametable values
		ppu.DataReadBuffer = mem.mmc.ReadVRAM(mem, addr&0x2fff)
		val = ppu.PaletteRAM[addr]
	} else if addr >= 0x2000 && addr < 0x3f00 {
		addr = addr & 0x2fff
		val = ppu.DataReadBuffer
		ppu.DataReadBuffer = mem.mmc.ReadVRAM(mem, addr)
	} else {
		val = ppu.DataReadBuffer
		ppu.DataReadBuffer = mem.mmc.ReadVRAM(mem, addr)
	}
	if ppu.IncrementStyleSelector == incrementBigStride {
		ppu.AddrReg += 0x20
	} else {
		ppu.AddrReg++
	}
	ppu.AddrReg &= 0x7fff // only a 15 bit reg
	return val
}

func (ppu *ppu) writeAddrReg(val byte) {
	if ppu.AddrRegSelector == 0 {
		ppu.TempAddrReg &^= 0xff00
		ppu.TempAddrReg |= uint16(val&0x3f) << 8 // yes 3, we clear the top scroll bit for some reason, here
		ppu.AddrRegSelector = 1
	} else {
		ppu.TempAddrReg &^= 0x00ff
		ppu.TempAddrReg |= uint16(val)
		ppu.AddrReg = ppu.TempAddrReg
		ppu.AddrRegSelector = 0
	}
}
func (ppu *ppu) readAddrReg() byte {
	return 0xff // write only
}

func (ppu *ppu) writeScrollReg(val byte) {
	if ppu.AddrRegSelector == 0 {
		ppu.TempAddrReg &^= 0x1f
		ppu.TempAddrReg |= uint16(val >> 3)
		ppu.FineScrollX = val & 0x07
		ppu.AddrRegSelector = 1
	} else {
		ppu.TempAddrReg &^= 0x03e0
		ppu.TempAddrReg |= uint16(val>>3) << 5
		ppu.TempAddrReg &^= 0x7000
		ppu.TempAddrReg |= uint16(val&0x07) << 12
		ppu.AddrRegSelector = 0
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

func (ppu *ppu) getCurrentNametableTileAddr() uint16 {
	return 0x2000 | ppu.AddrReg&0x0fff // 0x2000 | nametableSel | coarseY | coarseX
}
func (ppu *ppu) getCurrentNametableByte(cs *cpuState) byte {
	return ppu.read(cs, ppu.getCurrentNametableTileAddr())
}

func (ppu *ppu) getCurrentNametableAttributeAddr() uint16 {
	addr := 0x23c0 | (ppu.AddrReg & 0x0c00)
	addr |= ((ppu.AddrReg >> 5 >> 2) & 0x07) << 3 // high 3 bits of coarse y
	addr |= (ppu.AddrReg >> 2) & 0x07             // high 3 bits of coarse x
	return addr
}
func (ppu *ppu) getCurrentAttributeByte(cs *cpuState) byte {
	return ppu.read(cs, ppu.getCurrentNametableAttributeAddr())
}

func (ppu *ppu) getBGPatternAddr(tileID byte) uint16 {
	addr := uint16(tileID) << 4
	if ppu.UseUpperBGPatternTable {
		addr |= 0x1000
	}
	return addr
}
func (ppu *ppu) getCurrentTileBytes(cs *cpuState, tileID byte) (byte, byte) {
	patternAddr := ppu.getBGPatternAddr(tileID) + uint16(ppu.getFineScrollY()&0x07)
	patternPlane0 := ppu.read(cs, patternAddr)
	patternPlane1 := ppu.read(cs, patternAddr+8)
	return patternPlane0, patternPlane1
}

func (ppu *ppu) getPatternBG(x byte) byte {
	patternBit0 := (ppu.CurrentTileLowByte >> (7 - (x & 0x07))) & 0x01
	patternBit1 := (ppu.CurrentTileHighByte >> (7 - (x & 0x07))) & 0x01
	return byte((patternBit1 << 1) | patternBit0)
}

func (ppu *ppu) getPatternsForSpriteAtY(cs *cpuState, patternAddr uint16, y byte) [8]byte {
	patternAddr += uint16(y & 0x07)
	patternPlane0 := ppu.read(cs, patternAddr)
	patternPlane1 := ppu.read(cs, patternAddr+8)
	result := [8]byte{}
	for x := byte(0); x < 8; x++ {
		patternBit0 := (patternPlane0 >> (7 - (x & 0x07))) & 0x01
		patternBit1 := (patternPlane1 >> (7 - (x & 0x07))) & 0x01
		result[x] = byte((patternBit1 << 1) | patternBit0)
	}
	return result
}

func (ppu *ppu) read(cs *cpuState, addr uint16) byte {
	return cs.Mem.mmc.ReadVRAM(&cs.Mem, addr)
}

var defaultPalette = ntscPaletteSat

func (ppu *ppu) getRGB(nesColor byte) (byte, byte, byte) {
	if ppu.UseGreyscale {
		nesColor &= 0x30
	}
	emphasisSelector := uint(0)
	if ppu.EmphasizeRed {
		emphasisSelector |= 1
	}
	if ppu.EmphasizeGreen {
		emphasisSelector |= 2
	}
	if ppu.EmphasizeBlue {
		emphasisSelector |= 4
	}
	ntscPalIndex := uint(nesColor)
	return defaultPalette[emphasisSelector*64*3+ntscPalIndex*3],
		defaultPalette[emphasisSelector*64*3+ntscPalIndex*3+1],
		defaultPalette[emphasisSelector*64*3+ntscPalIndex*3+2]
}

func (ppu *ppu) getPaletteIDFromAttributeByte(attributes byte, tileX, tileY byte) byte {
	attrX := (tileX >> 1) & 0x01
	attrY := (tileY >> 1) & 0x01
	return (attributes >> (attrX * 2) >> (attrY * 4)) & 0x03
}

func (ppu *ppu) getBackgroundColor() byte {
	if !ppu.ShowBG && !ppu.ShowSprites {
		if ppu.AddrReg >= 0x3f00 && ppu.AddrReg < 0x4000 {
			// activate background color hack
			return ppu.PaletteRAM[getPaletteRAMAddr(ppu.AddrReg)]
		}
	}
	// otherwise we use the universal background color
	return ppu.PaletteRAM[0]
}

func (ppu *ppu) copyVerticalScrollBits() {
	ppu.AddrReg &^= 0xfbe0 // x1111.11111..... (x == dont care)
	ppu.AddrReg |= ppu.TempAddrReg & 0xfbe0
}
func (ppu *ppu) copyHorizontalScrollBits() {
	ppu.AddrReg &^= 0x041f // x....1.....11111 (x == dont care)
	ppu.AddrReg |= ppu.TempAddrReg & 0x041f
}
func (ppu *ppu) incrementVerticalScrollBits() {
	if ppu.AddrReg&0x7000 != 0x7000 {
		ppu.AddrReg += 0x1000
	} else {
		ppu.AddrReg &^= 0x7000
		tileY := (ppu.AddrReg & 0x03e0) >> 5
		switch tileY {
		case 29:
			ppu.AddrReg ^= 0x0800 // nametable swap
			tileY = 0
		case 31:
			tileY = 0
		default:
			tileY++
		}
		ppu.AddrReg &^= 0x03e0
		ppu.AddrReg |= tileY << 5
	}
}
func (ppu *ppu) incrementHorizontalScrollBits() {
	if ppu.AddrReg&0x001f == 0x001f {
		ppu.AddrReg &^= 0x001f
		ppu.AddrReg ^= 0x0400 // nametable swap
	} else {
		ppu.AddrReg++
	}
}
func (ppu *ppu) getBGTileX() byte     { return byte(ppu.AddrReg) & 0x1f }
func (ppu *ppu) getBGTileY() byte     { return byte(ppu.AddrReg>>5) & 0x1f }
func (ppu *ppu) getFineScrollY() byte { return byte(ppu.AddrReg>>12) & 0x07 }

// FIXME: (hopefully) temp hack due to timing issues (games setting fineX before we're ready)
var fineScrollXCopy byte

func (ppu *ppu) runCycle(cs *cpuState) {

	if ppu.ManuallyGenerateNMI {
		ppu.ManuallyGenerateNMI = false
		ppu.ManuallyGenerateNMIWaitingForStep = true
		ppu.ManuallyGenerateNMIStepRequested = cs.Steps
	}
	if ppu.ManuallyGenerateNMIWaitingForStep {
		if cs.Steps > ppu.ManuallyGenerateNMIStepRequested {
			ppu.ManuallyGenerateNMIWaitingForStep = false
			cs.NMI = true
		}
	}

	if ppu.PPUCyclesSinceYInc == 0 {
		if ppu.LineY == -1 && (ppu.ShowBG || ppu.ShowSprites) && ppu.FrameCounter&0x01 == 0x01 {
			ppu.PPUCyclesSinceYInc++ // skip 0th cycle
		}
	}

	switch ppu.PPUCyclesSinceYInc {
	case 1:
		if ppu.LineY == 241 {
			ppu.FrameCounter++
			if ppu.LastVBlankReset != ppu.PPUCycles {
				ppu.VBlankAlert = true
				if ppu.GenerateVBlankNMIs {
					cs.NMI = true
				}
			}
			cs.flipRequested = true
		} else if ppu.LineY == -1 {
			ppu.VBlankAlert = false
			ppu.SpriteZeroHit = false
		} else if ppu.LineY >= 0 && ppu.LineY < 240 {
			ppu.OAMForScanline = ppu.OAMForScanline[:0]
			ppu.OAMForScanline = append(ppu.OAMForScanline, ppu.OAMBeingParsed...)
			ppu.parseOAM()
		}
	case 257:
		if ppu.LineY >= -1 && ppu.LineY < 240 {
			ppu.getPatternDataForParsedOAM(cs, byte(ppu.LineY+1))
			if ppu.ShowBG || ppu.ShowSprites {
				ppu.copyHorizontalScrollBits()
				fineScrollXCopy = ppu.FineScrollX
			}
		}
	case 304:
		if ppu.LineY == -1 {
			// NOTE: technically, happens over and over from cycles 280-304
			if ppu.ShowBG || ppu.ShowSprites {
				ppu.copyVerticalScrollBits()
				ppu.copyHorizontalScrollBits()
				fineScrollXCopy = ppu.FineScrollX
			}
		}
	case 341:
		ppu.PPUCyclesSinceYInc = 0
		ppu.LineX = 0
		ppu.LineY++
		if ppu.LineY == 261 {
			ppu.LineY = -1
		}
	}

	if ppu.PPUCyclesSinceYInc >= 1 && ppu.PPUCyclesSinceYInc <= 256 {
		if ppu.LineY >= -1 && ppu.LineY < 240 && ppu.PPUCyclesSinceYInc&0x07 == 0 {

			ppu.CurrentNametableByte = ppu.getCurrentNametableByte(cs)
			ppu.CurrentAttributeByte = ppu.getCurrentAttributeByte(cs)
			ppu.CurrentTileLowByte, ppu.CurrentTileHighByte = ppu.getCurrentTileBytes(cs, ppu.CurrentNametableByte)

			for i := 0; i < 8; i++ {

				color := ppu.getBackgroundColor() & 0x3f
				bgPattern := byte(0)

				if ppu.ShowBG && (ppu.LineX >= 8 || ppu.ShowBGInLeftBorder) {
					bgPattern = ppu.getPatternBG(byte(ppu.LineX) + fineScrollXCopy)
					if bgPattern != 0 {
						attributeByte := ppu.CurrentAttributeByte
						paletteID := ppu.getPaletteIDFromAttributeByte(attributeByte, ppu.getBGTileX(), ppu.getBGTileY())
						colorAddr := (paletteID << 2) | bgPattern
						color = ppu.PaletteRAM[colorAddr] & 0x3f
					}
				}

				if ppu.ShowSprites && (ppu.LineX >= 8 || ppu.ShowSpritesInLeftBorder) {
					x := byte(ppu.LineX)
					for i := 0; i < len(ppu.OAMForScanline); i++ {
						entry := ppu.OAMForScanline[i]
						if ppu.xInRange(entry.X, x) {
							var spriteX byte
							if entry.FlipX {
								spriteX = 7 - (x - entry.X)
							} else {
								spriteX = x - entry.X
							}
							pattern := entry.PatternsForScanline[spriteX&0x07]
							if pattern != 0 {
								if entry.OAMIndex == 0 {
									if ppu.ShowBG && ppu.LineX != 255 {
										if bgPattern != 0 && ppu.PPUCyclesSinceYInc >= 2 {
											ppu.SpriteZeroHit = true
										}
									}
								}
								if !entry.BehindBG || bgPattern == 0 {
									colorAddr := 0x10 | (entry.PaletteID << 2) | pattern
									color = ppu.PaletteRAM[colorAddr] & 0x3f
								}
								break // the algo stops on non-transparency whether a pixel was drawn or not...
							}
						}
					}
				}

				if ppu.LineY != -1 {
					r, g, b := ppu.getRGB(color)
					ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4] = r
					ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+1] = g
					ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+2] = b
					ppu.FrameBuffer[ppu.LineY*256*4+ppu.LineX*4+3] = 0xff

					ppu.LineX++
					if (byte(ppu.LineX)+fineScrollXCopy)&0x07 == 0 {
						if ppu.ShowBG || ppu.ShowSprites {
							ppu.incrementHorizontalScrollBits()

							ppu.CurrentNametableByte = ppu.getCurrentNametableByte(cs)
							ppu.CurrentAttributeByte = ppu.getCurrentAttributeByte(cs)
							ppu.CurrentTileLowByte, ppu.CurrentTileHighByte = ppu.getCurrentTileBytes(cs, ppu.CurrentNametableByte)
						}
					}
				}
			}
			if ppu.PPUCyclesSinceYInc == 256 {
				if ppu.ShowBG || ppu.ShowSprites {
					if ppu.PPUCyclesSinceYInc == 256 {
						ppu.incrementVerticalScrollBits()
					}
					// NOTE: seems and acts wrong, but is perscribed in nesdev wiki
					// if ppu.PPUCyclesSinceYInc == 328 || ppu.PPUCyclesSinceYInc == 336 {
					// 	ppu.incrementHorizontalScrollBits()
					// }
				}
			}
		}
	}

	ppu.PPUCycles++
	ppu.PPUCyclesSinceYInc++
}

func (ppu *ppu) setNametableSelector(val byte) {
	ppu.TempAddrReg &^= 0x0c00
	ppu.TempAddrReg |= uint16(val&0x03) << 10
}

func (ppu *ppu) writeControlReg(val byte) {
	wasGenningVBlank := ppu.GenerateVBlankNMIs
	boolsFromByte(val,
		&ppu.GenerateVBlankNMIs,
		&ppu.MasterSlaveExtSelector,
		&ppu.UseBigSprites,
		&ppu.UseUpperBGPatternTable,
		&ppu.UseUpperSpritePatternTable,
		&ppu.IncrementStyleSelector,
		nil, nil,
	)
	if !wasGenningVBlank && ppu.GenerateVBlankNMIs {
		if ppu.VBlankAlert {
			ppu.ManuallyGenerateNMI = true
		}
	}
	ppu.setNametableSelector(val & 0x03)
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
	ppu.LastVBlankReset = ppu.PPUCycles
	ppu.AddrRegSelector = 0
	return result | (ppu.SharedReg & 0x1f)
}
