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
func (l *Lobby) LobbyManager(msg Message) {
	switch LobbyCommand(msg.Command) {
	case LobbyCreateRoom:
		roomName := string(msg.Payload)
		l.createRoom(roomName)
		fmt.Printf("Room '%s' created\n", roomName)
	case LobbyJoinRoom:
		roomID := int32(binary.BigEndian.Uint32(msg.Payload))
		l.JoinRoom(msg.Client, roomID)
		fmt.Printf("Room %d joined\n", roomID)
	case LobbyRefreshRooms:
		l.refreshRooms(msg.Client)
		fmt.Printf("Client %d requested room list refresh\n", msg.Client.ID)
	}
}

// generateRoomID는 새로운 방 ID를 생성합니다.
// 원자적 연산을 사용하여 안전하게 증가시킵니다.
func (l *Lobby) generateRoomID() int32 {
	return atomic.AddInt32(&l.nextRoomID, 1)
}

func (l *Lobby) JoinRoom(client *Client, roomID int32) {
	room, exists := l.Rooms[roomID]
	if !exists {
		fmt.Printf("Room %d does not exist\n", roomID)
		return
	}

	if len(room.Players) >= 2 {
		fmt.Printf("Room %d is full\n", roomID)
		return
	}

	room.Players = append(room.Players, client)
	client.State = ClientStateGame

	fmt.Printf("Client %d joined Room %d\n", client.ID, roomID)

	header := make([]byte, 6)
	header[0] = byte(ClientStateLobby)
	header[1] = byte(LobbyJoinRoom)

	payload := make([]byte, 5)
	payload[0] = byte(LobbyJoinRoom)
	binary.BigEndian.PutUint32(payload[1:5], uint32(roomID))

	binary.BigEndian.PutUint32(header[2:6], uint32(len(payload)))

	packet := append(header, payload...)

	client.Send <- packet
}

// header [Scene] [SceneCommand] [DataLength] payload [[roomID][nameLen][roomName][roomPlayerCount]...]
func (l *Lobby) refreshRooms(client *Client) bool {
	// payload 생성
	payload := []byte{}

	payload = append(payload, byte(len(l.Rooms)))

	for _, room := range l.Rooms {
		roomData := make([]byte, 5)

		binary.BigEndian.PutUint32(roomData[0:4], uint32(room.ID))
		roomData[4] = byte(len(room.Name))

		payload = append(payload, roomData...)
		payload = append(payload, []byte(room.Name)...)
		payload = append(payload, byte(len(room.Players)))
	}

	// header 생성
	header := make([]byte, 6)

	header[0] = byte(ClientStateLobby)
	header[1] = byte(LobbyRefreshRooms)

	binary.BigEndian.PutUint32(header[2:6], uint32(len(payload)))

	// 최종 packet
	packet := append(header, payload...)

	client.Send <- packet

	return true
}

func (l *Lobby) createRoom(roomName string) {
	roomID := l.generateRoomID()
	l.Rooms[roomID] = &Room{
		ID:      roomID,
		Name:    roomName,
		Players: []*Client{},
	}

	fmt.Printf("Room %d created\n", roomID)
}
