package main

import (
	"fmt"
	"net"
)

type Server struct {
	Join  chan *Client
	Leave chan *Client
	Msg   chan Message

	Clients map[*Client]bool
}

type Message struct {
	Client *Client
	Data   []byte
}

type Client struct {
	Conn net.Conn

	ID    int
	State ClientState
}

type ClientState int

const (
	ClientStateLobby ClientState = iota
	ClientStateWait
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
	}

	go s.Run()

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

func (s *Server) Run() {
	for {
		select {
		case client := <-s.Join:
			s.Clients[client] = true
			fmt.Printf("Client %d joined\n", client.ID)
		case client := <-s.Leave:
			delete(s.Clients, client)
			fmt.Printf("Client %d left\n", client.ID)
		case msg := <-s.Msg:
			fmt.Printf("Message from client %d: %s\n", msg.Client.ID, string(msg.Data))
		}
	}
}

func (s *Server) generateClientID() int {
	return len(s.Clients) + 1
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

func parseMessage(data []byte) (string, error) {

}
