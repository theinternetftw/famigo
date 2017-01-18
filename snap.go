package famigo

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

const currentSnapshotVersion = 1

const infoString = "famigo snapshot"

type snapshot struct {
	Version int
	Info    string
	State   json.RawMessage
	MMC     marshalledMMC
	ChrRAM  []byte
}

func (cs *cpuState) loadSnapshot(snapBytes []byte) (*cpuState, error) {
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
		return cs.convertOldSnapshot(&snap)
	} else if snap.Version > currentSnapshotVersion {
		return nil, fmt.Errorf("this version of famigo is too old to open this snapshot")
	}

	// NOTE: what about external RAM? Doesn't this overwrite .sav files with whatever's in the snapshot?

	return cs.convertLatestSnapshot(&snap)
}

func (cs *cpuState) convertLatestSnapshot(snap *snapshot) (*cpuState, error) {
	var err error
	var newState cpuState
	if err = json.Unmarshal(snap.State, &newState); err != nil {
		return nil, err
	} else if newState.Mem.mmc, err = unmarshalMMC(snap.MMC); err != nil {
		return nil, err
	}
	newState.Mem.prgROM = cs.Mem.prgROM
	if cs.CartInfo.IsChrRAM() {
		newState.Mem.chrROM = snap.ChrRAM
	} else {
		newState.Mem.chrROM = cs.Mem.chrROM
	}
	return &newState, nil
}

var snapshotConverters = map[int]func([]byte) []byte{
// If new field can be zero, no need for converter.
// Converters should look like this (including comment):
// added 2017-XX-XX
// 1: func(stateBytes []byte) []byte {
// 	stateBytes = stateBytes[:len(stateBytes)-1]
// 	return append(stateBytes, []byte(",\"ExampleNewField\":0}")...)
// },
}

func (cs *cpuState) convertOldSnapshot(snap *snapshot) (*cpuState, error) {

	var err error
	var newState cpuState

	// unfortunately, can't use json, as go is crazy enough to make it so
	// converting something in and out of json as a map[string]interface{}
	// will kill the ability to import it back in as a struct. so we have
	// to change it by hand to keep the go conventions that go will break
	// otherwise :/
	stateBytes := []byte(snap.State)

	for i := snap.Version; i < currentSnapshotVersion; i++ {
		converterFn, ok := snapshotConverters[snap.Version]
		if !ok {
			return nil, fmt.Errorf("unknown snapshot version: %v", snap.Version)
		}
		stateBytes = converterFn(stateBytes)
	}

	if err = json.Unmarshal(stateBytes, &newState); err != nil {
		return nil, fmt.Errorf("post-convert unpack err: %v", err)
	} else if newState.Mem.mmc, err = unmarshalMMC(snap.MMC); err != nil {
		return nil, fmt.Errorf("unpack mmc err: %v", err)
	}
	newState.Mem.prgROM = cs.Mem.prgROM
	if cs.CartInfo.IsChrRAM() {
		newState.Mem.chrROM = snap.ChrRAM
	} else {
		newState.Mem.chrROM = cs.Mem.chrROM
	}
	return &newState, nil
}

func (cs *cpuState) makeSnapshot() []byte {
	var err error
	var csJSON []byte
	var snapJSON []byte
	if csJSON, err = json.Marshal(cs); err != nil {
		panic(err)
	}
	snap := snapshot{
		Version: currentSnapshotVersion,
		Info:    infoString,
		State:   json.RawMessage(csJSON),
		MMC:     cs.Mem.mmc.Marshal(),
	}
	if cs.CartInfo.IsChrRAM() {
		snap.ChrRAM = cs.Mem.chrROM
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
