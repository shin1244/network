package main

import (
	"encoding/binary"
	"net"
)

const (
	SceneGame         byte = 1
	CmdStartHolePunch byte = 0
)

type GameOverReport struct {
	Winner       int32
	LeftScore    byte
	RightScore   byte
	GameOverTick uint32
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
