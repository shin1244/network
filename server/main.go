package main

import "net"

type Server struct {
	Clients map[*Client]bool
}

type ClientState int

const (
	ClientStateLobby ClientState = iota
	ClientStateWait
	ClientStateGame
)

type Client struct {
	Conn net.Conn

	ID    int
	State ClientState
}
