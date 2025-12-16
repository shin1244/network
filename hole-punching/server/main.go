package main

import (
	"fmt"
	"net"
)

var peers []*net.UDPAddr

func main() {
	addr, err := net.ResolveUDPAddr("udp", ":8080")
	if err != nil {
		panic(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	fmt.Println("UDP server listening on port 8080")

	buffer := make([]byte, 1024)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		if string(buffer[:n]) != "new" {
			continue
		}

		if !isRegistered(clientAddr) {
			peers = append(peers, clientAddr)
			fmt.Printf("Registered new peer: %s\n", clientAddr.String())
		}

		if len(peers) == 2 {
			peerA := peers[0]
			peerB := peers[1]

			fmt.Println("Matching!")

			sendAddr(conn, peerA, peerB)
			sendAddr(conn, peerB, peerA)

			peers = []*net.UDPAddr{}
			fmt.Println("Cleared peers list")
		}
	}
}

func isRegistered(addr *net.UDPAddr) bool {
	for _, peer := range peers {
		if peer.String() == addr.String() {
			return true
		}
	}
	return false
}

func sendAddr(conn *net.UDPConn, to *net.UDPAddr, addr *net.UDPAddr) {
	var message string
	message = fmt.Sprintf("%s:%d", addr.IP.String(), addr.Port)

	_, err := conn.WriteToUDP([]byte(message), to)
	if err != nil {
		fmt.Printf("Error sending address to %s: %v\n", to.String(), err)
	}
}
