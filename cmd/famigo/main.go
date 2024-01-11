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

	// TODO: config file instead
	devMode := fileExists("devmode")

	var emu famigo.Emulator

	fileMagic := string(romBytes[:4])
	if fileMagic == "NESM" || fileMagic == "NSFE" {
		// nsf(e) file
		emu = famigo.NewNsfPlayer(romBytes, devMode)
	} else {
		// rom file
		cartInfo, err := famigo.ParseCartInfo(romBytes)
		dieIf(err)

		if devMode {
			fmt.Println("PRG ROM SIZE:", cartInfo.GetROMSizePrg())
			fmt.Println("PRG RAM SIZE:", cartInfo.GetRAMSizePrg(), "( Battery backed:", cartInfo.HasBatteryBackedRAM(), ")")
			fmt.Println("CHR ROM SIZE:", cartInfo.GetROMSizeChr())
			fmt.Println("MAPPER NUM:", cartInfo.GetMapperNumber())
		}

		emu = famigo.NewEmulator(romBytes, devMode)
	}

	glimmer.InitDisplayLoop(glimmer.InitDisplayLoopOptions{
		WindowTitle: "famigo",
		WindowWidth: 256*2 + 40, WindowHeight: 240*2 + 40,
		RenderWidth: 256, RenderHeight: 240,
		InitCallback: func(sharedState *glimmer.WindowState) {
			startEmu(cartFilename, sharedState, emu, options{
				fastMode: *fastMode,
			})
		},
	})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func startEmu(filename string, window *glimmer.WindowState, emu famigo.Emulator, options options) {

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

	audio, audioErr := glimmer.OpenAudioBuffer(glimmer.OpenAudioBufferOptions{
		OutputBufDuration: 25 * time.Millisecond,
		SamplesPerSecond:  44100,
		BitsPerSample:     16,
		ChannelCount:      2,
	})
	dieIf(audioErr)
	workingAudioBuffer := make([]byte, audio.GetPrevCallbackReadLen())
	audioToGen := audio.GetPrevCallbackReadLen()

	snapshotMode := 'x'

	lastDrawTime := time.Now()
	lastSaveTime := time.Now()

	frameTimer := glimmer.MakeFrameTimer()

	for {
		window.InputMutex.Lock()
		newInput := famigo.Input{
			Joypad: famigo.Joypad{
				Sel: window.CharIsDown('t'), Start: window.CharIsDown('y'),
				Up: window.CharIsDown('w'), Down: window.CharIsDown('s'),
				Left: window.CharIsDown('a'), Right: window.CharIsDown('d'),
				A: window.CharIsDown('k'), B: window.CharIsDown('j'),
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
		window.InputMutex.Unlock()

		if numDown > '0' && numDown <= '9' {
			snapFilename := snapshotPrefix + string(numDown)
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

		if emu.GetSoundBufferUsed() >= audioToGen {
			if cap(workingAudioBuffer) < audioToGen {
				workingAudioBuffer = make([]byte, audioToGen)
			}
			workingAudioBuffer = workingAudioBuffer[:audioToGen]
			audio.Write(emu.ReadSoundBuffer(workingAudioBuffer))
		}

		if emu.FlipRequested() {
			frameTimer.MarkRenderComplete()
			if !options.fastMode || time.Now().Sub(lastDrawTime) > 17*time.Millisecond {

				window.RenderMutex.Lock()
				copy(window.Pix, emu.Framebuffer())
				window.RenderMutex.Unlock()

				lastDrawTime = time.Now()
			}

			if !options.fastMode {
				audio.WaitForPlaybackIfAhead()
			}

			frameTimer.MarkFrameComplete()

			if emu.InDevMode() {
				frameTimer.PrintStatsEveryXFrames(60 * 5)
			}
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
