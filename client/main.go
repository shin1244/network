package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480

	Unit = 1000 // 락스텝 좌표계에서 1픽셀을 1000단위로 표현

	PlayerSpeed  = 4 * Unit
	PlayerRadius = 10.0
	GridSize     = 10

	InputDelay = 5
)

type Command struct {
	PlayerIdx int
	ExecTick  int
	Action    int
	DestX     int
	DestY     int
	Seq       int
}

// 부동소수점 대신 정수 좌표계를 사용한 플레이어 구조체
type Player struct {
	X, Y         int
	DestX, DestY int
	Color        color.Color
}

type Game struct {
	Player1 *Player
	Player2 *Player

	PlayerIdx int

	// 네트워크 관련
	conn     *net.UDPConn
	peerAddr *net.UDPAddr
	recvCh   chan Command // 수신된 패킷을 게임 루프로 넘기는 채널

	// 락스텝 관련
	CurrentTick int

	PendingMap   map[int]Command
	PendingMutex sync.Mutex

	CommandQueue map[int]map[int]Command
	Seq          int
	ackChan      chan Command
}

func (g *Game) Update() error {
Loop:
	for {
		select {
		case cmd := <-g.recvCh:
			if g.CommandQueue[cmd.ExecTick] == nil {
				g.CommandQueue[cmd.ExecTick] = make(map[int]Command)
			}
			g.CommandQueue[cmd.ExecTick][cmd.PlayerIdx] = cmd
		default:
			break Loop
		}
	}

	targetTick := g.CurrentTick + InputDelay

	cmd := Command{
		PlayerIdx: int(g.PlayerIdx),
		ExecTick:  targetTick,
		Action:    0,
		Seq:       g.Seq,
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		mx, my := ebiten.CursorPosition()
		cmd.Action = 1
		cmd.DestX = mx * Unit
		cmd.DestY = my * Unit
	}

	if _, ok := g.CommandQueue[targetTick]; !ok {
		g.CommandQueue[targetTick] = make(map[int]Command)
	}
	g.CommandQueue[targetTick][g.PlayerIdx] = cmd

	sendData, _ := json.Marshal(cmd)
	g.conn.WriteToUDP(sendData, g.peerAddr)

	g.PendingMutex.Lock()
	g.PendingMap[g.Seq] = cmd
	g.PendingMutex.Unlock()
	g.Seq++

	cmds, ok := g.CommandQueue[g.CurrentTick]
	if !ok || len(cmds) < 2 {
		return nil
	}

	p1Cmd := cmds[1]
	if p1Cmd.Action == 1 {
		g.Player1.DestX = p1Cmd.DestX
		g.Player1.DestY = p1Cmd.DestY
	}

	// Player 2 명령 실행
	p2Cmd := cmds[2]
	if p2Cmd.Action == 1 {
		g.Player2.DestX = p2Cmd.DestX
		g.Player2.DestY = p2Cmd.DestY
	}

	// 실행 완료된 틱 삭제 (메모리 정리)
	delete(g.CommandQueue, g.CurrentTick)

	movePlayer(g.Player1)
	movePlayer(g.Player2)

	g.CurrentTick++

	return nil
}

// movePlayer: 정수 연산만 사용해서 이동
func movePlayer(p *Player) {
	// 거리 차이 (정수)
	dx := p.DestX - p.X
	dy := p.DestY - p.Y

	// 거리 계산 (피타고라스)
	// 제곱하면 숫자가 엄청 커지므로 int64나 float 변환을 잠깐 써야 합니다.
	// (엄격한 락스텝에선 정수 sqrt 함수를 따로 구현하지만, 여기선 편의상 math.Sqrt 사용 후 다시 int로 만듭니다)
	distFloat := math.Sqrt(float64(dx*dx + dy*dy))
	dist := int(distFloat)

	// 도착 판정 (오차 범위 내)
	if dist < PlayerSpeed {
		p.X = p.DestX
		p.Y = p.DestY
		return
	}

	// 이동 (비례식 적용)
	// 공식: X += dx * (속도 / 거리)
	// 주의: 정수 나눗셈은 소수점이 버려지므로 곱하기를 먼저 해야 함!

	if dist > 0 { // 0으로 나누기 방지
		p.X += (dx * PlayerSpeed) / dist
		p.Y += (dy * PlayerSpeed) / dist
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	drawGrid(screen)

	// ★ 그리기 단계: 여기서만 1000으로 나눔
	drawX1 := float32(g.Player1.X) / Unit
	drawY1 := float32(g.Player1.Y) / Unit
	vector.FillCircle(screen, drawX1, drawY1, PlayerRadius, g.Player1.Color, true)

	drawX2 := float32(g.Player2.X) / Unit
	drawY2 := float32(g.Player2.Y) / Unit
	vector.FillCircle(screen, drawX2, drawY2, PlayerRadius, g.Player2.Color, true)
}

func (g *Game) Layout(w, h int) (int, int) {
	return ScreenWidth, ScreenHeight
}

func main() {
	playerIdx, peerAddr, conn := MatchAndPunch()

	game := &Game{
		Player1: &Player{
			X: 100 * Unit, Y: 240 * Unit,
			DestX: 100 * Unit, DestY: 240 * Unit,
			Color: color.RGBA{0, 0, 255, 255},
		},
		Player2: &Player{
			X: 540 * Unit, Y: 240 * Unit,
			DestX: 540 * Unit, DestY: 240 * Unit,
			Color: color.RGBA{255, 0, 0, 255},
		},
		PlayerIdx:    playerIdx,
		peerAddr:     peerAddr,
		conn:         conn,
		recvCh:       make(chan Command, 100),
		CommandQueue: make(map[int]map[int]Command),
		CurrentTick:  0,
	}

	go game.ListenAndDispatch()
	go game.ProcessACKs()
	go game.RetransmitPendingPackets()

	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("Network test -> hole punching + lockstep")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func drawGrid(screen *ebiten.Image) {
	// 어두운 회색 배경
	screen.Fill(color.RGBA{20, 20, 20, 255})

	// 눈금 색
	gridColor := color.RGBA{60, 60, 60, 255}
	axisColor := color.RGBA{120, 120, 120, 255}

	// 세로선
	for x := 0; x <= ScreenWidth; x += GridSize {
		c := gridColor
		if x == ScreenWidth/2 {
			c = axisColor
		}
		vector.StrokeLine(
			screen,
			float32(x), 0,
			float32(x), ScreenHeight,
			1,
			c,
			true,
		)
	}

	// 가로선
	for y := 0; y <= ScreenHeight; y += GridSize {
		c := gridColor
		if y == ScreenHeight/2 {
			c = axisColor
		}
		vector.StrokeLine(
			screen,
			0, float32(y),
			ScreenWidth, float32(y),
			1,
			c,
			true,
		)
	}
}

func MatchAndPunch() (int, *net.UDPAddr, *net.UDPConn) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		panic(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}

	serverAddr, _ := net.ResolveUDPAddr("udp", "210.57.239.71:45678")
	conn.WriteToUDP([]byte("new"), serverAddr)

	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		panic(err)
	}

	playerIdx := int(buffer[0])
	peerInfo := strings.TrimSpace(string(buffer[1:n]))

	peerAddr, err := net.ResolveUDPAddr("udp", peerInfo)
	if err != nil {
		panic(err)
	}

	fmt.Println("--------------------------------")
	fmt.Printf("매칭 성공! 상대방 주소: %s\n", peerAddr.String())
	fmt.Println("--------------------------------")

	for i := 0; i < 3; i++ {
		conn.WriteToUDP([]byte("punch"), peerAddr)
	}

	// conn을 반환해서 게임에서 계속 쓰게 함
	return playerIdx, peerAddr, conn
}

func (g *Game) ListenAndDispatch() {
	buffer := make([]byte, 1024)
	for {
		n, _, err := g.conn.ReadFromUDP(buffer)
		if err != nil {
			log.Println("패킷 수신 오류:", err)
			continue
		}

		var cmd Command
		err = json.Unmarshal(buffer[:n], &cmd)
		if err != nil {
			log.Println("패킷 파싱 오류:", err)
			continue
		}

		// ACK 수신
		if cmd.Action == 8 {
			g.ackChan <- cmd
			continue
		}

		// 일반 명령 수신
		g.recvCh <- cmd

		// 3. 잘 받았다고 답장(ACK) 발송
		ackCmd := Command{
			PlayerIdx: int(g.PlayerIdx),
			Action:    8,       // 8 = ACK Type
			Seq:       cmd.Seq, // 받은 번호 그대로 돌려줌
		}
		ackBytes, _ := json.Marshal(ackCmd)
		g.conn.WriteToUDP(ackBytes, g.peerAddr)
	}
}

func (g *Game) ProcessACKs() {
	for ack := range g.ackChan {
		g.PendingMutex.Lock()
		fmt.Printf("✅ ACK 수신확인: Seq=%d (보관함 삭제)\n", ack.Seq)
		delete(g.PendingMap, ack.Seq)
		g.PendingMutex.Unlock()
	}
}

func (g *Game) RetransmitPendingPackets() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		g.PendingMutex.Lock()
		for seq, cmd := range g.PendingMap {
			sendData, _ := json.Marshal(cmd)
			g.conn.WriteToUDP(sendData, g.peerAddr)
			fmt.Printf("🔄 재전송 수행: Seq=%d\n", seq)
		}
		g.PendingMutex.Unlock()
	}
}
