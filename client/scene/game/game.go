package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type GameScene struct {
	state *State

	sendPacket    func([]byte) error
	OnChangeScene func(sceneID int, data []byte)
}

func NewGameScene(sendPacket func([]byte) error, onChangeScene func(sceneID int, data []byte)) *GameScene {
	return &GameScene{
		state: NewState(),

		sendPacket:    sendPacket,
		OnChangeScene: onChangeScene,
	}
}

func (g *GameScene) Update() error {
	g.state.Step(Input{
		Left:  keyboardAxis(ebiten.KeyW, ebiten.KeyS),
		Right: keyboardAxis(ebiten.KeyArrowUp, ebiten.KeyArrowDown),
	})
	return nil
}

func (g *GameScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{18, 18, 22, 255})

	for y := 0; y < ScreenHeight; y += 24 {
		vector.FillRect(screen, ScreenWidth/2-1, float32(y), 2, 12, color.RGBA{90, 90, 100, 255}, false)
	}

	vector.FillRect(screen, float32(g.state.LeftPaddle.X), float32(g.state.LeftPaddle.Y), PaddleWidth, PaddleHeight, color.White, false)
	vector.FillRect(screen, float32(g.state.RightPaddle.X), float32(g.state.RightPaddle.Y), PaddleWidth, PaddleHeight, color.White, false)
	vector.FillRect(screen, float32(g.state.Ball.X), float32(g.state.Ball.Y), BallSize, BallSize, color.White, false)

	score := fmt.Sprintf("%d          %d", g.state.LeftScore, g.state.RightScore)
	ebitenutil.DebugPrintAt(screen, score, ScreenWidth/2-42, 24)
}

func (g *GameScene) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return ScreenWidth, ScreenHeight
}

func keyboardAxis(up, down ebiten.Key) Axis {
	switch {
	case ebiten.IsKeyPressed(up) && !ebiten.IsKeyPressed(down):
		return AxisUp
	case ebiten.IsKeyPressed(down) && !ebiten.IsKeyPressed(up):
		return AxisDown
	default:
		return AxisNeutral
	}
}

// func (g *GameScene) HandleServerEvent(event network.Event) {
// 	scene := event.Scene
// 	cmd := event.SceneCommand

// 	if scene != byte(1) {
// 		return
// 	}

// 	switch cmd {
// 	case byte(0): // 상대 접속
// 		addr, err := net.ResolveUDPAddr("udp", ":0")
// 		if err != nil {
// 			log.Printf("failed to resolve UDP address: %v", err)
// 			return
// 		}
// 		conn, err := net.ListenUDP("udp", addr)
// 		if err != nil {
// 			log.Printf("failed to listen on UDP: %v", err)
// 			return
// 		}

// 		// network.MakePacket(1, 0, conn.)

// 	case byte(1): // 상대 접속 종료
// 	}
// }
