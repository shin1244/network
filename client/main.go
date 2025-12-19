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
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480
	Unit         = 1000 // 고정 소수점 단위

	PlayerSpeed   = 4 * Unit
	PlayerRadius  = 12.0 // 피격 판정 조금 넉넉하게
	RotationSpeed = 2    // 1틱당 2도 회전

	BulletSpeed  = 8 * Unit // 총알 속도
	BulletRadius = 6.0

	InputDelay = 5
)

const (
	ActionIdle  = 0
	ActionMove  = 1 << 0
	ActionShoot = 1 << 1
)

// 총알 (주인 없음, 네트워크 동기화 안 함, 오직 로직으로만 존재)
type Bullet struct {
	X, Y   int
	DX, DY int
}

type Command struct {
	PlayerIdx int
	ExecTick  int
	Action    int // 0:Idle, 1:Move, 2:Shoot
	DestX     int
	DestY     int
	Seq       int
}

type Player struct {
	X, Y         int
	DestX, DestY int
	Angle        int // 현재 바라보는 각도
	Color        color.Color
	IsDead       bool // 사망 여부

	LastShootTick int // 마지막 발사 틱 기록
}

type Game struct {
	Player1   *Player
	Player2   *Player
	PlayerIdx int

	// 총알 관리 (네트워크 전송 X, 로컬 시뮬레이션 O)
	Bullets []Bullet

	// 네트워크
	conn     *net.UDPConn
	peerAddr *net.UDPAddr
	recvCh   chan Command

	// 락스텝 & Reliable
	CurrentTick  int
	PendingMap   map[int]Command
	PendingMutex sync.Mutex
	CommandQueue map[int]map[int]Command
	Seq          int
	ackChan      chan Command

	IsGameOver bool
}

func (g *Game) Update() error {
	if g.IsGameOver {
		return nil
	}

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

	// 2. 입력 처리 및 예약
	targetTick := g.CurrentTick + InputDelay
	g.Seq++

	cmd := Command{
		PlayerIdx: int(g.PlayerIdx),
		ExecTick:  targetTick,
		Action:    ActionIdle,
		Seq:       g.Seq,
	}

	// 플레이어 선택
	p := g.Player1
	if g.PlayerIdx == 2 {
		p = g.Player2
	}

	// 이동 입력
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		mx, my := ebiten.CursorPosition()
		cmd.Action |= ActionMove
		cmd.DestX = mx * Unit
		cmd.DestY = my * Unit
	}

	// 발사 쿨타임 1초
	shootCooldownTicks := 60
	if ebiten.IsKeyPressed(ebiten.KeySpace) && (g.CurrentTick-p.LastShootTick >= shootCooldownTicks) {
		cmd.Action |= ActionShoot
		p.LastShootTick = g.CurrentTick
	}

	// 큐에 저장
	if g.CommandQueue[targetTick] == nil {
		g.CommandQueue[targetTick] = make(map[int]Command)
	}
	g.CommandQueue[targetTick][g.PlayerIdx] = cmd

	// 전송 및 보관
	sendData, _ := json.Marshal(cmd)
	g.conn.WriteToUDP(sendData, g.peerAddr)

	g.PendingMutex.Lock()
	g.PendingMap[cmd.Seq] = cmd
	g.PendingMutex.Unlock()

	// 3. 락스텝 실행 조건 확인
	cmds, ok := g.CommandQueue[g.CurrentTick]
	if !ok || len(cmds) < 2 {
		return nil // 대기
	}

	// -----------------------------------------------------------
	// ★ 결정론적 시뮬레이션 시작
	// -----------------------------------------------------------

	// (1) 플레이어 회전
	g.Player1.Angle = (g.Player1.Angle + RotationSpeed) % 360
	g.Player2.Angle = (g.Player2.Angle + RotationSpeed) % 360

	// (2) 명령 실행
	g.ExecuteCommand(g.Player1, cmds[1])
	g.ExecuteCommand(g.Player2, cmds[2])

	// (3) 물리 엔진 업데이트
	g.UpdatePhysics()

	// 메모리 정리 및 틱 증가
	delete(g.CommandQueue, g.CurrentTick)
	g.CurrentTick++

	return nil
}

func (g *Game) ExecuteCommand(p *Player, cmd Command) {
	if cmd.Action&ActionMove != 0 {
		p.DestX = cmd.DestX
		p.DestY = cmd.DestY
	}
	if cmd.Action&ActionShoot != 0 {
		rad := float64(p.Angle) * math.Pi / 180.0
		dx := int(math.Cos(rad) * float64(BulletSpeed))
		dy := int(math.Sin(rad) * float64(BulletSpeed))

		// ★ 총알 초기 위치: 플레이어 앞쪽
		startX := p.X + int(math.Cos(rad)*float64(PlayerRadius+BulletRadius)*Unit/1000)
		startY := p.Y + int(math.Sin(rad)*float64(PlayerRadius+BulletRadius)*Unit/1000)

		newBullet := Bullet{
			X: startX, Y: startY,
			DX: dx, DY: dy,
		}
		g.Bullets = append(g.Bullets, newBullet)
	}
}

// 물리 엔진: 총알 이동, 튕기기, 충돌 판정
func (g *Game) UpdatePhysics() {
	// 1. 플레이어 이동
	movePlayer(g.Player1)
	movePlayer(g.Player2)

	// 2. 총알 처리
	// 화면 경계 (x1000 단위)
	limitX := ScreenWidth * Unit
	limitY := ScreenHeight * Unit

	for i := range g.Bullets {
		b := &g.Bullets[i]

		// 이동
		b.X += b.DX
		b.Y += b.DY

		// 벽 튕기기 (좌우)
		if b.X <= 0 {
			b.X = 0      // 끼임 방지
			b.DX = -b.DX // 반사
		} else if b.X >= limitX {
			b.X = limitX
			b.DX = -b.DX
		}

		// 벽 튕기기 (상하)
		if b.Y <= 0 {
			b.Y = 0
			b.DY = -b.DY
		} else if b.Y >= limitY {
			b.Y = limitY
			b.DY = -b.DY
		}

		// 충돌 체크 (총알 -> 플레이어 1)
		if checkCollision(b, g.Player1) {
			g.Player1.IsDead = true
			g.IsGameOver = true
		}
		// 충돌 체크 (총알 -> 플레이어 2)
		if checkCollision(b, g.Player2) {
			g.Player2.IsDead = true
			g.IsGameOver = true
		}
	}
}

// 충돌 감지 (거리 계산)
func checkCollision(b *Bullet, p *Player) bool {
	dx := b.X - p.X
	dy := b.Y - p.Y
	// 제곱 거리로 비교 (sqrt 성능 최적화)
	distSq := int64(dx)*int64(dx) + int64(dy)*int64(dy)

	// 판정 반지름 합
	radiusSum := int64((PlayerRadius + BulletRadius) * Unit) // x1000 된 상태
	// Unit이 제곱되면 너무 커지므로, 비교할 때 주의 (여기선 반지름이 작아서 괜찮음)

	// 간단하게: 실제 거리(픽셀)로 변환해서 비교
	// (x1000 상태의 거리) < (반지름 합 * 1000)
	// 안전하게 float 변환 후 비교 (정확도보다 오버플로우 방지)
	dist := math.Sqrt(float64(distSq))

	return dist < float64(radiusSum)
}

func movePlayer(p *Player) {
	dx := p.DestX - p.X
	dy := p.DestY - p.Y
	dist := int(math.Sqrt(float64(dx*dx + dy*dy)))

	if dist < PlayerSpeed {
		p.X, p.Y = p.DestX, p.DestY
		return
	}
	if dist > 0 {
		p.X += (dx * PlayerSpeed) / dist
		p.Y += (dy * PlayerSpeed) / dist
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	drawGrid(screen)

	// 총알 그리기 (흰색 작은 점)
	for _, b := range g.Bullets {
		vector.FillCircle(screen, float32(b.X)/Unit, float32(b.Y)/Unit, BulletRadius, color.White, true)
	}

	// 플레이어 그리기
	drawPlayer(screen, g.Player1)
	drawPlayer(screen, g.Player2)

	// 게임 오버 메시지
	if g.IsGameOver {
		msg := "GAME OVER"
		if g.Player1.IsDead && g.Player2.IsDead {
			msg += "\nDraw!"
		} else if g.Player1.IsDead {
			msg += "\nPlayer 2 Wins!"
		} else {
			msg += "\nPlayer 1 Wins!"
		}
		ebitenutil.DebugPrintAt(screen, msg, ScreenWidth/2-40, ScreenHeight/2)
	} else {
		ebitenutil.DebugPrint(screen, fmt.Sprintf("Tick: %d\nBullets: %d", g.CurrentTick, len(g.Bullets)))
	}
}

func drawPlayer(screen *ebiten.Image, p *Player) {
	if p.IsDead {
		return
	} // 죽으면 안 그림

	px, py := float32(p.X)/Unit, float32(p.Y)/Unit
	vector.FillCircle(screen, px, py, PlayerRadius, p.Color, true)

	// 회전 방향 표시 (눈)
	rad := float64(p.Angle) * math.Pi / 180.0
	ex := px + float32(math.Cos(rad)*20)
	ey := py + float32(math.Sin(rad)*20)
	vector.StrokeLine(screen, px, py, ex, ey, 2, color.White, true)
}

func (g *Game) ListenAndDispatch() {
	buffer := make([]byte, 1024)
	for {
		n, _, err := g.conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}
		var cmd Command
		if err := json.Unmarshal(buffer[:n], &cmd); err != nil {
			continue
		}

		if cmd.Action == 8 {
			g.ackChan <- cmd
			continue
		}
		g.recvCh <- cmd

		// ACK 답장
		ack := Command{PlayerIdx: g.PlayerIdx, Action: 8, Seq: cmd.Seq}
		data, _ := json.Marshal(ack)
		g.conn.WriteToUDP(data, g.peerAddr)
	}
}

func (g *Game) ProcessACKs() {
	for ack := range g.ackChan {
		g.PendingMutex.Lock()
		delete(g.PendingMap, ack.Seq)
		g.PendingMutex.Unlock()
	}
}

func (g *Game) RetransmitPendingPackets() {
	ticker := time.NewTicker(100 * time.Millisecond)
	for range ticker.C {
		g.PendingMutex.Lock()
		for _, cmd := range g.PendingMap {
			data, _ := json.Marshal(cmd)
			g.conn.WriteToUDP(data, g.peerAddr)
		}
		g.PendingMutex.Unlock()
	}
}

func main() {
	playerIdx, peerAddr, conn := MatchAndPunch()

	game := &Game{
		Player1:      &Player{X: 100 * Unit, Y: 240 * Unit, DestX: 100 * Unit, DestY: 240 * Unit, Color: color.RGBA{0, 0, 255, 255}},
		Player2:      &Player{X: 540 * Unit, Y: 240 * Unit, DestX: 540 * Unit, DestY: 240 * Unit, Color: color.RGBA{255, 0, 0, 255}},
		PlayerIdx:    playerIdx,
		peerAddr:     peerAddr,
		conn:         conn,
		recvCh:       make(chan Command, 100),
		CommandQueue: make(map[int]map[int]Command),
		PendingMap:   make(map[int]Command),
		ackChan:      make(chan Command, 100),
		Seq:          1,
	}

	// ★ 중요: 0~4 틱(Delay) 채우기
	for i := 0; i < InputDelay; i++ {
		game.CommandQueue[i] = make(map[int]Command)
		game.CommandQueue[i][1] = Command{PlayerIdx: 1, ExecTick: i, Action: 0}
		game.CommandQueue[i][2] = Command{PlayerIdx: 2, ExecTick: i, Action: 0}
	}

	go game.ListenAndDispatch()
	go game.ProcessACKs()
	go game.RetransmitPendingPackets()

	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("P2P")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

// ... drawGrid, MatchAndPunch, Layout 등은 이전과 동일하므로 생략하지 않고 그대로 쓰시면 됩니다.
func (g *Game) Layout(w, h int) (int, int) { return ScreenWidth, ScreenHeight }
func drawGrid(screen *ebiten.Image)        { /* 이전 코드 복붙 */ }
func MatchAndPunch() (int, *net.UDPAddr, *net.UDPConn) {
	// 이전 코드와 동일 (테스트용)
	addr, _ := net.ResolveUDPAddr("udp", ":0")
	conn, _ := net.ListenUDP("udp", addr)
	serverAddr, _ := net.ResolveUDPAddr("udp", "210.57.239.71:45678")
	conn.WriteToUDP([]byte("new"), serverAddr)
	buf := make([]byte, 1024)
	n, _, _ := conn.ReadFromUDP(buf)
	pIdx := int(buf[0])
	pInfo := strings.TrimSpace(string(buf[1:n]))
	pAddr, _ := net.ResolveUDPAddr("udp", pInfo)
	fmt.Printf("Matched: %s\n", pAddr.String())
	for i := 0; i < 10; i++ {
		conn.WriteToUDP([]byte("punch"), pAddr)
		time.Sleep(10 * time.Millisecond)
	}
	return pIdx, pAddr, conn
}
