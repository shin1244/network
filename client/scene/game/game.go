package game

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"log"
	"net"
	"pong/network"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	InputDelay = 3

	UDPRegister byte = 2
	UDPPunch    byte = 3
	UDPAck      byte = 4
)

type GameScene struct {
	state    *State
	clientID int32

	UDPConn *net.UDPConn
	UDPAddr *net.UDPAddr

	countdownStartedAt time.Time

	sendPacket    func([]byte) error
	OnChangeScene func(sceneID int, data []byte)

	read chan []byte
}

func NewGameScene(sendPacket func([]byte) error, onChangeScene func(sceneID int, data []byte), clientID int32, data []byte) *GameScene {
	player := Player1
	if len(data) >= 4 {
		player = uint8(binary.BigEndian.Uint32(data))
	}

	GameScene := &GameScene{
		state:         NewState(player),
		clientID:      clientID,
		sendPacket:    sendPacket,
		OnChangeScene: onChangeScene,
		read:          make(chan []byte, 100),
	}

	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		log.Printf("failed to resolve UDP address: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("failed to listen on UDP: %v", err)
	}
	GameScene.UDPConn = conn
	payload := binary.BigEndian.AppendUint32(nil, uint32(clientID))
	serverAddr, err := net.ResolveUDPAddr("udp", "localhost:5555")
	if err != nil {
		log.Printf("failed to resolve server UDP address: %v", err)
	} else if _, err := GameScene.UDPConn.WriteToUDP(network.MakePacket(1, UDPRegister, payload), serverAddr); err != nil {
		log.Printf("failed to register udp addr: %v", err)
	}
	go GameScene.readUDPLoop()

	return GameScene
}

func (g *GameScene) Update() error {
	for {
		select {
		case data := <-g.read:
			if g.UDPAddr != nil && len(data) >= 5 {
				ack := network.MakePacket(
					1,
					UDPAck,
					data[0:4],
				)

				_, _ = g.UDPConn.WriteToUDP(
					ack,
					g.UDPAddr,
				)

				execTick := binary.BigEndian.Uint32(
					data[0:4],
				)

				execInput := Axis(
					int8(data[4]),
				)

				if g.state.CommandQueue[execTick] == nil {
					g.state.CommandQueue[execTick] = make(map[uint8]Axis)
				}

				g.state.CommandQueue[execTick][g.remotePlayerCommand()] = execInput
			}

		default:
			goto END
		}
	}

END:

	if !g.isReadyToStart() {
		return nil
	}

	state := g.state
	futureTick := g.state.Tick + InputDelay

	input := keyboardAxis(ebiten.KeyArrowUp, ebiten.KeyArrowDown)

	data := make([]byte, 5)

	binary.BigEndian.PutUint32(
		data[0:4],
		futureTick,
	)

	data[4] = byte(int8(input))

	if state.CommandQueue[futureTick] == nil {
		state.CommandQueue[futureTick] = make(map[uint8]Axis)
	}
	state.CommandQueue[futureTick][state.Player] = input

	if g.UDPAddr != nil {
		packet := network.MakePacket(1, state.Player, data)
		if _, err := g.UDPConn.WriteToUDP(packet, g.UDPAddr); err != nil {
			log.Printf("failed to send input packet to %v: %v", g.UDPAddr, err)
		}
	}

	currentTick := state.Tick
	cmds, ok := state.CommandQueue[currentTick]
	if !ok || len(cmds) < 2 { // 내 명령과 상대 명령이 모두 도착해야 게임 상태를 업데이트할 수 있음
		return nil
	}

	cmd := Input{
		Player1: cmds[1],
		Player2: cmds[2],
	}
	state.Step(cmd)

	// Step을 실행한 후에 state.Tick를 바로 삭제하면
	// 다음 업데이트에서 state.Tick+InputDelay에 명령이 도착했을 때,
	// Step에서 참조하는 state.CommandQueue[currentTick]가 삭제되어 nil이 되는 문제가 발생함
	delete(state.CommandQueue, currentTick)
	return nil
}

func (g *GameScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{18, 18, 22, 255})

	for y := 0; y < ScreenHeight; y += 24 {
		vector.FillRect(screen, ScreenWidth/2-1, float32(y), 2, 12, color.RGBA{90, 90, 100, 255}, false)
	}

	var leftColor color.Color = color.White
	var rightColor color.Color = color.White
	selfColor := color.RGBA{80, 220, 120, 255} // 플레이어 패들 색상(초록색)
	switch g.state.Player {
	case Player1:
		leftColor = selfColor
	case Player2:
		rightColor = selfColor
	}

	vector.FillRect(screen, float32(g.state.LeftPaddle.X), float32(g.state.LeftPaddle.Y), PaddleWidth, PaddleHeight, leftColor, false)
	vector.FillRect(screen, float32(g.state.RightPaddle.X), float32(g.state.RightPaddle.Y), PaddleWidth, PaddleHeight, rightColor, false)
	vector.FillRect(screen, float32(g.state.Ball.X), float32(g.state.Ball.Y), BallSize, BallSize, color.White, false)

	score := fmt.Sprintf("%d          %d", g.state.LeftScore, g.state.RightScore)
	ebitenutil.DebugPrintAt(screen, score, ScreenWidth/2-42, 24)

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
	scene := event.Scene
	cmd := event.SceneCommand

	if scene != byte(1) {
		return
	}

	switch cmd {
	case byte(0): // 접속
		go g.tryHolePunching(event.Data)
	case byte(1): // 상대 정보 업데이트

	}
}

func (g *GameScene) tryHolePunching(data []byte) {
	peerAddr, err := DeserializeUDPAddr(data)
	if err != nil {
		log.Printf("failed to deserialize peer addr: %v", err)
		return
	}

	log.Printf("start hole punching to %v", peerAddr)

	packet := network.MakePacket(1, UDPPunch, []byte("punch"))

	for i := 0; i < 10; i++ {
		if _, err := g.UDPConn.WriteToUDP(packet, peerAddr); err != nil {
			log.Printf("failed to send punch packet to %v: %v", peerAddr, err)
			return
		}

		time.Sleep(100 * time.Millisecond)
	}
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

func (g *GameScene) readUDPLoop() {
	buf := make([]byte, 1024)

	for {
		n, addr, err := g.UDPConn.ReadFromUDP(buf)
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
		command := msg[1]
		payloadLen := int(binary.BigEndian.Uint32(msg[2:6]))

		if len(msg) < 6+payloadLen {
			log.Printf("invalid udp packet from %v: payload length mismatch", addr)
			continue
		}

		payload := msg[6 : 6+payloadLen]

		if scene != 1 {
			continue
		}

		switch command {
		case UDPPunch:
			log.Printf("received punch from %v", addr)

			ack := network.MakePacket(1, UDPAck, nil)
			if _, err := g.UDPConn.WriteToUDP(ack, addr); err != nil {
				log.Printf("failed to send ack to %v: %v", addr, err)
			}

		case UDPAck:
			if len(payload) == 0 {
				log.Printf("received ack from %v", addr)
				g.UDPAddr = addr
			}

		case g.remotePlayerCommand():
			g.read <- append([]byte(nil), payload...)
		}
	}
}

func (g *GameScene) remotePlayerCommand() byte {
	if g.state.Player == Player1 {
		return Player2
	}

	return Player1
}

func (g *GameScene) isReadyToStart() bool {
	if g.UDPAddr == nil {
		return false
	}

	if g.countdownStartedAt.IsZero() {
		g.countdownStartedAt = time.Now()
		return false
	}

	return time.Since(g.countdownStartedAt) >= 3*time.Second
}

func (g *GameScene) drawStartStatus(screen *ebiten.Image) {
	if g.UDPAddr == nil {
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
