package game

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"pong/network"
	"time"
)

const (
	UDPRegister byte = 2
	UDPPunch    byte = 3
	UDPAck      byte = 4
)

type UDPTransport struct {
	conn   *net.UDPConn
	events chan PacketEvent

	peer *net.UDPAddr
}

// UDP 전송 구현 - 서버와의 홀펀칭 및 게임 패킷 "송수신" 담당
func NewUDPTransport(clientID int32) (*UDPTransport, error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return nil, fmt.Errorf("resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen UDP: %w", err)
	}

	t := &UDPTransport{
		conn:   conn,
		events: make(chan PacketEvent, 100),
	}

	t.register(clientID)
	go t.readLoop()

	return t, nil
}

func (t *UDPTransport) Events() <-chan PacketEvent {
	return t.events
}

func (t *UDPTransport) IsReady() bool {
	return t.peer != nil
}

func (t *UDPTransport) Send(command byte, payload []byte) error {
	peer := t.peer

	if peer == nil {
		return nil
	}

	_, err := t.conn.WriteToUDP(network.MakePacket(1, command, payload), peer)
	return err
}

func (t *UDPTransport) StartHolePunching(data []byte) {
	peer, err := DeserializeUDPAddr(data)
	if err != nil {
		log.Printf("failed to deserialize peer addr: %v", err)
		return
	}

	log.Printf("start hole punching to %v", peer)

	packet := network.MakePacket(1, UDPPunch, []byte("punch"))
	for i := 0; i < 10; i++ {
		if _, err := t.conn.WriteToUDP(packet, peer); err != nil {
			log.Printf("failed to send punch packet to %v: %v", peer, err)
			return
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (t *UDPTransport) register(clientID int32) {
	payload := binary.BigEndian.AppendUint32(nil, uint32(clientID))
	serverAddr, err := net.ResolveUDPAddr("udp", "localhost:5555")
	if err != nil {
		log.Printf("failed to resolve server UDP address: %v", err)
		return
	}

	if _, err := t.conn.WriteToUDP(network.MakePacket(1, UDPRegister, payload), serverAddr); err != nil {
		log.Printf("failed to register udp addr: %v", err)
	}
}

func (t *UDPTransport) readLoop() {
	buf := make([]byte, 1024)

	for {
		n, addr, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("udp read error: %v", err)
			return
		}

		if n < 6 {
			log.Printf("invalid udp packet from %v: too short (%d bytes)", addr, n)
			continue
		}

		msg := buf[:n]
		scene := msg[0]
		if scene != 1 {
			continue
		}

		command := msg[1]
		payloadLen := int(binary.BigEndian.Uint32(msg[2:6]))
		if n < 6+payloadLen {
			log.Printf("invalid udp packet from %v: payload length mismatch (expected %d, got %d)", addr, 6+payloadLen, n)
			continue
		}

		payload := append([]byte(nil), msg[6:6+payloadLen]...)

		switch command {
		case UDPPunch:
			ack := network.MakePacket(1, UDPAck, nil)
			if _, err := t.conn.WriteToUDP(ack, addr); err != nil {
				log.Printf("failed to send ack to %v: %v", addr, err)
			}

		case UDPAck:
			if len(payload) == 0 { // 홀펀칭 성공 패킷
				t.setPeer(addr)
				continue
			}
			t.events <- PacketEvent{Command: command, Payload: payload}

		default:
			t.events <- PacketEvent{Command: command, Payload: payload}
		}
	}
}

func (t *UDPTransport) setPeer(addr *net.UDPAddr) {
	t.peer = addr
}

func DeserializeUDPAddr(data []byte) (*net.UDPAddr, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("invalid udp addr payload: %d", len(data))
	}

	ip := net.IPv4(data[0], data[1], data[2], data[3])
	port := int(binary.BigEndian.Uint16(data[4:6]))

	return &net.UDPAddr{
		IP:   ip,
		Port: port,
	}, nil
}

func (t *UDPTransport) Close() error {
	return t.conn.Close()
}
