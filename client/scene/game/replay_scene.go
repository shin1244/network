package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var replaySpeeds = []float64{0.5, 1, 2, 4}

const (
	replaySpeedButtonX = 520
	replaySpeedButtonY = 18
	replaySpeedButtonW = 90
	replaySpeedButtonH = 32
)

type ReplayScene struct {
	state           *State
	replay          *StoredReplay
	index           int
	speedIndex      int
	stepAccumulator float64
}

func NewReplayScene(replay *StoredReplay) *ReplayScene {
	return &ReplayScene{
		state:      NewState(Player1),
		replay:     replay,
		index:      0,
		speedIndex: 1,
	}
}

func (r *ReplayScene) Update() error {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && r.isSpeedButtonHovered() {
		r.speedIndex = (r.speedIndex + 1) % len(replaySpeeds)
	}

	r.stepAccumulator += replaySpeeds[r.speedIndex]
	for r.stepAccumulator >= 1 {
		r.stepReplay()
		r.stepAccumulator--
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
	r.drawSpeedButton(screen)
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

func (r *ReplayScene) isSpeedButtonHovered() bool {
	mouseX, mouseY := ebiten.CursorPosition()
	return mouseX >= replaySpeedButtonX &&
		mouseX <= replaySpeedButtonX+replaySpeedButtonW &&
		mouseY >= replaySpeedButtonY &&
		mouseY <= replaySpeedButtonY+replaySpeedButtonH
}

func (r *ReplayScene) drawSpeedButton(screen *ebiten.Image) {
	buttonColor := color.RGBA{70, 70, 90, 255}
	if r.isSpeedButtonHovered() {
		buttonColor = color.RGBA{105, 105, 135, 255}
	}

	vector.FillRect(
		screen,
		replaySpeedButtonX,
		replaySpeedButtonY,
		replaySpeedButtonW,
		replaySpeedButtonH,
		buttonColor,
		false,
	)

	speedText := fmt.Sprintf("%.1fx", replaySpeeds[r.speedIndex])
	if replaySpeeds[r.speedIndex] == float64(int(replaySpeeds[r.speedIndex])) {
		speedText = fmt.Sprintf("%.0fx", replaySpeeds[r.speedIndex])
	}
	ebitenutil.DebugPrintAt(screen, speedText, replaySpeedButtonX+28, replaySpeedButtonY+10)
}
