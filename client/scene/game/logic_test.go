package game

import (
	"encoding/binary"
	"reflect"
	"testing"
	"time"
)

func TestStateStepIsDeterministic(t *testing.T) {
	first := NewState(Player1)
	second := NewState(Player1)
	inputs := []Input{
		{Player1: AxisUp, Player2: AxisDown},
		{Player1: AxisUp, Player2: AxisNeutral},
		{Player1: AxisNeutral, Player2: AxisDown},
		{Player1: AxisDown, Player2: AxisUp},
	}

	for i := 0; i < 240; i++ {
		input := inputs[i%len(inputs)]
		first.Step(input)
		second.Step(input)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same inputs produced different states: first=%+v second=%+v", first, second)
	}
}

func TestPaddlesStayInsideScreen(t *testing.T) {
	state := NewState(Player1)

	for i := 0; i < 200; i++ {
		state.Step(Input{Player1: AxisUp, Player2: AxisDown})
	}

	if state.LeftPaddle.Y != 0 {
		t.Fatalf("left paddle escaped top clamp: y=%d", state.LeftPaddle.Y)
	}
	if state.RightPaddle.Y != ScreenHeight-PaddleHeight {
		t.Fatalf("right paddle escaped bottom clamp: y=%d", state.RightPaddle.Y)
	}
}

func TestScoreResetsBall(t *testing.T) {
	state := NewState(Player1)
	state.Ball.X = -BallSize - 1
	state.Ball.VX = -BallDX

	state.Step(Input{})

	if state.RightScore != 1 {
		t.Fatalf("right score = %d, want 1", state.RightScore)
	}
	if state.Ball.X != (ScreenWidth-BallSize)/2 || state.Ball.Y != (ScreenHeight-BallSize)/2 {
		t.Fatalf("ball did not reset to center: %+v", state.Ball)
	}
	if state.Ball.VX != -BallDX {
		t.Fatalf("ball vx = %d, want %d", state.Ball.VX, -BallDX)
	}
}

func TestUnackedInputIsResentWithoutAck(t *testing.T) {
	transport := newFakeTransport()
	session := NewNetplaySession(Player1, transport)

	session.SendLocalInput(7, AxisUp)
	if len(transport.sent) != 1 {
		t.Fatalf("sent packet count = %d, want 1", len(transport.sent))
	}

	pending := session.unackedInputs[7]
	pending.lastSentAt = time.Now().Add(-InputResendInterval)
	session.unackedInputs[7] = pending
	session.ResendUnackedInputs()

	if len(transport.sent) != 2 {
		t.Fatalf("sent packet count = %d, want 2", len(transport.sent))
	}
	if got := packetTick(transport.sent[1].payload); got != 7 {
		t.Fatalf("resent tick = %d, want 7", got)
	}
}

func TestAckedInputIsNotResent(t *testing.T) {
	transport := newFakeTransport()
	session := NewNetplaySession(Player1, transport)
	state := NewState(Player1)

	session.SendLocalInput(9, AxisDown)
	transport.events <- PacketEvent{Command: UDPAck, Payload: tickPayload(9)}
	session.ProcessIncoming(state)

	if _, ok := session.unackedInputs[9]; ok {
		t.Fatal("tick 9 remained queued after ack was processed")
	}

	session.ResendUnackedInputs()
	if len(transport.sent) != 1 {
		t.Fatalf("sent packet count = %d, want only original send", len(transport.sent))
	}
}

func TestRemoteInputIsQueuedAndAcked(t *testing.T) {
	transport := newFakeTransport()
	session := NewNetplaySession(Player1, transport)
	state := NewState(Player1)

	payload := append(tickPayload(12), byte(int8(AxisDown)))
	transport.events <- PacketEvent{Command: Player2, Payload: payload}
	session.ProcessIncoming(state)

	frame := state.CommandQueue[12]
	if frame == nil || !frame.P2Ready || frame.P2Input != AxisDown {
		t.Fatalf("remote input was not queued correctly: %+v", frame)
	}
	if len(transport.sent) != 1 {
		t.Fatalf("sent packet count = %d, want ack", len(transport.sent))
	}
	if transport.sent[0].command != UDPAck {
		t.Fatalf("sent command = %d, want UDPAck", transport.sent[0].command)
	}
}

func TestDecodeReplay(t *testing.T) {
	data := []byte("PONGREP1")
	data = binary.BigEndian.AppendUint32(data, 3)
	data = binary.BigEndian.AppendUint32(data, 2)

	data = binary.BigEndian.AppendUint32(data, 10)
	data = append(data, replayAxisByte(AxisUp), replayAxisByte(AxisNeutral), 1, 1)

	data = binary.BigEndian.AppendUint32(data, 11)
	data = append(data, replayAxisByte(AxisNeutral), replayAxisByte(AxisDown), 1, 1)

	replay, err := DecodeReplay(data)
	if err != nil {
		t.Fatalf("DecodeReplay returned error: %v", err)
	}

	if replay.RoomID != 3 {
		t.Fatalf("room id = %d, want 3", replay.RoomID)
	}
	if len(replay.Frames) != 2 {
		t.Fatalf("frame count = %d, want 2", len(replay.Frames))
	}
	if replay.Frames[0].Tick != 10 || replay.Frames[0].P1 != AxisUp || replay.Frames[0].P2 != AxisNeutral {
		t.Fatalf("first frame decoded incorrectly: %+v", replay.Frames[0])
	}
	if replay.Frames[1].Tick != 11 || replay.Frames[1].P1 != AxisNeutral || replay.Frames[1].P2 != AxisDown {
		t.Fatalf("second frame decoded incorrectly: %+v", replay.Frames[1])
	}
}

type sentPacket struct {
	command byte
	payload []byte
}

type fakeTransport struct {
	ready  bool
	events chan PacketEvent
	sent   []sentPacket
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		ready:  true,
		events: make(chan PacketEvent, 10),
	}
}

func (f *fakeTransport) Send(command byte, payload []byte) error {
	f.sent = append(f.sent, sentPacket{
		command: command,
		payload: append([]byte(nil), payload...),
	})
	return nil
}

func (f *fakeTransport) Events() <-chan PacketEvent {
	return f.events
}

func (f *fakeTransport) IsReady() bool {
	return f.ready
}

func tickPayload(tick uint32) []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, tick)
	return payload
}

func packetTick(payload []byte) uint32 {
	return binary.BigEndian.Uint32(payload[0:4])
}

func replayAxisByte(axis Axis) byte {
	return byte(int8(axis))
}
