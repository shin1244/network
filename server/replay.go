package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

const replayMagic = "PONGREP1"

type StoredReplay struct {
	RoomID uint32
	Frames []StoredReplayFrame
}

type StoredReplayFrame struct {
	Tick    uint32
	P1      Axis
	P2      Axis
	P1Ready bool
	P2Ready bool
}

func (l *Lobby) ReplayListPayload() []byte {
	replayList := replayNames()
	payload := []byte{byte(len(replayList))}
	for idx, replay := range replayList {
		nameBytes := []byte(replay)

		idBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(idBytes, uint32(idx))

		payload = append(payload, idBytes...)
		payload = append(payload, byte(len(nameBytes)))
		payload = append(payload, nameBytes...)
	}

	return payload
}

func (l *Lobby) ReplayPayload(payload []byte) []byte {
	if len(payload) < 4 {
		log.Printf("invalid replay request payload: %v", payload)
		return nil
	}

	replayID := int(binary.BigEndian.Uint32(payload[0:4]))
	replayList := replayNames()
	if replayID < 0 || replayID >= len(replayList) {
		log.Printf("invalid replay id: %d", replayID)
		return nil
	}

	replayPath := filepath.Join("replay", replayList[replayID])
	data, err := os.ReadFile(replayPath)
	if err != nil {
		log.Printf("failed to read replay %s: %v", replayPath, err)
		return nil
	}

	return data
}

func replayNames() []string {
	replayList := []string{}
	files, err := os.ReadDir("./replay")
	if err != nil {
		log.Printf("failed to read replay directory: %v", err)
		return replayList
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) == ".rep" {
			replayList = append(replayList, file.Name())
		}
	}

	sort.Strings(replayList)
	return replayList
}

func DecodeReplay(data []byte) (*StoredReplay, error) {
	const headerSize = 16
	const frameSize = 8

	if len(data) < headerSize {
		return nil, fmt.Errorf("replay file too short: %d bytes", len(data))
	}
	if string(data[0:8]) != replayMagic {
		return nil, fmt.Errorf("invalid replay magic: %q", string(data[0:8]))
	}

	roomID := binary.BigEndian.Uint32(data[8:12])
	frameCount := int(binary.BigEndian.Uint32(data[12:16]))
	expectedLen := headerSize + frameCount*frameSize
	if len(data) != expectedLen {
		return nil, fmt.Errorf("invalid replay length: got %d, want %d", len(data), expectedLen)
	}

	frames := make([]StoredReplayFrame, 0, frameCount)
	reader := bytes.NewReader(data[headerSize:])
	for i := 0; i < frameCount; i++ {
		var tick uint32
		if err := binary.Read(reader, binary.BigEndian, &tick); err != nil {
			return nil, fmt.Errorf("read replay tick %d: %w", i, err)
		}

		p1, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read replay p1 input %d: %w", i, err)
		}

		p2, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read replay p2 input %d: %w", i, err)
		}

		p1Ready, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read replay p1 ready %d: %w", i, err)
		}

		p2Ready, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read replay p2 ready %d: %w", i, err)
		}

		frames = append(frames, StoredReplayFrame{
			Tick:    tick,
			P1:      Axis(int8(p1)),
			P2:      Axis(int8(p2)),
			P1Ready: p1Ready != 0,
			P2Ready: p2Ready != 0,
		})
	}

	return &StoredReplay{
		RoomID: roomID,
		Frames: frames,
	}, nil
}

func replayFramesPayload(recorder map[uint32]*ReplayFrame, ticks []uint32) []byte {
	if ticks == nil {
		ticks = make([]uint32, 0, len(recorder))
		for tick := range recorder {
			ticks = append(ticks, tick)
		}
	}

	sort.Slice(ticks, func(i, j int) bool {
		return ticks[i] < ticks[j]
	})

	frames := make([]byte, 0, 2+len(ticks)*8)
	frames = append(frames, 0, 0)
	frameCount := 0
	var lastTick uint32
	hasLastTick := false

	for _, tick := range ticks {
		if hasLastTick && tick == lastTick {
			continue
		}
		lastTick = tick
		hasLastTick = true

		frame := recorder[tick]
		if frame == nil {
			continue
		}
		if !frame.P1Ready || !frame.P2Ready {
			continue
		}

		frames = binary.BigEndian.AppendUint32(frames, tick)
		frames = append(frames, byte(int8(frame.P1)))
		frames = append(frames, byte(int8(frame.P2)))
		frames = append(frames, boolByte(frame.P1Ready))
		frames = append(frames, boolByte(frame.P2Ready))
		frameCount++
	}

	binary.BigEndian.PutUint16(frames[0:2], uint16(frameCount))
	return frames
}
