package main

import (
	"encoding/binary"
	"fmt"
	"net"
)

type Axis int

const (
	SceneGame         byte = 1
	CmdStartHolePunch byte = 0

	AxisNeutral Axis = 0
	AxisUp      Axis = -1
	AxisDown    Axis = 1

	GameOverCommand    byte = 2
	ReplayBatchCommand byte = 3
)

type GameOverReport struct {
	Winner       int32
	LeftScore    byte
	RightScore   byte
	GameOverTick uint32
}

type ReplayFrame struct {
	Tick    uint32
	P1      Axis
	P2      Axis
	P1Ready bool
	P2Ready bool
}

type ReplayInput struct {
	Tick  uint32
	Input Axis
}

func (s *Server) StartHolePunching(room *Room) {
	if len(room.Players) < 2 {
		return
	}

	p1 := room.Players[0]
	p2 := room.Players[1]

	// P1에게 P2의 공인 UDP 주소를 전송
	p1.Send <- MakePacket(SceneGame, CmdStartHolePunch, SerializeUDPAddr(p2.UDPAddr))
	// P2에게 P1의 공인 UDP 주소를 전송
	p2.Send <- MakePacket(SceneGame, CmdStartHolePunch, SerializeUDPAddr(p1.UDPAddr))

	p1.State = ClientStateGame
	p2.State = ClientStateGame
}

func SerializeUDPAddr(addr *net.UDPAddr) []byte {
	data := make([]byte, 6)

	copy(data[0:4], addr.IP.To4())

	binary.BigEndian.PutUint16(
		data[4:6],
		uint16(addr.Port),
	)

	return data
}

func parseReplayBatch(payload []byte) ([]ReplayInput, error) {
	if len(payload) < 2 {
		return nil, fmt.Errorf("payload too short")
	}

	count := int(binary.BigEndian.Uint16(payload[0:2]))
	expectedLen := 2 + count*5
	if len(payload) != expectedLen {
		return nil, fmt.Errorf("invalid payload length: got %d, want %d", len(payload), expectedLen)
	}

	inputs := make([]ReplayInput, 0, count)
	offset := 2

	for i := 0; i < count; i++ {
		tick := binary.BigEndian.Uint32(payload[offset : offset+4])
		offset += 4

		input := Axis(int8(payload[offset]))
		offset++

		inputs = append(inputs, ReplayInput{
			Tick:  tick,
			Input: input,
		})
	}

	return inputs, nil
}

func (l *Lobby) HandleReplayBatch(client *Client, inputs []ReplayInput) {
	room, exists := l.ClientRoom[client.ID]
	if !exists {
		fmt.Printf("Client %d is not in a room, ignoring replay batch\n", client.ID)
		return
	}

	player := room.PlayerNumber(client)
	if player == 0 {
		fmt.Printf("Client %d is not a player in Room %d\n", client.ID, room.ID)
		return
	}

	for _, input := range inputs {
		frame := room.Recorder[input.Tick]
		if frame == nil {
			frame = &ReplayFrame{Tick: input.Tick}
			room.Recorder[input.Tick] = frame
		}

		switch player {
		case 1:
			frame.P1 = input.Input
			frame.P1Ready = true
		case 2:
			frame.P2 = input.Input
			frame.P2Ready = true
		}
	}

	fmt.Printf("Room %d replay inputs stored from Client %d: %d\n", room.ID, client.ID, len(inputs))
}
