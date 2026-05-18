package game

import "testing"

func TestStateStepIsDeterministic(t *testing.T) {
	first := NewState()
	second := NewState()
	inputs := []Input{
		{Left: AxisUp, Right: AxisDown},
		{Left: AxisUp, Right: AxisNeutral},
		{Left: AxisNeutral, Right: AxisDown},
		{Left: AxisDown, Right: AxisUp},
	}

	for i := 0; i < 240; i++ {
		input := inputs[i%len(inputs)]
		first.Step(input)
		second.Step(input)
	}

	if *first != *second {
		t.Fatalf("same inputs produced different states: first=%+v second=%+v", first, second)
	}
}

func TestPaddlesStayInsideScreen(t *testing.T) {
	state := NewState()

	for i := 0; i < 200; i++ {
		state.Step(Input{Left: AxisUp, Right: AxisDown})
	}

	if state.LeftPaddle.Y != 0 {
		t.Fatalf("left paddle escaped top clamp: y=%d", state.LeftPaddle.Y)
	}
	if state.RightPaddle.Y != ScreenHeight-PaddleHeight {
		t.Fatalf("right paddle escaped bottom clamp: y=%d", state.RightPaddle.Y)
	}
}

func TestScoreResetsBall(t *testing.T) {
	state := NewState()
	state.Ball.X = -BallSize - 1
	state.Ball.VX = -BallDX

	state.Step(Input{})

	if state.RightScore != 1 {
		t.Fatalf("right score = %d, want 1", state.RightScore)
	}
	if state.Ball.X != (ScreenWidth-BallSize)/2 || state.Ball.Y != (ScreenHeight-BallSize)/2 {
		t.Fatalf("ball did not reset to center: %+v", state.Ball)
	}
	if state.Ball.VX != -BallDX {
		t.Fatalf("ball vx = %d, want %d", state.Ball.VX, -BallDX)
	}
}
