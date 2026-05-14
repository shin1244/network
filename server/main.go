package main

import (
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

	Lobby *Lobby
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

	s := &Server{
		Join:    make(chan *Client),
		Leave:   make(chan *Client),
		Msg:     make(chan Message),
		Clients: make(map[*Client]bool),
		Lobby: &Lobby{
			Rooms:      make(map[int32]*Room),
			nextRoomID: 0,
		},
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
				s.LobbyManager(msg)
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

func (c *Client) WritePacket(packet []byte) error {
	_, err := c.Conn.Write(packet)
	return err
}
