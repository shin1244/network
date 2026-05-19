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
	JoinGame
)

type Room struct {
	ID      int32
	Name    string
	Players []*Client
}

type Lobby struct {
	Rooms      map[int32]*Room // 방 ID를 키로 하는 방 목록
	nextRoomID int32

	ClientRoom map[int32]*Room // 클라이언트 ID를 키로 하는 클라이언트-방 매핑
}

// Lobby 함수는 클라이언트로부터 받은 데이터를 처리하여 로비 관련 명령을 수행합니다.
// 05.19 수정 사항 ) JoinRoom의 네트워크 송신 부분 분리 LobbyManager 함수로 이동
func (l *Lobby) LobbyManager(msg Message) (*Room, bool) {
	switch LobbyCommand(msg.Command) {
	case LobbyCreateRoom:
		roomName := string(msg.Payload)
		l.createRoom(roomName)
		fmt.Printf("Room '%s' created\n", roomName)
	case LobbyJoinRoom:
		roomID := int32(binary.BigEndian.Uint32(msg.Payload))

		room, err := l.JoinRoom(msg.Client, roomID)
		if err != nil {
			fmt.Printf("Error joining room %d: %v\n", roomID, err)
			return nil, false
		}

		fmt.Printf("Room %d joined\n", roomID)

		player := len(room.Players)
		payload := make([]byte, 4)
		binary.BigEndian.PutUint32(payload, uint32(player))

		packet := MakePacket(
			byte(ClientStateLobby),
			byte(LobbyJoinRoom),
			payload,
		)

		msg.Client.Send <- packet

		return room, room.IsReady()

	case LobbyRefreshRooms:
		payload := l.RoomListPayload()

		packet := MakePacket(
			byte(ClientStateLobby),
			byte(LobbyRefreshRooms),
			payload,
		)
		msg.Client.Send <- packet
	}
	return nil, false
}

// generateRoomID는 새로운 방 ID를 생성합니다.
// 원자적 연산을 사용하여 안전하게 증가시킵니다.
func (l *Lobby) generateRoomID() int32 {
	return atomic.AddInt32(&l.nextRoomID, 1)
}

func (l *Lobby) JoinRoom(client *Client, roomID int32) (*Room, error) {
	room, exists := l.Rooms[roomID]
	if !exists {
		return nil, fmt.Errorf("room %d does not exist", roomID)
	}

	if !room.CanJoin(client) {
		return nil, fmt.Errorf("room %d is full", roomID)
	}

	room.Players = append(room.Players, client)
	l.ClientRoom[client.ID] = room
	client.State = ClientStateGame

	return room, nil
}

func (r *Room) IsFull() bool {
	return len(r.Players) >= 2
}

func (r *Room) CanJoin(client *Client) bool {
	return !r.IsFull()
}

func (r *Room) IsReady() bool {
	return len(r.Players) == 2
}

// header [Scene] [SceneCommand] [DataLength] payload [[roomID][nameLen][roomName][roomPlayerCount]...]
func (l *Lobby) RoomListPayload() []byte {
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

	return payload
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
