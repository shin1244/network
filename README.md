<div align="center">

# Lockstep Pong

### 서버 비용을 최소화한 실시간 1v1 대전 구조를<br/>UDP P2P 락스텝으로 직접 구현했습니다.

`Go` `UDP Hole Punching` `Lockstep` `결정론 시뮬레이션` `서버 사이드 검증`

<img width="400" alt="게임 화면" src="https://github.com/user-attachments/assets/0b16b70f-8593-445a-9837-e1f5b47ca3bb" />

🎬 **[데모 영상](https://youtu.be/vFVg0i2LPBg)**

</div>

---

## 설계 결정과 그 대가

하나의 선택이 다음 문제를 만들었고, 그 문제를 해결한 과정을 순서대로 정리했습니다.

| 결정 | 얻은 것 | 새로 생긴 문제 | 해결 |
| :-- | :-- | :-- | :-- |
| 게임 트래픽을 **P2P**로 | 서버의 틱 연산·상태 브로드캐스트 제거 | UDP 유실 시 두 클라이언트 상태가 갈라짐 | **락스텝** + ACK 재전송 |
| 상태 대신 **입력만 교환** | 대역폭 최소화, 리플레이·관전이 따라옴 | 상대 입력을 기다리며 게임이 멈춤 | **Input Delay** (+3틱) |
| 서버가 **경기 상태를 모름** | 서버 비용 최소 | 결과를 클라이언트가 자기 신고 | 종료 후 **서버 1회 재시뮬레이션** |

<p align="center">
  <img src="docs/server-role.png" width="560" alt="서버의 역할 — 매칭 / 검증 / 기록" />
</p>

---

## 프로젝트를 시작한 이유

실시간 게임의 일반적인 서버 권위 구조에서는 서버가 매 틱 게임 상태를 계산해 클라이언트에게 전달합니다. 안정적이지만 경기 수가 늘수록 서버의 연산량과 네트워크 트래픽이 함께 증가합니다.

```text
Client → Input → Server
Client ← State ← Server
```

여기서 상태 전송을 걷어내면 어디까지 갈 수 있을지 궁금해 다음 세 가지를 목표로 잡았습니다.

- **낮은 서버 비용** — 서버가 경기 상태를 매 틱 계산하거나 브로드캐스트하지 않는다
- **통신 지연 절감** — 입력이 서버를 거치지 않고 상대에게 직접 전달된다
- **공정성** — 서버가 경기 결과를 독립적으로 검증할 수 있어야 한다

앞의 두 가지는 P2P로 풀리지만, 세 번째는 P2P를 선택했기 때문에 **생겨난** 문제였습니다.

---

## 1 · 게임을 서버에서 분리하다

서버는 매칭과 UDP 홀펀칭만 담당하고, 연결이 성립되면 게임 데이터에서 완전히 손을 뗍니다.

```text
                    Server
               ┌─────────────┐
               │ Matchmaking │
               │ Hole Punch  │
               └──────┬──────┘
                 UDP 주소 전달
          ┌───────────┴───────────┐
          ▼                       ▼
     ┌─────────┐   UDP P2P   ┌─────────┐
     │ Client 1│ <─────────> │ Client 2│
     └─────────┘             └─────────┘
```

전송은 TCP 대신 UDP를 쓰되, ACK·재전송처럼 게임에 필요한 신뢰성만 직접 구현했습니다. 대신 두 가지가 남았습니다. UDP는 전달을 보장하지 않고, 입력 하나만 빠져도 두 클라이언트의 상태가 갈라집니다.

---

## 2 · 상태가 아니라 입력을 동기화하다

> **한줄 요약** — 상태를 주고받는 대신 입력만 교환하고, 양쪽 입력이 모두 도착한 틱만 실행합니다.

Pong의 입력은 `위 / 아래 / 정지` 세 가지뿐입니다. 같은 입력을 같은 순서로 처리하면 두 클라이언트가 동일한 상태에 도달할 수 있습니다.

```text
Client 1                    Client 2
Input ────────────────────> Input
      <──────────────────── 
              ↓
        Tick N Simulation
              ↓
        동일한 Game State
```

**결정론 확보** · 물리와 충돌은 전부 정수 연산으로 처리했습니다. 부동소수점 연산은 환경에 따라 결과가 달라질 수 있어 락스텝의 전제를 깨뜨립니다.

**로직 공유** · 게임 로직을 `pongsim` 모듈로 분리해 클라이언트와 서버가 같은 코드를 실행하도록 했습니다. 이 선택이 뒤에서 서버 검증을 가능하게 만듭니다.

<p align="center">
  <img src="docs/commandqueue.png" width="620" alt="틱 기반 입력 처리와 CommandQueue" />
</p>

<details>
<summary><b>양쪽 입력이 준비된 틱만 실행 — 코드로 보기</b></summary>

<br/>

```go
func (g *GameScene) simulateCurrentTick() {
    frame, ok := g.state.CommandQueue[g.state.Tick]
    if !ok || !frame.P1Ready || !frame.P2Ready {
        return                                   // 아직 안 왔으면 이 틱은 대기
    }
    g.state.Step(Input{Player1: frame.P1Input, Player2: frame.P2Input})
    delete(g.state.CommandQueue, g.state.Tick)
}
```

</details>

---

## 3 · UDP 입력 유실을 해결하다

락스텝에서는 입력 하나가 유실되면 이후 모든 틱이 어긋납니다. 그래서 전송한 입력은 ACK를 받을 때까지 `unackedInputs`에 보관하고, 100ms가 지나도 응답이 없으면 재전송합니다.

TCP처럼 모든 데이터에 완전한 신뢰성을 부여하는 대신, **게임 입력에 필요한 만큼의 신뢰성만 UDP 위에 직접 구현**했습니다.

<details>
<summary><b>ACK 재전송 — 코드로 보기</b></summary>

<br/>

```go
func (n *NetplaySession) ResendUnackedInputs() {
    now := time.Now()
    for tick, pending := range n.unackedInputs {
        if now.Sub(pending.lastSentAt) < InputResendInterval {  // 100ms
            continue
        }
        n.sendInput(tick, pending.input)
        pending.lastSentAt = now
        n.unackedInputs[tick] = pending
    }
}
```

</details>

---

## 4 · 멈추는 락스텝을 Input Delay로 풀다

> **한줄 요약** — 양쪽 입력을 기다리느라 게임이 멈추던 문제를, 입력을 미래 틱에 예약해 왕복 시간을 미리 버는 방식으로 해결했습니다.

락스텝은 양쪽 입력이 다 와야 다음 틱을 실행하므로, 상대 패킷이 조금만 늦어도 화면이 멈췄습니다. 그래서 지금 누른 입력을 즉시 실행하지 않고 **현재 Tick + 3**에 예약해 전송합니다. 그 틱이 실제로 실행될 때쯤이면 상대 입력도 대부분 도착해 있습니다.

> 📌 **Trade-off** — 입력이 3틱만큼 늦게 반영되지만, 멈췄다 한꺼번에 진행되는 것보다 일정한 진행을 유지하는 편이 체감상 훨씬 자연스럽다고 판단했습니다.

<details>
<summary><b>입력을 미래 틱에 예약 — 코드로 보기</b></summary>

<br/>

```go
func (g *GameScene) handleLocalInput() {
    state := g.state
    futureTick := state.Tick + InputDelay        // ← 왕복 시간을 벌기 위한 예약
    input := keyboardAxis(ebiten.KeyArrowUp, ebiten.KeyArrowDown)

    state.PushCommand(futureTick, state.Player, input)

    if g.netplay != nil && g.netplay.IsReady() {
        g.netplay.SendLocalInput(futureTick, input)
        g.recordReplayInput(futureTick, input)
    }
}
```

</details>

---

## 5 · 입력만 있으면 리플레이도 된다

입력을 저장하고 있다면 같은 시뮬레이션에 다시 흘려보내는 것만으로 경기를 재현할 수 있습니다. 상태 스냅샷 없이 입력 로그만으로 `.rep` 파일이 만들어집니다.

문제는 입력이 P2P로 상대에게만 갔다는 점이었습니다. 그래서 클라이언트가 입력을 두 경로로 보내도록 바꿨습니다.

```text
Client 1 ─── UDP P2P ───> Client 2        (실시간 플레이)

Client 1 ──┐
           ├── TCP Batch ──> Server        (60틱마다 배치)
Client 2 ──┘
```

두 경로가 분리돼 있어 서버로 가는 배치 전송이 실시간 플레이 지연에 영향을 주지 않습니다.

---

## 6 · 리플레이에서 관전으로 확장하다

> **관전자는 경기 도중에 들어오는데, 현재 상황까지 어떻게 따라오게 만들까?**

현재 상태를 통째로 보내는 방법도 있었지만, 입력만 주고받아 온 구조와 맞지 않았습니다. 답은 **스트리트 파이터 6의 관전 방식**에서 찾았습니다. 상태 대신 처음부터의 입력을 빠르게 재생해 현재 시점까지 따라잡는 방식으로, 락스텝 구조는 이미 그 조건을 갖추고 있었습니다.

<p align="center">
  <img width="400" alt="SF6 관전" src="https://github.com/user-attachments/assets/b0e3c09e-4732-4f6c-8881-9504b65b1383" />
  &nbsp;
  <img width="313" alt="관전 화면" src="https://github.com/user-attachments/assets/61097642-6e59-4aa3-b8d7-d01fa7c8b70f" />
</p>

관전자는 많이 뒤처져 있으면 4배속으로 시뮬레이션을 돌려 따라잡고, 현재 틱에 가까워지면 실시간 속도로 전환합니다. 관전자에게도 상태가 아닌 입력만 전달하므로 서버의 네트워크 비용은 그대로 유지됩니다.

<details>
<summary><b>관전자 빨리 감기 — 코드로 보기</b></summary>

<br/>

```go
func (o *ObserverScene) updateSpeed() {
    if o.state.Tick >= o.latestReceivedTick {
        o.speed = observerLiveSpeed
        return
    }
    lag := int(o.latestReceivedTick) - int(o.state.Tick)

    if o.speed == observerLiveSpeed && lag > observerCatchUpLagTicks {
        o.speed = observerCatchUpSpeed      // 많이 뒤처짐 → 4배속
        return
    }
    if o.speed == observerCatchUpSpeed && lag <= observerLiveLagTicks {
        o.speed = observerLiveSpeed         // 거의 따라잡음 → 실시간
    }
}
```

</details>

---

## 7 · 서버가 경기를 다시 실행해 결과를 검증하다

> **한줄 요약** — 서버가 경기 상태를 모르니 결과를 믿을 수 없었고, 이미 쌓아둔 입력 로그로 종료 후 한 번만 재시뮬레이션해 해결했습니다.

앞의 두 목표는 달성했지만 가장 큰 구멍이 남았습니다. 서버가 경기 상태를 모르기 때문에, 한쪽이 조작된 결과를 보내도 거짓 여부를 확인할 수 없습니다. 처음에는 양쪽 결과가 일치할 때만 신뢰하는 방식을 썼지만, 결국 클라이언트를 믿어야 한다는 점은 그대로였습니다.

해결의 실마리는 라이엇의 **[2XKO 온라인 플레이 방식](https://2xko.riotgames.com/ko-kr/news/dev/how-2xko-handles-online-play/)** 에서 얻었습니다. 클라이언트끼리는 기존대로 플레이하고, 서버는 별도로 같은 게임을 시뮬레이션해 결과를 검증하는 구조입니다. 리플레이·관전을 위해 이미 양쪽 입력을 서버에 기록하고 있었으므로, 그 입력으로 **경기를 한 번 더 실행하기만** 하면 됐습니다.

```text
        Client 1 ←── UDP ──→ Client 2
             └──── Inputs ─────┘
                     ▼
                   Server
                     │  경기 종료 후 1회
                Re-Simulation
                     ▼
          Server Result  ↔  Client Report
                     비교
```

핵심은 서버가 **경기 중에는 아무것도 계산하지 않는다**는 점입니다. 검증은 종료 후 단 한 번뿐이고, Pong 시뮬레이션은 가벼워 부담이 작습니다. 상태 브로드캐스트는 없애고 공정성을 위한 최소 계산만 서버에 남긴 구조입니다.

<details>
<summary><b>서버 검증 시뮬레이션 — 코드로 보기</b></summary>

<br/>

```go
// 기록된 입력만으로 경기를 처음부터 재현해 "정답"을 계산
func VerifyRoom(room *Room) VerifyResult {
    state := sim.NewState(sim.Player1)

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

    // 클라이언트와 동일한 규칙으로 진행
    for state.Winner == 0 && state.Tick <= maxTick {
        frame := state.CommandQueue[state.Tick]
        if frame == nil || !frame.P1Ready || !frame.P2Ready {
            break
        }
        state.Step(sim.Input{Player1: frame.P1Input, Player2: frame.P2Input})
    }

    return VerifyResult{
        Winner: state.Winner, LeftScore: state.LeftScore,
        RightScore: state.RightScore, Completed: state.Winner != 0,
    }
}

// 서버 정답과 클라이언트 보고가 어긋나면 desync 또는 조작으로 판정
func compareReport(report GameOverReport, result VerifyResult) (string, bool) {
    if int32(result.Winner) != report.Winner {
        return "winner 불일치", false
    }
    if result.LeftScore != int(report.LeftScore) ||
        result.RightScore != int(report.RightScore) {
        return "score 불일치", false
    }
    return "", true
}
```

</details>

---

## 최종 구조

| 역할 | 담당 |
| :-- | :-- |
| 매칭 · 방 관리 · UDP 홀펀칭 | Server |
| 게임 입력 전달 | Client ↔ Client |
| 게임 시뮬레이션 | Client |
| **게임 상태 브로드캐스트** | **없음** |
| 리플레이 입력 기록 · 저장 | Server |
| 관전자 입력 중계 | Server |
| **경기 결과 검증** | **Server (종료 후 1회)** |

```text
PONG/
├─ sim/       # pongsim — 결정론 시뮬레이션 코어 (클라·서버 공유)
├─ client/    # Ebiten 클라이언트 — 게임 / 리플레이 / 관전 씬
└─ server/    # TCP 로비 + UDP 중개 + 기록 / 검증
```

---

## Trade-off

이 구조가 모든 실시간 게임에 적합하지는 않습니다.

**얻은 것** · 서버가 매 틱 계산·브로드캐스트를 하지 않음 · 입력이 서버를 거치지 않아 지연 감소 · 입력 로그만으로 리플레이와 관전이 따라옴 · 서버가 결과를 독립 검증 가능

**감수한 것** · 상대 입력 지연이 곧 내 화면 지연 · Input Delay만큼 반응성 손해 · 결정론 시뮬레이션과 NAT Traversal 구현 필요 · 로직이 바뀌면 기존 리플레이 호환성이 깨짐

**아직 해결하지 못한 것** · 클라이언트가 P2P와 서버에 서로 다른 입력을 보내면 재시뮬레이션 검증을 우회할 수 있습니다. 양쪽 입력에 서명을 붙이거나 상대가 받은 입력을 교차 검증하는 방식이 필요한데, 현재 구조에는 반영되어 있지 않습니다.

따라서 이 구조는 **상태가 작고 결정론적으로 만들 수 있는 실시간 게임**에 적합하다고 판단했습니다.

---

## 프로젝트에서 배운 것

```text
서버 비용을 줄이고 싶다 → P2P → 동기화 문제 → Lockstep
    → UDP 유실 → ACK/재전송 → 입력 지연 → Input Delay
        → 입력 로그가 남는다 → Replay · Observer
            → 서버에도 입력이 있다 → Server Verification
```

가장 크게 남은 것은 특정 네트워크 기술을 구현했다는 사실이 아니라, **하나의 구조적 선택이 어떤 문제를 만들고 그 다음 구조를 어떻게 결정하는가**를 끝까지 따라가 본 경험이었습니다. 서버 비용을 줄이려 시작한 선택이 마지막에는 공정성이라는 전혀 다른 문제로 돌아왔고, 그것을 앞선 선택의 부산물(입력 로그)로 해결할 수 있었습니다.

---

<div align="center">

**Go** · **Ebiten** · **TCP / UDP** · **UDP Hole Punching** · **Lockstep** · **결정론 시뮬레이션** · **서버 사이드 검증**

🎬 **[데모 영상](https://youtu.be/vFVg0i2LPBg)**

</div>
