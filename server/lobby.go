package main

import (
	"encoding/binary"
	"fmt"
	"log"
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

	Recorder map[uint32]*ReplayFrame

	GameOverVotes map[int32]bool
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
		room := l.createRoom(roomName)

		room, err := l.JoinRoom(msg.Client, room.ID)
		if err != nil {
			fmt.Printf("Error joining room %d: %v\n", room.ID, err)
			return nil, false
		}

		fmt.Printf("Room %d joined\n", room.ID)

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
		log.Printf("Received room list refresh request from Client %d\n", msg.Client.ID)
		payload := l.RoomListPayload()
		log.Printf("Room list payload: %v\n", payload)

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

	room.GameOverVotes[client.ID] = false

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

// 패킷: [방 개수(최대 255)] + 갱신 루프 [방ID(4B)][이름길이(1B)][이름(...)][인원수(1B)]...
func (l *Lobby) RoomListPayload() []byte {
	payload := make([]byte, 1)
	payload[0] = byte(len(l.Rooms))

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

func (l *Lobby) createRoom(roomName string) *Room {
	roomID := l.generateRoomID()
	room := &Room{
		ID:       roomID,
		Name:     roomName,
		Players:  []*Client{},
		Recorder: make(map[uint32]*ReplayFrame),

		GameOverVotes: make(map[int32]bool),
	}
	l.Rooms[roomID] = room
	fmt.Printf("Room %d ('%s') created successfully\n", roomID, roomName)
	return room
}

func (l *Lobby) LeaveRoom(client *Client) {
	room, exists := l.ClientRoom[client.ID]
	if !exists {
		return
	}

	// 방 플레이어 목록에서 제거
	for i, p := range room.Players {
		if p.ID == client.ID {
			room.Players = append(room.Players[:i], room.Players[i+1:]...)
			break
		}
	}
	delete(l.ClientRoom, client.ID)
	fmt.Printf("Client %d removed from Room %d\n", client.ID, room.ID)

	// 방에 아무도 없으면 방 자체를 파괴
	if len(room.Players) == 0 {
		delete(l.Rooms, room.ID)
		fmt.Printf("Room %d closed (No players left)\n", room.ID)
	}
}

func (l *Lobby) HandleGameOver(client *Client, report GameOverReport) {
	room, exists := l.ClientRoom[client.ID]
	if !exists {
		log.Printf("Client %d is not in a room, cannot handle game over report\n", client.ID)
		return
	}

	room.GameOverVotes[client.ID] = true

	allVoted := true
	for _, voted := range room.GameOverVotes {
		if !voted {
			allVoted = false
			break
		}
	}

	log.Println(report)
	if allVoted {
		for _, p := range room.Players {
			p.State = ClientStateLobby
			delete(l.ClientRoom, p.ID)
		}
		delete(l.Rooms, room.ID)
	}
}

func (r *Room) PlayerNumber(client *Client) byte {
	for i, player := range r.Players {
		if player.ID != client.ID {
			continue
		}

		return byte(i + 1)
	}

	return 0
}
