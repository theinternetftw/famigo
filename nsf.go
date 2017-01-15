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
		emuErr(err)
	}
	if hdr.SoundChipFlags != 0 {
		emuErr("can't play nsf, needs this unimplemented chip:", hdr.SoundChipFlags)
	}

	fmt.Println()
	fmt.Println(string(hdr.SongName[:]))
	fmt.Println(string(hdr.ArtistName[:]))
	fmt.Println(string(hdr.CopyrightName[:]))
	fmt.Println("Track count:", hdr.NumSongs)
	fmt.Println()

	data := nsf[0x80:]

	var mapper mmc
	var cart []byte
	if hdr.usesBanks() {
		mapper = &mapper031{}
		padding := hdr.LoadAddr & 0x0fff
		cart = append(make([]byte, padding), data...)
	} else {
		if hdr.LoadAddr < 0x8000 {
			emuErr(fmt.Sprintf("we don't support sub-0x8000 LoadAddrs: %04x", hdr.LoadAddr))
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
	}
	np.init()

	np.initTune(np.CurrentSong)

	return &np
}

func (np *nsfPlayer) initTune(songNum byte) {

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

var lastTrackChange time.Time

func (np *nsfPlayer) UpdateInput(input Input) {
	// put e.g. track skip controls here
	now := time.Now()
	if now.Sub(lastTrackChange).Seconds() > 0.25 {
		if input.Joypad.Left {
			if np.CurrentSong > 0 {
				np.CurrentSong--
				np.initTune(np.CurrentSong)
			}
			lastTrackChange = now
		}
		if input.Joypad.Right {
			if np.CurrentSong < np.Hdr.NumSongs-1 {
				np.CurrentSong++
				np.initTune(np.CurrentSong)
			}
			lastTrackChange = now
		}
	}
}

func (np *nsfPlayer) Step() {
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
		np.runCycles(1)
	}
}
