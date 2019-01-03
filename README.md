# famigo - a nes emulator / nsf player in go

My other emulators:
[dmgo](https://github.com/theinternetftw/dmgo),
[vcsgo](https://github.com/theinternetftw/vcsgo),
[segmago](https://github.com/theinternetftw/segmago), and
[a1go](https://github.com/theinternetftw/a1go).

#### Features:
 * Audio (on windows)!
 * Saved game support!
 * Quicksave/Quickload, too!
 * Plays NSF and NSFE files! ([here's a good album to try](http://rainwarrior.ca/projects/nes/pico.html))
 * Missing a few [mappers](http://wiki.nesdev.com/w/index.php/Mapper), the NES has literally hundreds!
 * Glitches are rare, but less rare than dmgo, and still totally happen!
 * Graphical cross-platform support!

That last bit relies on [glimmer](https://github.com/theinternetftw/glimmer). Tested on windows 10 and ubuntu 18.10.

#### Dependencies:

 * You can compile on windows with no C dependencies.
 * Linux users should 'apt install libasound2-dev' or equivalent.
 * FreeBSD (and Mac?) users should 'pkg install openal-soft' or equivalent.

#### Compile instructions

 * If you have go version >= 1.11, `go build ./cmd/famigo` should be enough.
 * The interested can also see my build script `b` for profiling and such.
 * Non-windows users will need the dependencies listed above.

#### Important Notes:

 * Keybindings are currently hardcoded to WSAD / JK / TY (arrowpad, ba, start/select)
 * The NSF player uses the same keys for pause (start), and track skip (left/right)
 * Saved games use/expect a slightly different naming convention than usual: romfilename.nes.sav
 * Quicksave/Quickload is done by pressing m or l (make or load quicksave), followed by a number key
