package famigo

type ppu struct {
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

	CyclesSinceYInc int
	LineY           int

	EmphasizeBlue           bool
	EmphasizeGreen          bool
	EmphasizeRed            bool
	ShowSprites             bool
	ShowBG                  bool
	ShowSpritesInLeftBorder bool
	ShowBGInLeftBorder      bool
	UseGreyscale            bool

	RequestedScrollX  byte
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
	if ppu.AddrReg >= 0x3f00 && ppu.AddrReg < 0x3f20 {
		ppu.PaletteRAM[ppu.AddrReg-0x3f00] = val
	} else {
		mem.MMC.WriteVRAM(mem, ppu.AddrReg, val)
	}
	if ppu.IncrementStyleSelector == incrementBigStride {
		ppu.AddrReg += 0x20
	} else {
		ppu.AddrReg++
	}
}
func (ppu *ppu) readDataReg(mem *mem) byte {
	var val byte
	if ppu.AddrReg >= 0x3f00 && ppu.AddrReg < 0x3f20 {
		val = ppu.PaletteRAM[ppu.AddrReg-0x3f00]
	} else {
		val = mem.MMC.ReadVRAM(mem, ppu.AddrReg)
	}
	if ppu.IncrementStyleSelector == incrementBigStride {
		ppu.AddrReg += 0x20
	} else {
		ppu.AddrReg++
	}
	return val
}

func (ppu *ppu) writeAddrReg(val byte) {
	if ppu.AddrRegSelector == 0 {
		ppu.AddrReg &^= 0xff00
		ppu.AddrReg |= uint16(val) << 8
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
		ppu.RequestedScrollX = val
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

func (ppu *ppu) runCycle(cs *cpuState) {
	switch ppu.CyclesSinceYInc {
	case 1:
		if ppu.LineY == 241 {
			ppu.InVBlank = true
			ppu.VBlankAlert = true
			if ppu.GenerateVBlankNMIs {
				cs.NMI = true
			}
		}
	case 257:
	case 321:
	case 337:
	case 340:
		ppu.CyclesSinceYInc = 0
		ppu.LineY++
		if ppu.LineY == 260 {
			ppu.InVBlank = false
			ppu.LineY = -1
		}
	}

	ppu.CyclesSinceYInc++
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
