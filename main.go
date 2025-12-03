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
	defer func() {
		server.u.Remove(conn)
		conn.Close()
	}()

	reader := bufio.NewReader(conn)

	for {
		// 1. 헤더(명령어) 1바이트 먼저 읽기
		head, err := reader.ReadByte()
		if err != nil {
			return
		}

		fmt.Println("Received head:", head)

		switch head {
		case MsgMatch:
			// 매칭은 추가 데이터 없이 헤더만으로 처리
			isQueued := server.q.Toggle(conn)

			// ★ 핵심: 클라이언트에게 현재 상태를 알려줌 (피드백)
			if isQueued {
				conn.Write([]byte{MsgChat})
				conn.Write([]byte("[System] Matching queue registered.\n"))
			} else {
				conn.Write([]byte{MsgChat})
				conn.Write([]byte("[System] Matching cancelled.\n"))
			}
			fmt.Println("Match Toggle:", isQueued)

		case MsgChat:
			// 채팅은 엔터(\n)까지 읽기
			body, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}

			server.message <- []byte{MsgChat}
			server.message <- body
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
