package game

// 게임 시뮬레이션 코어는 클라이언트와 서버가 공유하는 pongsim 모듈로 추출되었습니다.
// 이 파일은 기존 game 패키지 API(game.State, game.NewState 등)를 유지하기 위해
// 공유 모듈의 타입·상수를 타입 별칭으로 재노출하는 얇은 shim 입니다.
// 실제 로직은 sim 패키지 한 곳에만 존재하므로, 클라/서버 시뮬레이션이 구조적으로 항상 동일합니다.

import "pongsim"

type (
	Axis       = sim.Axis
	Input      = sim.Input
	Rect       = sim.Rect
	Ball       = sim.Ball
	State      = sim.State
	FrameInput = sim.FrameInput
)

const (
	ScreenWidth  = sim.ScreenWidth
	ScreenHeight = sim.ScreenHeight

	PaddleWidth  = sim.PaddleWidth
	PaddleHeight = sim.PaddleHeight
	PaddleSpeed  = sim.PaddleSpeed
	PaddleMargin = sim.PaddleMargin

	BallSize = sim.BallSize
	BallDX   = sim.BallDX
	BallDY   = sim.BallDY

	WinningScore = sim.WinningScore

	InputDelay = sim.InputDelay

	AxisNeutral = sim.AxisNeutral
	AxisUp      = sim.AxisUp
	AxisDown    = sim.AxisDown

	Player1 = sim.Player1
	Player2 = sim.Player2
)

func NewState(player uint8) *State {
	return sim.NewState(player)
}
