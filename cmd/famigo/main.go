package main

import (
	"github.com/theinternetftw/famigo"
	"github.com/theinternetftw/famigo/profiling"
	"github.com/theinternetftw/glimmer"

	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"
)

type options struct {
	fastMode bool
}

func main() {

	defer profiling.Start().Stop()

	fastMode := flag.Bool("fast", false, "starts in fast mode (no frame wait)")
	flag.Parse()

	args := flag.Args()
	assert(len(args) == 1, "usage: ./famigo ROM_FILENAME")
	cartFilename := args[0]

	romBytes, err := ioutil.ReadFile(cartFilename)
	dieIf(err)

	assert(len(romBytes) > 4, "cannot parse file, illegal header")

	var emu famigo.Emulator

	fileMagic := string(romBytes[:4])
	if fileMagic == "NESM" || fileMagic == "NSFE" {
		// nsf(e) file
		emu = famigo.NewNsfPlayer(romBytes)
	} else {
		// rom file
		cartInfo, err := famigo.ParseCartInfo(romBytes)
		dieIf(err)

		fmt.Println("PRG ROM SIZE:", cartInfo.GetROMSizePrg())
		fmt.Println("PRG RAM SIZE:", cartInfo.GetRAMSizePrg(), "( Battery backed:", cartInfo.HasBatteryBackedRAM(), ")")
		fmt.Println("CHR ROM SIZE:", cartInfo.GetROMSizeChr())
		fmt.Println("MAPPER NUM:", cartInfo.GetMapperNumber())

		emu = famigo.NewEmulator(romBytes)
	}

	glimmer.InitDisplayLoop("famigo", 256*2+40, 240*2+40, 256, 240, func(sharedState *glimmer.WindowState) {
		startEmu(cartFilename, sharedState, emu, options{
			fastMode: *fastMode,
		})
	})
}

func startEmu(filename string, window *glimmer.WindowState, emu famigo.Emulator, options options) {

	// FIXME: settings are for debug right now
	lastFlipTime := time.Now()
	lastDrawTime := time.Now()
	lastSaveTime := time.Now()

	snapshotPrefix := filename + ".snapshot"

	saveFilename := filename + ".sav"
	saveFile, err := ioutil.ReadFile(saveFilename)
	if err == nil {
		err = emu.SetPrgRAM(saveFile)
	}

	if err == nil {
		fmt.Println("loaded save!")
	} else if !os.IsNotExist(err) {
		fmt.Println("error loading savefile,", err)
	}

	audio, err := glimmer.OpenAudioBuffer(4, 4096, 44100, 16, 2)
	workingAudioBuffer := make([]byte, audio.BufferSize())
	dieIf(err)

	snapshotMode := 'x'

	for {
		window.Mutex.Lock()
		newInput := famigo.Input {
			Joypad: famigo.Joypad {
				Sel:  window.CharIsDown('t'), Start: window.CharIsDown('y'),
				Up:   window.CharIsDown('w'), Down:  window.CharIsDown('s'),
				Left: window.CharIsDown('a'), Right: window.CharIsDown('d'),
				A:    window.CharIsDown('k'), B:     window.CharIsDown('j'),
			},
		}
		numDown := 'x'
		for r := '0'; r <= '9'; r++ {
			if window.CharIsDown(r) {
				numDown = r
				break
			}
		}
		if window.CharIsDown('m') {
			snapshotMode = 'm'
		} else if window.CharIsDown('l') {
			snapshotMode = 'l'
		}
		window.Mutex.Unlock()

		if numDown > '0' && numDown <= '9' {
			snapFilename := snapshotPrefix+string(numDown)
			if snapshotMode == 'm' {
				snapshotMode = 'x'
				snapshot := emu.MakeSnapshot()
				if len(snapshot) > 0 {
					ioutil.WriteFile(snapFilename, snapshot, os.FileMode(0644))
				}
			} else if snapshotMode == 'l' {
				snapshotMode = 'x'
				snapBytes, err := ioutil.ReadFile(snapFilename)
				if err != nil {
					fmt.Println("failed to load snapshot:", err)
					continue
				}
				newEmu, err := emu.LoadSnapshot(snapBytes)
				if err != nil {
					fmt.Println("failed to load snapshot:", err)
					continue
				}
				emu = newEmu
			}
		}

		emu.UpdateInput(newInput)
		emu.Step()

		bufferAvailable := audio.BufferAvailable()
		// if bufferAvailable == audio.BufferSize() {
		// 	fmt.Println("Platform AudioBuffer empty!")
		// }
		workingAudioBuffer = workingAudioBuffer[:bufferAvailable]
		audio.Write(emu.ReadSoundBuffer(workingAudioBuffer))

		if emu.FlipRequested() {
			if !options.fastMode || time.Now().Sub(lastDrawTime) > 17*time.Millisecond {

				window.Mutex.Lock()
				copy(window.Pix, emu.Framebuffer())
				window.RequestDraw()
				window.Mutex.Unlock()

				lastDrawTime = time.Now()
			}

			spent := time.Now().Sub(lastFlipTime)
			if !options.fastMode {
				toWait := 17*time.Millisecond - spent
				if toWait > time.Duration(0) {
					<-time.NewTimer(toWait).C
				}
			}
			lastFlipTime = time.Now()
		}
		if time.Now().Sub(lastSaveTime) > 5*time.Second {
			ram := emu.GetPrgRAM()
			if len(ram) > 0 {
				ioutil.WriteFile(saveFilename, ram, os.FileMode(0644))
				lastSaveTime = time.Now()
			}
		}
	}
}

func assert(test bool, msg string) {
	if !test {
		fmt.Println(msg)
		os.Exit(1)
	}
}

func dieIf(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
