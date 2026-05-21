package game

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
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

func LoadReplay(filename string) (*StoredReplay, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return DecodeReplay(data)
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
