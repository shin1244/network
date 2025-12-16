package main

import (
	"fmt"
	"net"
	"strings"
)

func main() {
	// 1. 소켓 열기
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		panic(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	fmt.Println("--------------------------------")
	fmt.Println("클라이언트 시작. 서버에 접속 중...")
	fmt.Println("--------------------------------")

	// 2. 서버에 등록
	serverAddr, _ := net.ResolveUDPAddr("udp", "210.57.239.71:45678")
	conn.WriteToUDP([]byte("new"), serverAddr)

	// 3. 상대방 주소 수신
	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		panic(err)
	}

	fmt.Println("--------------------------------")
	peerInfo := strings.TrimSpace(string(buffer[:n]))
	fmt.Println("받은 peerInfo =", peerInfo)
	fmt.Println("--------------------------------")

	peerAddr, err := net.ResolveUDPAddr("udp", peerInfo)
	if err != nil {
		fmt.Println("상대 주소 파싱 실패:", err)
		return
	}

	fmt.Println("--------------------------------")
	fmt.Printf("매칭 성공 상대방 주소: %s\n", peerAddr.String())
	fmt.Println("--------------------------------")

	conn.WriteToUDP([]byte("punch"), peerAddr)
	fmt.Println()

	go func() {
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				fmt.Println("수신 오류:", err)
				continue
			}

			message := string(buffer[:n])
			fmt.Printf("📩 받은 메시지 [%s]: %s\n", remoteAddr.String(), message)
		}
	}()

	go func() {
		var input string
		for {
			fmt.Print("보낼 메시지 입력: ")
			fmt.Scanln(&input)
			sendMessage(conn, peerAddr, input)
		}
	}()

	select {}
}

func sendMessage(conn *net.UDPConn, addr *net.UDPAddr, message string) {
	_, err := conn.WriteToUDP([]byte(message), addr)
	if err != nil {
		fmt.Println("메시지 전송 오류:", err)
	}
}
