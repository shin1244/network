package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
)

type Server struct {
	Join  chan *Client
	Leave chan *Client

	Read chan Message

	Clients      map[*Client]bool
	ClientByID   map[int32]*Client
	nextClientID int32

	UDPConn *net.UDPConn

	Lobby *Lobby
}

type Message struct {
	Client  *Client
	Scene   byte
	Command byte
	Payload []byte
}

type Client struct {
	Conn net.Conn

	ID    int32
	State ClientState

	UDPAddr *net.UDPAddr

	Send chan []byte
}

type ClientState int

const (
	ClientStateLobby ClientState = iota
	ClientStateGame
)

func main() {
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	fmt.Println("Server started on :9000")

	server := &Server{
		Join:       make(chan *Client),
		Leave:      make(chan *Client),
		Clients:    make(map[*Client]bool),
		ClientByID: make(map[int32]*Client),
		Read:       make(chan Message),
		Lobby: &Lobby{
			Rooms:      make(map[int32]*Room),
			ClientRoom: make(map[int32]*Room),
			nextRoomID: 0,
		},
	}

	go server.Route()

	addr, err := net.ResolveUDPAddr("udp", ":5555")
	if err != nil {
		fmt.Printf("Failed to resolve UDP address: %v\n", err)
		return
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Failed to listen on UDP: %v\n", err)
		return
	}
	defer conn.Close()

	server.UDPConn = conn
	go server.udpReadLoop()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}

		client := &Client{
			Conn:  conn,
			ID:    server.generateClientID(),
			State: ClientStateLobby,

			Send: make(chan []byte, 32),
		}

		server.Join <- client

		header := make([]byte, 6)
		header[0] = byte(ClientStateLobby)
		header[1] = byte(JoinGame)

		payload := make([]byte, 4)
		binary.BigEndian.PutUint32(payload, uint32(client.ID))

		binary.BigEndian.PutUint32(header[2:6], uint32(len(payload)))

		client.Send <- append(header, payload...)

		go server.readLoop(client)
		go client.WriteLoop()
	}
}

func (s *Server) Route() {
	for {
		select {
		case client := <-s.Join:
			s.Clients[client] = true
			s.ClientByID[client.ID] = client
			fmt.Printf("Client %d joined\n", client.ID)

		case client := <-s.Leave:
			s.Lobby.LeaveRoom(client)

			delete(s.Clients, client)
			delete(s.ClientByID, client.ID)
			fmt.Printf("Client %d left\n", client.ID)

		case msg := <-s.Read:
			switch msg.Client.State {
			case ClientStateLobby:
				room, ready := s.Lobby.LobbyManager(msg)
				if ready {
					log.Printf("Starting game in room %d\n", room.ID)
					s.TryStartHolePunching(msg.Client)
				}
			case ClientStateGame:
				if msg.Scene != 1 {
					continue
				}
				switch msg.Command {
				case GameOverCommand:
					if len(msg.Payload) < 10 {
						log.Printf("invalid game over payload from client %d: %d bytes", msg.Client.ID, len(msg.Payload))
						continue
					}
					report := GameOverReport{
						Winner:       int32(binary.BigEndian.Uint32(msg.Payload[0:4])),
						LeftScore:    msg.Payload[4],
						RightScore:   msg.Payload[5],
						GameOverTick: binary.BigEndian.Uint32(msg.Payload[6:10]),
					}
					s.Lobby.HandleGameOver(msg.Client, report)
				case ReplayBatchCommand:
					inputs, err := parseReplayBatch(msg.Payload)
					if err != nil {
						log.Printf("Error parsing replay batch: %v", err)
						continue
					}
					s.Lobby.HandleReplayBatch(msg.Client, inputs)
				}
			}
		}
	}
}

// generateClientID는 새로운 클라이언트 ID를 생성합니다.
// 원자적 연산을 사용하여 안전하게 증가시킵니다.
func (s *Server) generateClientID() int32 {
	return atomic.AddInt32(&s.nextClientID, 1)
}

func (s *Server) readLoop(c *Client) {
	for {
		var header [6]byte

		_, err := io.ReadFull(c.Conn, header[:]) // 성능 최적화를 위해 고정 크기의 배열 사용
		if err != nil {
			fmt.Printf("Client %d disconnected\n", c.ID)
			return
		}

		scene := header[0]
		command := header[1]

		payloadLength := binary.BigEndian.Uint32(
			header[2:6],
		)

		// Payload 읽기
		payload := make([]byte, payloadLength)

		_, err = io.ReadFull(c.Conn, payload)
		if err != nil {
			fmt.Printf("Client %d disconnected\n", c.ID)
			return
		}

		// 완성된 packet 전달
		s.Read <- Message{
			Client:  c,
			Scene:   scene,
			Command: command,
			Payload: payload,
		}
	}
}

func (c *Client) WriteLoop() {
	for packet := range c.Send {
		_, err := c.Conn.Write(packet)
		if err != nil {
			fmt.Printf("Client %d write error: %v\n", c.ID, err)

			c.Conn.Close()
			return
		}
	}
}

func (s *Server) udpReadLoop() {
	buf := make([]byte, 1024)
	for {
		n, addr, err := s.UDPConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("UDP read error: %v\n", err)
			return
		}

		msg := buf[:n]
		log.Printf("Received UDP message from %v: %v\n", addr, msg)
		scene := msg[0]
		command := msg[1]

		payload := msg[6:]

		if scene == 1 && command == 2 {
			log.Printf("Received hole punching request from %v\n", addr)
			log.Printf("Payload: %v\n", payload)
			clientID := int32(binary.BigEndian.Uint32(payload))

			client, ok := s.ClientByID[clientID]
			if !ok {
				fmt.Printf("Unknown client ID %d from %v\n", clientID, addr)
				continue
			}
			client.UDPAddr = addr
			s.TryStartHolePunching(client)
		}
	}
}

func MakePacket(scene byte, command byte, payload []byte) []byte {
	header := make([]byte, 6)

	header[0] = scene
	header[1] = command

	binary.BigEndian.PutUint32(
		header[2:6],
		uint32(len(payload)),
	)

	return append(header, payload...)
}

func (s *Server) TryStartHolePunching(client *Client) {
	if client.UDPAddr == nil {
		log.Printf("Client %d has not established UDP connection yet", client.ID)
		return
	}

	room := s.Lobby.ClientRoom[client.ID]
	if room == nil {
		log.Printf("Client %d is not in any room", client.ID)
		return
	}

	if !room.IsReadyForPunching() {
		log.Printf("Room %d is not ready for hole punching", room.ID)
		return
	}

	s.StartHolePunching(room)
}

func (r *Room) IsReadyForPunching() bool {
	if len(r.Players) != 2 {
		return false
	}

	return r.Players[0].UDPAddr != nil &&
		r.Players[1].UDPAddr != nil
}
