package famigo

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

const currentSnapshotVersion = 2

const infoString = "famigo snapshot"

type snapshot struct {
	Version int
	Info    string
	State   json.RawMessage
	MMC     marshalledMMC
	ChrRAM  []byte
}

func (emu *emuState) loadSnapshot(snapBytes []byte) (*emuState, error) {
	var err error
	var reader io.Reader
	var unpackedBytes []byte
	var snap snapshot
	if reader, err = gzip.NewReader(bytes.NewReader(snapBytes)); err != nil {
		return nil, err
	} else if unpackedBytes, err = ioutil.ReadAll(reader); err != nil {
		return nil, err
	} else if err = json.Unmarshal(unpackedBytes, &snap); err != nil {
		return nil, err
	} else if snap.Version < currentSnapshotVersion {
		return emu.convertOldSnapshot(&snap)
	} else if snap.Version > currentSnapshotVersion {
		return nil, fmt.Errorf("this version of famigo is too old to open this snapshot")
	}

	// NOTE: what about external RAM? Doesn't this overwrite .sav files with whatever's in the snapshot?

	return emu.convertLatestSnapshot(&snap)
}

func (emu *emuState) convertLatestSnapshot(snap *snapshot) (*emuState, error) {

	var err error
	var newState emuState

	if err = json.Unmarshal(snap.State, &newState); err != nil {
		return nil, err
	}

	if newState.Mem.mmc, err = unmarshalMMC(snap.MMC); err != nil {
		return nil, err
	}

	newState.Mem.prgROM = emu.Mem.prgROM
	if emu.CartInfo.IsChrRAM() {
		newState.Mem.chrROM = snap.ChrRAM
	} else {
		newState.Mem.chrROM = emu.Mem.chrROM
	}

	newState.CPU.Write = newState.write
	newState.CPU.Read = newState.read
	newState.CPU.RunCycles = newState.runCycles
	newState.CPU.Err = func(e error) { emuErr(e) }

	return &newState, nil
}

var snapshotConverters = map[int]func(map[string]interface{}) error{

	// If new field can be zero, no need for converter.

	// added 2018-06-13
	1: convertSnap1To2,
}

// added 2018-06-13
func convertSnap1To2(state map[string]interface{}) error {
	newCPU := map[string]interface{}{
		"IgnoreDecimalMode": true,
	}
	keys := []string{
		"PC",
		"P", "A", "X", "Y", "S",
		"IRQ", "BRK", "NMI", "RESET",
		"LastStepsP",
		"Steps",
	}
	for _, key := range keys {
		val, ok := state[key]
		if !ok {
			return fmt.Errorf("missing key %v", key)
		}
		newCPU[key] = val
		delete(state, key)
	}
	state["CPU"] = newCPU

	return nil
}

func (emu *emuState) convertOldSnapshot(snap *snapshot) (*emuState, error) {

	var state map[string]interface{}
	if err := json.Unmarshal(snap.State, &state); err != nil {
		return nil, fmt.Errorf("json unpack err: %v", err)
	}

	for i := snap.Version; i < currentSnapshotVersion; i++ {
		if converterFn, ok := snapshotConverters[snap.Version]; !ok {
			return nil, fmt.Errorf("unknown snapshot version: %v", snap.Version)
		} else if err := converterFn(state); err != nil {
			return nil, fmt.Errorf("error converting snapshot version %v: %v", i, err)
		}
	}

	var err error
	if snap.State, err = json.Marshal(state); err != nil {
		return nil, fmt.Errorf("json pack err: %v", err)
	}

	return emu.convertLatestSnapshot(snap)
}

func (emu *emuState) makeSnapshot() []byte {
	var err error
	var emuJSON []byte
	var snapJSON []byte
	if emuJSON, err = json.Marshal(emu); err != nil {
		panic(err)
	}
	snap := snapshot{
		Version: currentSnapshotVersion,
		Info:    infoString,
		State:   json.RawMessage(emuJSON),
		MMC:     emu.Mem.mmc.Marshal(),
	}
	if emu.CartInfo.IsChrRAM() {
		snap.ChrRAM = emu.Mem.chrROM
	}
	if snapJSON, err = json.Marshal(&snap); err != nil {
		panic(err)
	}
	buf := &bytes.Buffer{}
	writer := gzip.NewWriter(buf)
	writer.Write(snapJSON)
	writer.Close()
	return buf.Bytes()
}
