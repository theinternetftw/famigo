# famigo - a nes emulator / nsf player in go

After [dmgo](https://github.com/theinternetftw/dmgo), I figured I'd make a NES emulator. So I did.

#### Features:
 * Audio (on windows)!
 * Saved game support!
 * Quicksave/Quickload, too!
 * Plays NSF and NSFE files! ([here's a good album to try](http://rainwarrior.ca/projects/nes/pico.html))
 * Missing a few [mappers](http://wiki.nesdev.com/w/index.php/Mapper), the NES has literally hundreds!
 * Glitches are rare, but less rare than dmgo, and still totally happen!
 * Graphical cross-platform support in native golang, with no hooks into C libraries needed!

That last bit relies on [exp/shiny](https://github.com/golang/exp/tree/master/shiny), which is still a work in progress. Let me know if it fails on your platform.
Tested on windows 10 and xubuntu.

#### Build instructions:

famigo uses [glide](https://github.com/Masterminds/glide) for dependencies, so run `glide update` first (or just `go get` the packages mentioned in the `glide.yaml` file).

After that, `go build ./cmd/famigo` should be enough. The interested can also see my build script `b` for more options (profiling and cross-compiling and such).

#### Important Notes:

 * Keybindings are currently hardcoded to WSAD / JK / TY (arrowpad, ba, start/select)
 * The NSF player uses the same keys for pause (start), and track skip (left/right)
 * Saved games use/expect a slightly different naming convention than usual: romfilename.nes.sav
 * Quicksave/Quickload is done by pressing m or l (make or load quicksave), followed by a number key
