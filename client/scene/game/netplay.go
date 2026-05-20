package game

import (
	"encoding/binary"
	"log"
	"time"
)

const InputResendInterval = 100 * time.Millisecond

type PacketEvent struct {
	Command byte
	Payload []byte
}

type GameTransport interface {
	Send(command byte, payload []byte) error
	Events() <-chan PacketEvent
	IsReady() bool
}

type pendingInput struct {
	input      Axis
	lastSentAt time.Time
}

type NetplaySession struct {
	player    uint8
	remote    uint8
	transport GameTransport

	unackedInputs map[uint32]pendingInput
}

// 게임의 네트워크 세션 관리 - 로컬 입력 송신, 원격 입력 수신 및 ACK 처리 담당
func NewNetplaySession(player uint8, transport GameTransport) *NetplaySession {
	remote := Player1
	if player == Player1 {
		remote = Player2
	}

	return &NetplaySession{
		player:        player,
		remote:        remote,
		transport:     transport,
		unackedInputs: make(map[uint32]pendingInput),
	}
}

func (n *NetplaySession) IsReady() bool {
	return n.transport.IsReady()
}

func (n *NetplaySession) ProcessIncoming(state *State) {
	for {
		select {
		case event := <-n.transport.Events():
			n.handleEvent(state, event)
		default:
			return
		}
	}
}

func (n *NetplaySession) handleEvent(state *State, event PacketEvent) {
	switch event.Command {
	case UDPAck:
		if len(event.Payload) < 4 {
			return
		}
		ackedTick := binary.BigEndian.Uint32(event.Payload[0:4])
		delete(n.unackedInputs, ackedTick)

	case n.remote:
		if len(event.Payload) < 5 {
			return
		}

		execTick := binary.BigEndian.Uint32(event.Payload[0:4])
		execInput := Axis(int8(event.Payload[4]))
		state.PushCommand(execTick, n.remote, execInput)

		ackPayload := append([]byte(nil), event.Payload[0:4]...)
		if err := n.transport.Send(UDPAck, ackPayload); err != nil {
			log.Printf("failed to send ack for tick %d: %v", execTick, err)
		}
	}
}

func (n *NetplaySession) SendLocalInput(tick uint32, input Axis) {
	n.unackedInputs[tick] = pendingInput{
		input:      input,
		lastSentAt: time.Now(),
	}
	n.sendInput(tick, input)
}

func (n *NetplaySession) ResendUnackedInputs() {
	now := time.Now()
	for tick, pending := range n.unackedInputs {
		if now.Sub(pending.lastSentAt) < InputResendInterval {
			continue
		}

		n.sendInput(tick, pending.input)
		pending.lastSentAt = now
		n.unackedInputs[tick] = pending
	}
}

func (n *NetplaySession) sendInput(tick uint32, input Axis) {
	payload := make([]byte, 5)
	binary.BigEndian.PutUint32(payload[0:4], tick)
	payload[4] = byte(int8(input))

	if err := n.transport.Send(n.player, payload); err != nil {
		log.Printf("failed to send local input tick %d: %v", tick, err)
	}
}
