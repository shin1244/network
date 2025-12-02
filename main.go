package main

import (
	"bufio"
	"fmt"
	"net"
	"server/match"
	"server/users"
)

const (
	MsgChat  uint8 = 1
	MsgMatch uint8 = 2
)

type Server struct {
	u       *users.Users
	q       *match.Match
	message chan []byte
}

func main() {
	ln, _ := net.Listen("tcp", ":9909")
	defer ln.Close()

	server := newServer()
	go server.matchMaker()
	go server.broadcast()

	for {
		conn, _ := ln.Accept()
		go handleConn(conn, server)
	}
}

func handleConn(conn net.Conn, server *Server) {
	server.u.Add(conn)
	fmt.Println("New connection:", conn.RemoteAddr())
	defer func() {
		server.u.Remove(conn)
		conn.Close()
	}()

	reader := bufio.NewReader(conn)
	for {
		head, _ := reader.ReadByte()

		switch head {
		case MsgMatch:
			server.q.Toggle(conn)
		case MsgChat:
			msg, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			server.message <- msg
		}
	}
}

func newServer() *Server {
	return &Server{
		u:       users.NewUsers(),
		q:       match.NewMatch(),
		message: make(chan []byte),
	}
}

func (s *Server) broadcast() {
	for m := range s.message {
		s.u.Broadcast(m)
	}
}

func (s *Server) matchMaker() {
	s.q.MatchMaker()
}
