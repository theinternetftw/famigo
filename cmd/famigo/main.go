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

	cartInfo, err := famigo.ParseCartInfo(romBytes)
	dieIf(err)
	fmt.Println("PRG ROM SIZE:", cartInfo.GetROMSizePrg())
	fmt.Println("CHR ROM SIZE:", cartInfo.GetROMSizeChr())
	fmt.Println("MAPPER NUM:", cartInfo.GetMapperNumber())

	platform.InitDisplayLoop(256*2, 240*2, 256, 240, func(sharedState *platform.WindowState) {
		startEmu(cartFilename, sharedState, romBytes)
	})
}

func startEmu(filename string, window *platform.WindowState, romBytes []byte) {
	emu := famigo.NewEmulator(romBytes)

	// FIXME: settings are for debug right now
	lastVBlankTime := time.Now()

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
