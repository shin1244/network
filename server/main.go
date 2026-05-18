package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync/atomic"
)

type Server struct {
	Join  chan *Client
	Leave chan *Client

	Read chan Message

	Clients      map[*Client]bool
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
		Join:    make(chan *Client),
		Leave:   make(chan *Client),
		Clients: make(map[*Client]bool),
		Read:    make(chan Message),
		Lobby: &Lobby{
			Rooms:      make(map[int32]*Room),
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
		server.Clients[client] = true

		header := make([]byte, 6)
		header[0] = byte(ClientStateLobby)
		header[1] = byte(JoinGame)

		payload := make([]byte, 4)
		binary.BigEndian.PutUint32(payload[1:4], uint32(client.ID))

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
			fmt.Printf("Client %d joined\n", client.ID)

		case client := <-s.Leave:
			delete(s.Clients, client)
			fmt.Printf("Client %d left\n", client.ID)

		case msg := <-s.Read:
			switch msg.Client.State {
			case ClientStateLobby:
				s.Lobby.LobbyManager(msg)
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
		scene := msg[0]
		command := msg[1]

		payload := msg[6:]

		fmt.Printf("Received UDP message from %s: Scene=%d, Command=%d, Payload=%v\n", addr, scene, command, payload)
	}
}
