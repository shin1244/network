package main

import (
	"fmt"
	"log"

	sim "pongsim"
)

// VerifyResult는 서버가 기록된 입력 기반으로 서버에서 시뮬
type VerifyResult struct {
	Winner       uint8
	LeftScore    int
	RightScore   int
	GameOverTick uint32
	Completed    bool // 기록된 입력만으로 실제 게임 종료(승자 확정)까지 재현됐는지
}

// VerifyRoom은 room.Recorder에 쌓인 양쪽 플레이어의 입력을,
// 클라이언트와 동일한 로직으로 처음부터 시뮬레이션
func VerifyRoom(room *Room) VerifyResult {
	state := sim.NewState(sim.Player1)

	// 기록된 입력을 커맨드 큐에 적재 (클라이언트의 PushCommand와 동일한 방식)
	var maxTick uint32
	for tick, frame := range room.Recorder {
		if frame.P1Ready {
			state.PushCommand(tick, sim.Player1, sim.Axis(frame.P1))
		}
		if frame.P2Ready {
			state.PushCommand(tick, sim.Player2, sim.Axis(frame.P2))
		}
		if tick > maxTick {
			maxTick = tick
		}
	}

	// 양쪽 입력이 모두 준비된 틱만 순서대로 진행 (클라이언트 simulateCurrentTick과 동일)
	for state.Winner == 0 && state.Tick <= maxTick {
		frame := state.CommandQueue[state.Tick]
		if frame == nil || !frame.P1Ready || !frame.P2Ready {
			break // 입력이 비어 있으면 더 진행할 수 없음
		}

		state.Step(sim.Input{
			Player1: frame.P1Input,
			Player2: frame.P2Input,
		})
	}

	return VerifyResult{
		Winner:       state.Winner,
		LeftScore:    state.LeftScore,
		RightScore:   state.RightScore,
		GameOverTick: state.GameOverTick,
		Completed:    state.Winner != 0,
	}
}

// verifyReports는 서버가 재현한 정답과 각 클라이언트의 보고를 대조하고,
// 불일치(desync 또는 조작 의심)를 서버 로그로 경고.
func (l *Lobby) verifyReports(room *Room, result VerifyResult) {
	if !result.Completed {
		log.Printf("[VERIFY][WARN] Room %d: 기록된 입력만으로 게임 종료를 재현하지 못했습니다 "+
			"(server tick=%d, winner=%d). 입력 로그가 불완전할 수 있습니다.",
			room.ID, result.GameOverTick, result.Winner)
	}

	log.Printf("[VERIFY] Room %d server result: winner=%d score=%d:%d gameOverTick=%d",
		room.ID, result.Winner, result.LeftScore, result.RightScore, result.GameOverTick)

	for id, report := range room.Reports {
		if reason, ok := compareReport(report, result); !ok {
			log.Printf("[VERIFY][DESYNC] Room %d Client %d 결과 불일치 → %s "+
				"(client report: winner=%d score=%d:%d tick=%d)",
				room.ID, id, reason,
				report.Winner, report.LeftScore, report.RightScore, report.GameOverTick)
			continue
		}
		log.Printf("[VERIFY][OK] Room %d Client %d 보고가 서버 재현 결과와 일치", room.ID, id)
	}
}

// compareReport는 클라이언트 보고와 서버 정답을 비교해, 일치 여부와 불일치 사유를 반환합니다.
func compareReport(report GameOverReport, result VerifyResult) (reason string, ok bool) {
	if int32(result.Winner) != report.Winner {
		return fmt.Sprintf("winner 불일치(server=%d, client=%d)", result.Winner, report.Winner), false
	}
	if result.LeftScore != int(report.LeftScore) {
		return fmt.Sprintf("leftScore 불일치(server=%d, client=%d)", result.LeftScore, report.LeftScore), false
	}
	if result.RightScore != int(report.RightScore) {
		return fmt.Sprintf("rightScore 불일치(server=%d, client=%d)", result.RightScore, report.RightScore), false
	}
	if result.GameOverTick != report.GameOverTick {
		return fmt.Sprintf("gameOverTick 불일치(server=%d, client=%d)", result.GameOverTick, report.GameOverTick), false
	}
	return "", true
}
