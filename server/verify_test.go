package main

import (
	"testing"

	"pongsim"
)

// 서버가 기록된 입력만으로 경기를 재현했을 때,
// 동일 입력을 직접 돌린 참조 시뮬레이션과 결과가 완전히 일치하는지 검증합니다.
// (클라이언트와 서버가 같은 pongsim 코어를 쓰므로, 이는 곧 서버 검증의 정합성 보장입니다.)
func TestVerifyRoomReproducesGame(t *testing.T) {
	ref := sim.NewState(sim.Player1)
	recorder := make(map[uint32]*ReplayFrame)

	// 양 패들을 위로 고정 → 대부분 공을 놓쳐 득점이 쌓이고 결정론적으로 게임이 종료됨
	for ref.Winner == 0 {
		tick := ref.Tick
		p1, p2 := sim.AxisUp, sim.AxisUp

		recorder[tick] = &ReplayFrame{
			Tick:    tick,
			P1:      Axis(p1),
			P2:      Axis(p2),
			P1Ready: true,
			P2Ready: true,
		}

		ref.Step(sim.Input{Player1: p1, Player2: p2})

		if ref.Tick > 100000 {
			t.Fatal("참조 시뮬레이션이 종료되지 않음")
		}
	}

	room := &Room{ID: 1, Recorder: recorder}
	result := VerifyRoom(room)

	if !result.Completed {
		t.Fatal("VerifyRoom이 게임 종료를 재현하지 못함")
	}
	if result.Winner != ref.Winner {
		t.Fatalf("winner 불일치: server=%d, ref=%d", result.Winner, ref.Winner)
	}
	if result.LeftScore != ref.LeftScore || result.RightScore != ref.RightScore {
		t.Fatalf("score 불일치: server=%d:%d, ref=%d:%d",
			result.LeftScore, result.RightScore, ref.LeftScore, ref.RightScore)
	}
	if result.GameOverTick != ref.GameOverTick {
		t.Fatalf("gameOverTick 불일치: server=%d, ref=%d", result.GameOverTick, ref.GameOverTick)
	}
}

// 서버 정답과 클라이언트 보고가 어긋나면 불일치로 판정되는지 확인합니다.
func TestCompareReportDetectsMismatch(t *testing.T) {
	result := VerifyResult{Winner: 1, LeftScore: 3, RightScore: 1, GameOverTick: 500, Completed: true}

	honest := GameOverReport{Winner: 1, LeftScore: 3, RightScore: 1, GameOverTick: 500}
	if _, ok := compareReport(honest, result); !ok {
		t.Fatal("정직한 보고가 불일치로 판정됨")
	}

	// 승자를 조작한 보고
	cheat := GameOverReport{Winner: 2, LeftScore: 3, RightScore: 1, GameOverTick: 500}
	if _, ok := compareReport(cheat, result); ok {
		t.Fatal("조작된 승자 보고가 감지되지 않음")
	}

	// 점수를 조작한 보고
	cheatScore := GameOverReport{Winner: 1, LeftScore: 3, RightScore: 0, GameOverTick: 500}
	if _, ok := compareReport(cheatScore, result); ok {
		t.Fatal("조작된 점수 보고가 감지되지 않음")
	}
}
