package famigo

import "fmt"

// CartInfo represents a nes cart header
type CartInfo struct {
	PrgROMSizeCode byte
	ChrROMSizeCode byte
	Flags6         byte
	Flags7         byte

	// iNES 1.0 only
	PrgRAMSizeCode byte

	// iNES 2.0 only
	Flags8  byte
	Flags9  byte
	Flags10 byte
	Flags11 byte
	Flags12 byte
	Flags13 byte

	IsNES2 bool
}

// ParseCartInfo parses a nes cart header
func ParseCartInfo(cartBytes []byte) (*CartInfo, error) {
	if len(cartBytes) < 16 {
		return nil, fmt.Errorf("rom file too short")
	}
	if string(cartBytes[:3]) != "NES" || cartBytes[3] != 0x1a {
		return nil, fmt.Errorf("unknown rom file format")
	}

	cart := CartInfo{
		PrgROMSizeCode: cartBytes[4],
		ChrROMSizeCode: cartBytes[5],
		Flags6:         cartBytes[6],
		Flags7:         cartBytes[7],
		Flags8:         cartBytes[8],
		Flags9:         cartBytes[9],
		Flags10:        cartBytes[10],
		Flags11:        cartBytes[11],
		Flags12:        cartBytes[12],
		Flags13:        cartBytes[13],
	}
	if cart.Flags7&0xc0 == 0x80 {
		cart.IsNES2 = true
	} else {
		cart.PrgRAMSizeCode = cartBytes[8]
	}

	return &cart, nil
}

// MirrorInfo needs docs
type MirrorInfo int

const (
	// HorizontalMirroring needs docs
	HorizontalMirroring MirrorInfo = iota
	// VerticalMirroring needs docs
	VerticalMirroring
	// FourScreenVRAM needs docs
	FourScreenVRAM

	// only for MMCs:

	// OneScreenLowerMirroring needs docs
	OneScreenLowerMirroring
	// OneScreenUpperMirroring needs docs
	OneScreenUpperMirroring
)

// GetMirrorInfo needs docs
func (cart *CartInfo) GetMirrorInfo() MirrorInfo {
	if cart.Flags6&0x08 == 0x08 {
		return FourScreenVRAM
	}
	if cart.Flags6&0x01 == 0x01 {
		return VerticalMirroring
	}
	return HorizontalMirroring
}

// HasBatteryBackedRAM needs docs
func (cart *CartInfo) HasBatteryBackedRAM() bool {
	return cart.Flags6&0x02 != 0
}

// HasTrainer needs docs
func (cart *CartInfo) HasTrainer() bool {
	return cart.Flags6&0x04 != 0
}

// GetMapperNumber needs docs
func (cart *CartInfo) GetMapperNumber() int {
	low := cart.Flags6 >> 4
	high := cart.Flags7 & 0xf0
	return int(high | low)
}

// GetROMSizePrg needs docs
func (cart *CartInfo) GetROMSizePrg() int {
	if cart.IsNES2 {
		panic("must be updated for nes2.0")
	}
	return int(cart.PrgROMSizeCode) * 16 * 1024
}

// GetROMSizeChr needs docs
func (cart *CartInfo) GetROMSizeChr() int {
	if cart.IsNES2 {
		panic("must be updated for nes2.0")
	}
	return int(cart.ChrROMSizeCode) * 8 * 1024
}

// IsChrRAM needs docs
func (cart *CartInfo) IsChrRAM() bool {
	if cart.IsNES2 {
		panic("must be updated for nes2.0")
	}
	return cart.ChrROMSizeCode == 0
}

// GetRAMSizeChr needs docs
func (cart *CartInfo) GetRAMSizeChr() int {
	if cart.IsNES2 {
		panic("must be updated for nes2.0")
	}
	if cart.IsChrRAM() {
		return 8 * 1024
	}
	return 0
}

// GetRAMSizePrg needs docs
func (cart *CartInfo) GetRAMSizePrg() int {
	if cart.IsNES2 {
		panic("must be updated for nes2.0")
	}
	if int(cart.PrgRAMSizeCode) == 0 {
		return 8 * 1024
	}
	return int(cart.PrgRAMSizeCode) * 8 * 1024
}

// GetROMOffsetPrg needs docs
func (cart *CartInfo) GetROMOffsetPrg() int {
	offs := 16
	if cart.HasTrainer() {
		offs += 512
	}
	return offs
}

// GetROMOffsetChr needs docs
func (cart *CartInfo) GetROMOffsetChr() int {
	offs := 16
	if cart.HasTrainer() {
		offs += 512
	}
	offs += cart.GetROMSizePrg()
	return offs
}
