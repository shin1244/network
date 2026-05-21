package game

import (
	"fmt"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const defaultReplayPath = "C:\\Users\\shin\\Desktop\\Project\\PONG\\server\\replay\\room_1.rep"

type ReplayScene struct {
	state  *State
	replay *StoredReplay
	index  int
}

func NewReplayScene(replay *StoredReplay) *ReplayScene {
	if replay == nil {
		loaded, err := LoadReplay(defaultReplayPath)
		if err != nil {
			log.Printf("failed to load replay: %v", err)
		} else {
			replay = loaded
		}
	}

	return &ReplayScene{
		state:  NewState(Player1),
		replay: replay,
		index:  0,
	}
}

func (r *ReplayScene) Update() error {
	for i := 0; i < 2; i++ {
		r.stepReplay()
	}
	return nil
}

func (r *ReplayScene) Draw(screen *ebiten.Image) {
	drawState(screen, r.state)

	if r.replay == nil {
		ebitenutil.DebugPrintAt(screen, "Replay not found", ScreenWidth/2-58, ScreenHeight/2-8)
		return
	}

	progress := fmt.Sprintf("Replay %d / %d", r.index, len(r.replay.Frames))
	ebitenutil.DebugPrintAt(screen, progress, 20, 20)
}

func (r *ReplayScene) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return ScreenWidth, ScreenHeight
}

func (r *ReplayScene) stepReplay() {
	if r.replay == nil || r.index >= len(r.replay.Frames) {
		return
	}

	frame := r.replay.Frames[r.index]
	if frame.P1Ready && frame.P2Ready {
		r.state.Step(Input{
			Player1: frame.P1,
			Player2: frame.P2,
		})
	}

	r.index++
}
