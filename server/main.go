package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"
)

type Server struct {
	Join  chan *Client
	Leave chan *Client
	Msg   chan Message

	Clients      map[*Client]bool
	nextClientID int32

	Rooms      map[int32]*Room
	nextRoomID int32
}

type Message struct {
	Client *Client
	Data   []byte
}

type Client struct {
	Conn net.Conn

	ID    int32
	State ClientState
}

type Room struct {
	ID      int32
	Name    string
	Players []*Client
}

type ClientState int

const (
	ClientStateLobby ClientState = iota
	ClientStateWait
	ClientStateGame
)

type LobbyCommand int

const (
	LobbyCreateRoom LobbyCommand = iota
	LobbyJoinRoom
	LobbyRefreshRooms
)

func main() {
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	fmt.Println("Server started on :9000")

	s := &Server{
		Join:    make(chan *Client),
		Leave:   make(chan *Client),
		Msg:     make(chan Message),
		Clients: make(map[*Client]bool),
		Rooms:   make(map[int32]*Room),
	}

	go s.Route()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}

		client := &Client{
			Conn:  conn,
			ID:    s.generateClientID(),
			State: ClientStateLobby,
		}

		s.Join <- client

		go s.handleClient(client)
	}
}

func (s *Server) Route() {
	for {
		select {
		case client := <-s.Join:
			s.Clients[client] = true
			fmt.Printf("Client %d joined\n", client.ID)

		case client := <-s.Leave:
			delete(s.Clients, client)
			fmt.Printf("Client %d left\n", client.ID)

		case msg := <-s.Msg:
			switch msg.Client.State {
			case ClientStateLobby:
				s.Lobby(msg)
			case ClientStateWait:
			case ClientStateGame:
			}
		}
	}
}

// generateClientID는 새로운 클라이언트 ID를 생성합니다.
// 원자적 연산을 사용하여 안전하게 증가시킵니다.
func (s *Server) generateClientID() int32 {
	return atomic.AddInt32(&s.nextClientID, 1)
}

func (s *Server) handleClient(c *Client) {
	defer func() {
		c.Conn.Close()
		s.Leave <- c
	}()

	buf := make([]byte, 1024)
	for {
		n, err := c.Conn.Read(buf)
		if err != nil {
			fmt.Printf("Client %d disconnected\n", c.ID)
			return
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		s.Msg <- Message{Client: c, Data: data}
	}
}

// Lobby 함수는 클라이언트로부터 받은 데이터를 처리하여 로비 관련 명령을 수행합니다.
func (s *Server) Lobby(msg Message) {
	switch LobbyCommand(msg.Data[0]) {
	case LobbyCreateRoom:
		RoomID := s.generateRoomID()
		s.Rooms[RoomID] = &Room{
			ID:      RoomID,
			Name:    fmt.Sprintf("Room %s", string(msg.Data[1:])),
			Players: []*Client{},
		}

		// s.JoinRoom(msg.Client, RoomID) --- IGNORE ---

		fmt.Printf("Room %d created\n", RoomID)
	case LobbyJoinRoom:
		roomID := int32(binary.BigEndian.Uint32(msg.Data[1:5]))
		s.JoinRoom(msg.Client, roomID)
		fmt.Printf("Client %d requested to join room %d\n", msg.Client.ID, roomID)
	case LobbyRefreshRooms:
		s.refreshRooms(msg.Client)
		fmt.Printf("Client %d requested room list refresh\n", msg.Client.ID)
	}
}

// generateRoomID는 새로운 방 ID를 생성합니다.
// 원자적 연산을 사용하여 안전하게 증가시킵니다.
func (s *Server) generateRoomID() int32 {
	return atomic.AddInt32(&s.nextRoomID, 1)
}

func (s *Server) JoinRoom(client *Client, roomID int32) {
	room, exists := s.Rooms[roomID]
	if !exists {
		fmt.Printf("Room %d does not exist\n", roomID)
		return
	}
	room.Players = append(room.Players, client)
	client.State = ClientStateWait
}

func (c *Client) WritePacket(packet []byte) error {
	_, err := c.Conn.Write(packet)
	return err
}

// [Scene] [SceneCommand] [roomLen] [[roomID][nameLen][roomName][roomPlayerCount]...]
func (s *Server) refreshRooms(client *Client) bool {
	buf := []byte{byte(ClientStateLobby)} // Scene: Lobby

	buf = append(buf, byte(LobbyRefreshRooms)) // SceneCommand: RefreshRooms
	buf = append(buf, byte(len(s.Rooms)))      // 방 갯수

	for _, room := range s.Rooms {
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
