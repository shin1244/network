package game

import (
	"encoding/binary"
	"fmt"
	"log"
	"pong/network"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	SceneLobbyCommand       byte = 0
	LobbyWatchRoomEvent     byte = 5
	observerLiveLagTicks         = 30
	observerCatchUpLagTicks      = 90
	observerLiveSpeed            = 1
	observerCatchUpSpeed         = 4
)

type ObserverScene struct {
	state *State

	latestReceivedTick uint32
	receivedFrames     int
	speed              float64
	accumulator        float64
}

func NewObserverScene(data []byte) *ObserverScene {
	observer := &ObserverScene{
		state: NewState(0),
		speed: observerCatchUpSpeed,
	}

	observer.AppendPayload(data)
	return observer
}

func (o *ObserverScene) Update() error {
	o.updateSpeed()

	o.accumulator += o.speed
	for o.accumulator >= 1 {
		if !o.stepFrame() {
			o.accumulator = 0
			break
		}
		o.accumulator--
	}

	return nil
}

func (o *ObserverScene) Draw(screen *ebiten.Image) {
	drawState(screen, o.state)

	status := fmt.Sprintf(
		"Observer %.0fx  tick %d / %d  frames %d",
		o.speed,
		o.state.Tick,
		o.latestReceivedTick,
		o.receivedFrames,
	)
	ebitenutil.DebugPrintAt(screen, status, 20, 20)
}

func (o *ObserverScene) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return ScreenWidth, ScreenHeight
}

func (o *ObserverScene) HandleServerEvent(event network.Event) {
	if event.Scene != SceneLobbyCommand || event.SceneCommand != LobbyWatchRoomEvent {
		return
	}

	o.AppendPayload(event.Data)
}

func (o *ObserverScene) AppendPayload(data []byte) {
	frames, err := DecodeReplayFrames(data)
	if err != nil {
		log.Printf("failed to decode observer frames: %v", err)
		return
	}

	o.AppendFrames(frames)
}

func (o *ObserverScene) AppendFrames(frames []StoredReplayFrame) {
	if len(frames) == 0 {
		return
	}

	for _, frame := range frames {
		if !frame.P1Ready || !frame.P2Ready {
			continue
		}
		if frame.Tick < o.state.Tick {
			continue
		}

		o.state.PushCommand(frame.Tick, Player1, frame.P1)
		o.state.PushCommand(frame.Tick, Player2, frame.P2)
		o.receivedFrames++

		if frame.Tick > o.latestReceivedTick {
			o.latestReceivedTick = frame.Tick
		}
	}
}

func (o *ObserverScene) updateSpeed() {
	if o.state.Tick >= o.latestReceivedTick {
		o.speed = observerLiveSpeed
		return
	}

	lag := int(o.latestReceivedTick) - int(o.state.Tick)

	if o.speed == observerLiveSpeed && lag > observerCatchUpLagTicks {
		o.speed = observerCatchUpSpeed
		return
	}

	if o.speed == observerCatchUpSpeed && lag <= observerLiveLagTicks {
		o.speed = observerLiveSpeed
	}
}

func (o *ObserverScene) stepFrame() bool {
	frame := o.state.CommandQueue[o.state.Tick]
	if frame == nil || !frame.P1Ready || !frame.P2Ready {
		return false
	}

	o.state.Step(Input{
		Player1: frame.P1Input,
		Player2: frame.P2Input,
	})

	delete(o.state.CommandQueue, o.state.Tick-1)
	return true
}

func DecodeReplayFrames(data []byte) ([]StoredReplayFrame, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("observer frame payload too short: %d bytes", len(data))
	}

	count := int(binary.BigEndian.Uint16(data[0:2]))
	expectedLen := 2 + count*8
	if len(data) != expectedLen {
		return nil, fmt.Errorf("invalid observer frame payload length: got %d, want %d", len(data), expectedLen)
	}

	frames := make([]StoredReplayFrame, 0, count)
	offset := 2
	for i := 0; i < count; i++ {
		tick := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		p1 := Axis(int8(data[offset]))
		offset++

		p2 := Axis(int8(data[offset]))
		offset++

		p1Ready := data[offset] != 0
		offset++

		p2Ready := data[offset] != 0
		offset++

		frames = append(frames, StoredReplayFrame{
			Tick:    tick,
			P1:      p1,
			P2:      p2,
			P1Ready: p1Ready,
			P2Ready: p2Ready,
		})
	}

	return frames, nil
}
