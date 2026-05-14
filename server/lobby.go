package main

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
)

type LobbyCommand int

const (
	LobbyCreateRoom LobbyCommand = iota
	LobbyJoinRoom
	LobbyRefreshRooms
)

type Room struct {
	ID      int32
	Name    string
	Players []*Client
}

type Lobby struct {
	Rooms      map[int32]*Room
	nextRoomID int32
}

// Lobby 함수는 클라이언트로부터 받은 데이터를 처리하여 로비 관련 명령을 수행합니다.
func (s *Server) LobbyManager(msg Message) {
	switch LobbyCommand(msg.Data[0]) {
	case LobbyCreateRoom:
		RoomID := s.generateRoomID()
		s.Lobby.Rooms[RoomID] = &Room{
			ID:      RoomID,
			Name:    string(msg.Data[1:]),
			Players: []*Client{},
		}

		fmt.Printf("Room %d created\n", RoomID)
	case LobbyJoinRoom:
		roomID := int32(binary.BigEndian.Uint32(msg.Data[1:5]))
		s.JoinRoom(msg.Client, roomID)
		fmt.Printf("Room %d joined\n", roomID)
	case LobbyRefreshRooms:
		s.refreshRooms(msg.Client)
		fmt.Printf("Client %d requested room list refresh\n", msg.Client.ID)
	}
}

// generateRoomID는 새로운 방 ID를 생성합니다.
// 원자적 연산을 사용하여 안전하게 증가시킵니다.
func (s *Server) generateRoomID() int32 {
	return atomic.AddInt32(&s.Lobby.nextRoomID, 1)
}

func (s *Server) JoinRoom(client *Client, roomID int32) {
	room, exists := s.Lobby.Rooms[roomID]
	if !exists {
		fmt.Printf("Room %d does not exist\n", roomID)
		return
	}

	room.Players = append(room.Players, client)
	client.State = ClientStateGame

	buf := []byte{byte(ClientStateLobby)}
	buf = append(buf, byte(LobbyJoinRoom))
	buf = append(buf, byte(roomID))
	client.WritePacket(buf)
}

// [Scene] [SceneCommand] [roomLen] [[roomID][nameLen][roomName][roomPlayerCount]...]
func (s *Server) refreshRooms(client *Client) bool {
	buf := []byte{byte(ClientStateLobby)} // Scene: Lobby

	buf = append(buf, byte(LobbyRefreshRooms))  // SceneCommand: RefreshRooms
	buf = append(buf, byte(len(s.Lobby.Rooms))) // 방 갯수

	for _, room := range s.Lobby.Rooms {
		roomData := make([]byte, 5)
		binary.BigEndian.PutUint32(roomData[0:4], uint32(room.ID))
		roomData[4] = byte(len(room.Name)) // 방 이름 길이
		buf = append(buf, roomData...)
		buf = append(buf, []byte(room.Name)...)
		buf = append(buf, byte(len(room.Players)))
	}

	client.WritePacket(buf)
	return true
}
