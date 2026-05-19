package game

import "log"

const (
	ScreenWidth  = 640
	ScreenHeight = 480

	PaddleWidth  = 10
	PaddleHeight = 72
	PaddleSpeed  = 6
	PaddleMargin = 28

	BallSize = 10
	BallDX   = 5
	BallDY   = 3
)

type Axis int

const (
	AxisNeutral Axis = 0
	AxisUp      Axis = -1
	AxisDown    Axis = 1

	Player1 uint8 = 1
	Player2 uint8 = 2
)

type Input struct {
	Player1 Axis
	Player2 Axis
}

type Rect struct {
	X int
	Y int
}

type Ball struct {
	X  int
	Y  int
	VX int
	VY int
}

type State struct {
	LeftPaddle  Rect
	RightPaddle Rect
	Ball        Ball
	LeftScore   int
	RightScore  int
	Tick        uint32

	CommandQueue map[uint32]map[uint8]Axis
	Player       uint8
}

func NewState(player uint8) *State {
	s := &State{
		LeftPaddle: Rect{
			X: PaddleMargin,
			Y: (ScreenHeight - PaddleHeight) / 2,
		},
		RightPaddle: Rect{
			X: ScreenWidth - PaddleMargin - PaddleWidth,
			Y: (ScreenHeight - PaddleHeight) / 2,
		},
		Player:       player,
		CommandQueue: make(map[uint32]map[uint8]Axis),
	}

	for tick := uint32(0); tick < InputDelay; tick++ {
		s.CommandQueue[tick] = map[uint8]Axis{
			Player1: AxisNeutral,
			Player2: AxisNeutral,
		}
	}

	log.Printf("Initial CommandQueue: %v", s.CommandQueue)

	s.resetBall(1)
	return s
}

func (s *State) Step(input Input) {
	s.Tick++

	s.movePaddle(&s.LeftPaddle, input.Player1)
	s.movePaddle(&s.RightPaddle, input.Player2)
	s.moveBall()
}

func (s *State) movePaddle(paddle *Rect, axis Axis) {
	paddle.Y += int(axis) * PaddleSpeed
	paddle.Y = clamp(paddle.Y, 0, ScreenHeight-PaddleHeight)
}

func (s *State) moveBall() {
	s.Ball.X += s.Ball.VX
	s.Ball.Y += s.Ball.VY

	if s.Ball.Y <= 0 {
		s.Ball.Y = 0
		s.Ball.VY = abs(s.Ball.VY)
	} else if s.Ball.Y+BallSize >= ScreenHeight {
		s.Ball.Y = ScreenHeight - BallSize
		s.Ball.VY = -abs(s.Ball.VY)
	}

	if s.Ball.VX < 0 && overlapsPaddle(s.Ball, s.LeftPaddle) {
		s.Ball.X = s.LeftPaddle.X + PaddleWidth
		s.Ball.VX = abs(s.Ball.VX)
		s.Ball.VY = bounceVelocity(s.Ball, s.LeftPaddle)
	} else if s.Ball.VX > 0 && overlapsPaddle(s.Ball, s.RightPaddle) {
		s.Ball.X = s.RightPaddle.X - BallSize
		s.Ball.VX = -abs(s.Ball.VX)
		s.Ball.VY = bounceVelocity(s.Ball, s.RightPaddle)
	}

	if s.Ball.X+BallSize < 0 {
		s.RightScore++
		s.resetBall(-1)
	} else if s.Ball.X > ScreenWidth {
		s.LeftScore++
		s.resetBall(1)
	}
}

func (s *State) resetBall(direction int) {
	s.Ball = Ball{
		X:  (ScreenWidth - BallSize) / 2,
		Y:  (ScreenHeight - BallSize) / 2,
		VX: direction * BallDX,
		VY: BallDY,
	}
}

func overlapsPaddle(ball Ball, paddle Rect) bool {
	return ball.X < paddle.X+PaddleWidth &&
		ball.X+BallSize > paddle.X &&
		ball.Y < paddle.Y+PaddleHeight &&
		ball.Y+BallSize > paddle.Y
}

func bounceVelocity(ball Ball, paddle Rect) int {
	ballCenter := ball.Y + BallSize/2
	paddleCenter := paddle.Y + PaddleHeight/2
	offset := ballCenter - paddleCenter
	velocity := offset / 8

	if velocity == 0 {
		if offset < 0 {
			return -1
		}
		return 1
	}

	return clamp(velocity, -6, 6)
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
