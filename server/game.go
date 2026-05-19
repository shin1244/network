package main

import (
	"encoding/binary"
	"net"
)

const (
	StartHolePunching byte = iota
)

func (s *Server) StartHolePunching(room *Room) {
	p1 := room.Players[0]
	p2 := room.Players[1]

	p1.Send <- MakePacket(1, 0, SerializeUDPAddr(p2.UDPAddr))
	p2.Send <- MakePacket(1, 0, SerializeUDPAddr(p1.UDPAddr))
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
