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
	HdrExtended      *parsedNsfe
	PlayCallInterval float64
	LastPlayCall     time.Time
	CurrentSong      byte
	TvStdBit         byte
	Paused           bool
	DbgTerminal      dbgTerminal
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

type parsedNsfe struct {
	info infoChunk
	data []byte
	bank *bankChunk
	plst *plstChunk
	fade *fadeChunk
	tlbl *tlblChunk
	auth *authChunk
	text *textChunk
}

type chunkHdr struct {
	ChunkLen uint32
	Fourcc   [4]byte
}

const (
	defaultSpeedPal    = 19997
	defaultSpeedNtsc   = 16639
	defaultSpeedAPUIRQ = 16666
)

func (p *parsedNsfe) getNsfHeader() nsfHeader {
	hdr := nsfHeader{
		LoadAddr:       p.info.LoadAddr,
		InitAddr:       p.info.InitAddr,
		PlayAddr:       p.info.PlayAddr,
		TvStdFlags:     p.info.TvStdFlags,
		SoundChipFlags: p.info.SoundChipFlags,
		NumSongs:       p.info.NumSongs,
		StartSong:      p.info.StartSong + 1,
	}
	if hdr.isNTSC() {
		hdr.PlaySpeedNtsc = defaultSpeedNtsc
	} else {
		hdr.PlaySpeedPal = defaultSpeedPal
	}
	if p.bank != nil {
		hdr.BankVals = p.bank.BankVals
	}
	if p.auth != nil {
		copy(hdr.SongName[:], p.auth.GameTitle) // what it really is, anyway... or album
		copy(hdr.ArtistName[:], p.auth.Artist)
		copy(hdr.CopyrightName[:], p.auth.Copyright)
	}
	return hdr
}

func readStructLE(structBytes []byte, iface interface{}) error {
	return binary.Read(bytes.NewReader(structBytes), binary.LittleEndian, iface)
}

func getNullStr(bytes []byte) string {
	for i := 0; i < len(bytes); i++ {
		if bytes[i] == 0 {
			return string(bytes[:i])
		}
	}
	return ""
}

func parseNsfe(nsfe []byte) (parsedNsfe, error) {
	parsed := parsedNsfe{}
	nsfe = nsfe[4:] // skip magic
	var sawInfo, sawData, sawNend bool
	for len(nsfe) > 0 {
		chunkHdr := chunkHdr{}
		if err := readStructLE(nsfe, &chunkHdr); err != nil {
			return parsedNsfe{}, err
		}
		if int(chunkHdr.ChunkLen) > len(nsfe) {
			return parsedNsfe{}, fmt.Errorf("bad nsfe chunk length %v", chunkHdr.ChunkLen)
		}
		nsfe = nsfe[8:] // past hdr
		chunkName := string(chunkHdr.Fourcc[:])
		switch chunkName {
		case "INFO":
			sawInfo = true
			if err := readStructLE(nsfe, &parsed.info); err != nil {
				return parsedNsfe{}, err
			}
		case "DATA":
			sawData = true
			parsed.data = nsfe[:chunkHdr.ChunkLen]
		case "BANK":
			bank := bankChunk{}
			for i := uint32(0); i < 8 && i < chunkHdr.ChunkLen; i++ {
				bank.BankVals[i] = nsfe[i]
			}
			parsed.bank = &bank
		case "NEND":
			sawNend = true
			break
		case "auth":
			auth := authChunk{}
			authBytes := nsfe[:chunkHdr.ChunkLen]
			auth.GameTitle = getNullStr(authBytes)
			authBytes = authBytes[len(auth.GameTitle)+1:]
			auth.Artist = getNullStr(authBytes)
			authBytes = authBytes[len(auth.Artist)+1:]
			auth.Copyright = getNullStr(authBytes)
			authBytes = authBytes[len(auth.Copyright)+1:]
			auth.Ripper = getNullStr(authBytes)
			parsed.auth = &auth
		default:
			if chunkName[0] >= 'A' && chunkName[0] <= 'Z' {
				return parsedNsfe{}, fmt.Errorf("unknown and required nsfe chunk %q", chunkName)
			}
		}
		nsfe = nsfe[chunkHdr.ChunkLen:]
	}
	if !sawInfo {
		return parsedNsfe{}, fmt.Errorf("bad nsfe, missing required chunk INFO")
	}
	if !sawData {
		return parsedNsfe{}, fmt.Errorf("bad nsfe, missing required chunk DATA")
	}
	if !sawNend {
		return parsedNsfe{}, fmt.Errorf("bad nsfe, missing required chunk NEND")
	}
	return parsed, nil
}

type plstChunk struct{ Playlist []byte }
type timeChunk struct{ SongLengths []int32 }
type fadeChunk struct{ FadeTimes []int32 }
type bankChunk struct{ BankVals [8]byte }
type tlblChunk struct{ SongNames []string }
type authChunk struct {
	GameTitle string
	Artist    string
	Copyright string
	Ripper    string
}
type textChunk struct {
	Text string
}
type infoChunk struct {
	LoadAddr       uint16
	InitAddr       uint16
	PlayAddr       uint16
	TvStdFlags     byte
	SoundChipFlags byte
	NumSongs       byte
	StartSong      byte
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

func parseNsf(nsf []byte) (nsfHeader, []byte, error) {
	hdr := nsfHeader{}
	if err := readStructLE(nsf, &hdr); err != nil {
		return nsfHeader{}, nil, fmt.Errorf("nsf player error\n%s", err.Error())
	}
	if hdr.SoundChipFlags != 0 {
		return nsfHeader{}, nil, fmt.Errorf("nsf player error\nunimplemented chip: %v", hdr.SoundChipFlags)
	}
	if hdr.Version != 1 {
		return nsfHeader{}, nil, fmt.Errorf("nsf player error\nunsupported nsf version: %v", hdr.Version)
	}
	data := nsf[0x80:]
	return hdr, data, nil
}

// NewNsfPlayer creates an nsfPlayer session
func NewNsfPlayer(nsf []byte) Emulator {

	var nsfe parsedNsfe
	var hdr nsfHeader
	var data []byte
	var err error
	switch string(nsf[:4]) {
	case "NESM":
		hdr, data, err = parseNsf(nsf)
	case "NSFE":
		nsfe, err = parseNsfe(nsf)
		if err == nil {
			hdr = nsfe.getNsfHeader()
			data = nsfe.data
		}
	default:
		err = fmt.Errorf("Unknown format: %q", string(nsf[:4]))
	}
	if err != nil {
		return NewErrEmu(fmt.Sprintf("nsf player error\n%s", err.Error()))
	}

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
		cart = make([]byte, 32*1024)
		copy(cart[hdr.LoadAddr-0x8000:], data)
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
	np.DbgTerminal = dbgTerminal{w: 256, h: 240, screen: np.DbgScreen[:]}

	np.init()

	np.initTune(np.CurrentSong)

	np.updateScreen()

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

func (np *nsfPlayer) updateScreen() {

	np.DbgTerminal.setPos(0, 1)
	np.DbgTerminal.writeString("NSF Player\n")
	np.DbgTerminal.newline()
	np.DbgTerminal.writeString("Title: " + string(np.Hdr.SongName[:]) + "\n")
	np.DbgTerminal.writeString("Artist: " + string(np.Hdr.ArtistName[:]) + "\n")
	np.DbgTerminal.writeString(string(np.Hdr.CopyrightName[:]) + "\n")

	np.DbgTerminal.clearLine()
	np.DbgTerminal.writeString(fmt.Sprintf("Track %02d/%02d\n", np.CurrentSong+1, np.Hdr.NumSongs))

	np.DbgTerminal.newline()

	np.DbgTerminal.clearLine()
	if np.Paused {
		np.DbgTerminal.writeString("*PAUSED*\n")
	}
	np.DbgFlipRequested = true
}

var lastInput time.Time

func (np *nsfPlayer) UpdateInput(input Input) {
	now := time.Now()
	if now.Sub(lastInput).Seconds() > 0.20 {
		if input.Joypad.Left {
			if np.CurrentSong > 0 {
				np.CurrentSong--
				np.initTune(np.CurrentSong)
				np.updateScreen()
			}
			lastInput = now
		}
		if input.Joypad.Right {
			if np.CurrentSong < np.Hdr.NumSongs-1 {
				np.CurrentSong++
				np.initTune(np.CurrentSong)
				np.updateScreen()
			}
			lastInput = now
		}
		if input.Joypad.Start {
			np.Paused = !np.Paused
			lastInput = now
			np.updateScreen()
		}
	}
}

func (np *nsfPlayer) Step() {
	if !np.Paused {
		now := time.Now()
		if np.PC == 0x0001 {
			timeLeft := np.PlayCallInterval - now.Sub(np.LastPlayCall).Seconds()
			if timeLeft <= 0 {
				np.LastPlayCall = now
				np.S = 0xfd
				np.push16(0x0000)
				np.PC = np.Hdr.PlayAddr
			} else {
				toWait := time.Duration(timeLeft / 2 * 1000 * float64(time.Millisecond))
				if toWait > time.Millisecond {
					<-time.NewTimer(toWait).C
				}
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
