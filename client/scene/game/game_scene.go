package game

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"log"
	"pong/network"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type GameScene struct {
	state    *State
	clientID int32

	transport *UDPTransport
	netplay   *NetplaySession

	countdownStartedAt time.Time
	recorder           []ReplayFrame

	sendPacket    func([]byte) error
	OnChangeScene func(sceneID int, data []byte)
}

type ReplayFrame struct {
	Tick  uint32
	Input Axis
}

func NewGameScene(sendPacket func([]byte) error, onChangeScene func(sceneID int, data []byte), clientID int32, data []byte) *GameScene {
	player := Player1
	if len(data) >= 4 {
		player = uint8(binary.BigEndian.Uint32(data))
	}

	transport, err := NewUDPTransport(clientID)
	if err != nil {
		log.Printf("failed to create udp transport: %v", err)
	}

	g := &GameScene{
		state:         NewState(player),
		clientID:      clientID,
		transport:     transport,
		sendPacket:    sendPacket,
		OnChangeScene: onChangeScene,
		recorder:      []ReplayFrame{},
	}
	if transport != nil {
		g.netplay = NewNetplaySession(player, transport)
	}

	return g
}

func (g *GameScene) Update() error {
	if g.state.Winner != 0 {
		g.finishGame()
		return nil
	}
	if g.netplay != nil {
		g.netplay.ProcessIncoming(g.state)
	}
	if !g.isReadyToStart() {
		return nil
	}
	if g.netplay != nil {
		g.netplay.ResendUnackedInputs()
	}
	g.handleLocalInput()
	g.simulateCurrentTick()

	return nil
}

func (g *GameScene) Draw(screen *ebiten.Image) {
	drawState(screen, g.state)
	g.drawStartStatus(screen)
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

func (g *GameScene) HandleServerEvent(event network.Event) {
	if event.Scene != byte(1) {
		return
	}

	switch event.SceneCommand {
	case byte(0):
		if g.transport != nil {
			go g.transport.StartHolePunching(event.Data)
		}
	}
}

func (g *GameScene) isReadyToStart() bool {
	if g.netplay == nil || !g.netplay.IsReady() {
		return false
	}

	if g.countdownStartedAt.IsZero() {
		g.countdownStartedAt = time.Now()
		return false
	}

	return time.Since(g.countdownStartedAt) >= 3*time.Second
}

func (g *GameScene) drawStartStatus(screen *ebiten.Image) {
	if g.netplay == nil || !g.netplay.IsReady() {
		ebitenutil.DebugPrintAt(screen, "Waiting for peer...", ScreenWidth/2-58, ScreenHeight/2-8)
		return
	}

	if g.countdownStartedAt.IsZero() {
		return
	}

	remaining := 3 - int(time.Since(g.countdownStartedAt)/time.Second)
	if remaining <= 0 {
		return
	}

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", remaining), ScreenWidth/2-4, ScreenHeight/2-8)
}

func (g *GameScene) handleLocalInput() {
	state := g.state
	futureTick := state.Tick + InputDelay
	input := keyboardAxis(ebiten.KeyArrowUp, ebiten.KeyArrowDown)

	state.PushCommand(futureTick, state.Player, input)

	if g.netplay != nil && g.netplay.IsReady() {
		g.netplay.SendLocalInput(futureTick, input)
		g.recordReplayInput(futureTick, input)
	}
}

func (g *GameScene) simulateCurrentTick() {
	state := g.state
	currentTick := state.Tick
	frame, ok := state.CommandQueue[currentTick]

	if !ok || !frame.P1Ready || !frame.P2Ready {
		return
	}

	cmd := Input{
		Player1: frame.P1Input,
		Player2: frame.P2Input,
	}
	state.Step(cmd)

	delete(state.CommandQueue, currentTick)
}

type GameOverReport struct {
	Winner       int32
	LeftScore    byte
	RightScore   byte
	GameOverTick uint32
}

func (g *GameScene) SendGameOverPacket() {
	report := GameOverReport{
		Winner:       int32(g.state.Winner),
		LeftScore:    byte(g.state.LeftScore),
		RightScore:   byte(g.state.RightScore),
		GameOverTick: g.state.GameOverTick,
	}

	payload := make([]byte, 10)
	binary.BigEndian.PutUint32(payload[0:4], uint32(report.Winner))
	payload[4] = report.LeftScore
	payload[5] = report.RightScore
	binary.BigEndian.PutUint32(payload[6:10], report.GameOverTick)

	if err := g.sendPacket(network.MakePacket(1, byte(2), payload)); err != nil {
		log.Printf("failed to send game over packet: %v", err)
	}
}

func (g *GameScene) finishGame() {
	g.Flush()
	g.SendGameOverPacket()

	// UDP 연결이 열려 있으면 닫기
	if g.transport != nil {
		_ = g.transport.Close()
	}

	g.OnChangeScene(0, nil)
}

func (g *GameScene) Flush() {
	if len(g.recorder) == 0 {
		return
	}

	g.sendPacket(network.MakePacket(1, byte(3), serializeReplay(g.recorder)))

	g.recorder = g.recorder[:0]
}

func serializeReplay(frames []ReplayFrame) []byte {
	payload := make([]byte, 2+len(frames)*5)
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(frames)))

	offset := 2
	for _, frame := range frames {
		binary.BigEndian.PutUint32(payload[offset:offset+4], frame.Tick)
		offset += 4

		payload[offset] = byte(int8(frame.Input))
		offset++
	}

	return payload
}

func (g *GameScene) recordReplayInput(tick uint32, input Axis) {
	g.recorder = append(g.recorder, ReplayFrame{
		Tick:  tick,
		Input: input,
	})

	if len(g.recorder) >= 60 {
		g.Flush()
	}
}
func drawState(screen *ebiten.Image, state *State) {
	screen.Fill(color.RGBA{18, 18, 22, 255})

	for y := 0; y < ScreenHeight; y += 24 {
		vector.FillRect(screen, ScreenWidth/2-1, float32(y), 2, 12, color.RGBA{90, 90, 100, 255}, false)
	}

	leftPaddleColor := color.RGBA{255, 255, 255, 255}
	rightPaddleColor := color.RGBA{255, 255, 255, 255}
	if state.Player == Player1 {
		leftPaddleColor = color.RGBA{80, 220, 120, 255}
	} else if state.Player == Player2 {
		rightPaddleColor = color.RGBA{80, 220, 120, 255}
	}

	vector.FillRect(screen, float32(state.LeftPaddle.X), float32(state.LeftPaddle.Y), PaddleWidth, PaddleHeight, leftPaddleColor, false)
	vector.FillRect(screen, float32(state.RightPaddle.X), float32(state.RightPaddle.Y), PaddleWidth, PaddleHeight, rightPaddleColor, false)
	vector.FillRect(screen, float32(state.Ball.X), float32(state.Ball.Y), BallSize, BallSize, color.White, false)

	score := fmt.Sprintf("%d          %d", state.LeftScore, state.RightScore)
	ebitenutil.DebugPrintAt(screen, score, ScreenWidth/2-42, 24)
}
