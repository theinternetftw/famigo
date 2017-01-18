package main

import (
	"github.com/theinternetftw/famigo"
	"github.com/theinternetftw/famigo/profiling"
	"github.com/theinternetftw/famigo/platform"

	"fmt"
	"io/ioutil"
	"os"
	"time"
)

func main() {

	defer profiling.Start().Stop()

	assert(len(os.Args) == 2, "usage: ./famigo ROM_FILENAME")
	cartFilename := os.Args[1]

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
		fmt.Println("PRG RAM SIZE:", cartInfo.GetRAMSizePrg())
		fmt.Println("CHR ROM SIZE:", cartInfo.GetROMSizeChr())
		fmt.Println("MAPPER NUM:", cartInfo.GetMapperNumber())

		emu = famigo.NewEmulator(romBytes)
	}

	platform.InitDisplayLoop(256*2+40, 240*2+40, 256, 240, func(sharedState *platform.WindowState) {
		startEmu(cartFilename, sharedState, emu)
	})
}

func startEmu(filename string, window *platform.WindowState, emu famigo.Emulator) {

	// FIXME: settings are for debug right now
	lastVBlankTime := time.Now()
	lastSaveTime := time.Now()

	saveFilename := filename + ".sav"
	if saveFile, err := ioutil.ReadFile(saveFilename); err == nil {
		err = emu.SetPrgRAM(saveFile)
		if err != nil {
			fmt.Println("error loading savefile,", err)
		}
		fmt.Println("loaded save!")
	}

	audio, err := platform.OpenAudioBuffer(4, 4096, 44100, 16, 2)
	workingAudioBuffer := make([]byte, audio.BufferSize())
	dieIf(err)

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
		window.Mutex.Unlock()

		emu.UpdateInput(newInput)
		emu.Step()

		bufferAvailable := audio.BufferAvailable()
		// if bufferAvailable == audio.BufferSize() {
		// 	fmt.Println("Platform AudioBuffer empty!")
		// }
		workingAudioBuffer = workingAudioBuffer[:bufferAvailable]
		audio.Write(emu.ReadSoundBuffer(workingAudioBuffer))

		if emu.FlipRequested() {
			window.Mutex.Lock()
			copy(window.Pix, emu.Framebuffer())
			window.RequestDraw()
			window.Mutex.Unlock()

			spent := time.Now().Sub(lastVBlankTime)
			toWait := 17*time.Millisecond - spent
			if toWait > time.Duration(0) {
				<-time.NewTimer(toWait).C
			}
			lastVBlankTime = time.Now()
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
