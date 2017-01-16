package famigo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

type nsfPlayer struct {
	cpuState
	Hdr              nsfHeader
	PlayCallInterval float64
	LastPlayCall     time.Time
	CurrentSong      byte
	TvStdBit         byte
	Paused           bool
	DbgCursor        dbgCursor
	DbgScreen        [256 * 240 * 4]byte
	DbgFlipRequested bool
}

type nsfHeader struct {
	Magic          [5]byte
	Version        byte
	NumSongs       byte
	StartSong      byte
	LoadAddr       uint16
	InitAddr       uint16
	PlayAddr       uint16
	SongName       [32]byte
	ArtistName     [32]byte
	CopyrightName  [32]byte
	PlaySpeedNtsc  uint16
	BankVals       [8]byte
	PlaySpeedPal   uint16
	TvStdFlags     byte
	SoundChipFlags byte
	Reserved       [4]byte
}

func (hdr *nsfHeader) isNTSC() bool {
	return hdr.TvStdFlags&0x01 == 0 || hdr.TvStdFlags&0x02 == 0x02
}

func (hdr *nsfHeader) usesBanks() bool {
	for i := 0; i < 8; i++ {
		if hdr.BankVals[i] != 0 {
			return true
		}
	}
	return false
}

// NewNsfPlayer creates an nsfPlayer session
func NewNsfPlayer(nsf []byte) Emulator {
	hdr := nsfHeader{}
	if err := binary.Read(bytes.NewReader(nsf), binary.LittleEndian, &hdr); err != nil {
		return NewErrEmu(fmt.Sprintf("nsf player error\n%s", err.Error()))
	}
	if hdr.SoundChipFlags != 0 {
		return NewErrEmu(fmt.Sprintf("nsf player error\nunimplemented chip: %v", hdr.SoundChipFlags))
	}
	if hdr.Version != 1 {
		return NewErrEmu(fmt.Sprintf("nsf player error\nunsupported nsf version: %v", hdr.Version))
	}

	data := nsf[0x80:]

	var mapper mmc
	var cart []byte
	if hdr.usesBanks() {
		mapper = &mapper031{}
		padding := hdr.LoadAddr & 0x0fff
		cart = append(make([]byte, padding), data...)
	} else {
		if hdr.LoadAddr < 0x8000 {
			return NewErrEmu("unsupported nsf parameter\nsub-0x8000 LoadAddrs")
		}
		mapper = &mapper000{}
		padding := hdr.LoadAddr & 0x0fff
		cart = append(make([]byte, padding), data...)
	}

	var tvBit byte
	var playSpeed float64
	if hdr.isNTSC() {
		playSpeed = float64(hdr.PlaySpeedNtsc) / 1000000.0
		tvBit = 0
	} else {
		// just go with the slightly skewed timing
		// rather than completely fail...
		playSpeed = float64(hdr.PlaySpeedPal) / 1000000.0
		tvBit = 1
	}

	np := nsfPlayer{
		cpuState: cpuState{
			Mem: mem{
				MMC:    mapper,
				PrgROM: cart,
				ChrROM: make([]byte, 8192),
				PrgRAM: make([]byte, 8192),
			},
		},
		PlayCallInterval: playSpeed,
		Hdr:              hdr,
		CurrentSong:      hdr.StartSong - 1,
		TvStdBit:         tvBit,
		DbgCursor:        dbgCursor{w: 256, h: 240},
	}
	np.init()

	np.DbgCursor.newline()
	np.DbgCursor.writeString(np.DbgScreen[:], "NSF Player\n")
	np.DbgCursor.newline()
	np.DbgCursor.writeString(np.DbgScreen[:], "Title: "+string(hdr.SongName[:])+"\n")
	np.DbgCursor.writeString(np.DbgScreen[:], "Artist: "+string(hdr.ArtistName[:])+"\n")
	np.DbgCursor.writeString(np.DbgScreen[:], string(hdr.CopyrightName[:])+"\n")
	np.DbgFlipRequested = true

	np.initTune(np.CurrentSong)

	return &np
}

func (np *nsfPlayer) initTune(songNum byte) {

	np.DbgCursor.y = 8 * 7
	np.DbgCursor.x = 0
	np.DbgCursor.clearLine(np.DbgScreen[:])
	np.DbgCursor.writeString(np.DbgScreen[:], fmt.Sprintf("Track %02d/%02d", songNum+1, np.Hdr.NumSongs))
	np.DbgFlipRequested = true

	for addr := uint16(0x0000); addr < 0x0800; addr++ {
		np.write(addr, 0x00)
	}
	for addr := uint16(0x6000); addr < 0x8000; addr++ {
		np.write(addr, 0x00)
	}
	for addr := uint16(0x4000); addr < 0x4014; addr++ {
		np.write(addr, 0x00)
	}
	np.write(0x4015, 0x0f)
	np.write(0x4017, 0x40)

	if np.Hdr.usesBanks() {
		for i := uint16(0); i < 8; i++ {
			np.write(0x5ff8+i, np.Hdr.BankVals[i])
		}
	}

	np.A = songNum
	np.X = np.TvStdBit // should usually be 0 for ntsc

	// force a RESET-y call to INIT
	np.S = 0xfd
	np.push16(0x0000)
	np.P |= flagIrqDisabled
	np.PC = np.Hdr.InitAddr
	for np.PC != 0x0001 {
		np.step()
	}
}

var lastInput time.Time

func (np *nsfPlayer) UpdateInput(input Input) {
	// put e.g. track skip controls here
	now := time.Now()
	if now.Sub(lastInput).Seconds() > 0.25 {
		if input.Joypad.Left {
			if np.CurrentSong > 0 {
				np.CurrentSong--
				np.initTune(np.CurrentSong)
			}
			lastInput = now
		}
		if input.Joypad.Right {
			if np.CurrentSong < np.Hdr.NumSongs-1 {
				np.CurrentSong++
				np.initTune(np.CurrentSong)
			}
			lastInput = now
		}
		if input.Joypad.Start {
			np.Paused = !np.Paused
			lastInput = now
		}
	}
}

func (np *nsfPlayer) Step() {
	if !np.Paused {
		now := time.Now()
		if np.PC == 0x0001 {
			if now.Sub(np.LastPlayCall).Seconds() >= np.PlayCallInterval {
				np.LastPlayCall = now
				np.S = 0xfd
				np.push16(0x0000)
				np.PC = np.Hdr.PlayAddr
			}
		}
		if np.PC != 0x0001 {
			np.step()
		} else {
			np.runCycles(2)
		}
	}
}

func (np *nsfPlayer) Framebuffer() []byte {
	return np.DbgScreen[:]
}

func (np *nsfPlayer) FlipRequested() bool {
	result := np.DbgFlipRequested
	np.DbgFlipRequested = false
	return result
}
