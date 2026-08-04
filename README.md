<div align="center">

# Lockstep Pong — 서버 비용을 최소화한 실시간 1v1 대전

TCP 로비와 **UDP P2P Lockstep**으로 구현한 결정론적 실시간 Pong<br/>
서버는 게임 상태를 소유하지 않고 **매칭 · 홀펀칭 중개 · 검증 · 기록**만 담당하며,<br/>
실제 플레이는 두 클라이언트가 P2P로 **입력(input)만 교환**해 각자 동일하게 시뮬레이션합니다.

<br/>

<img src="docs/demo.jpeg" width="640" alt="두 클라이언트의 동일한 게임 상태 — Lockstep 동기화 결과" />

<sub>**▲ 두 클라이언트가 P2P로 입력만 교환하고, 각자 동일한 로직으로 계산한 결과 — 상태가 완전히 일치한다**</sub>

<br/>

🎬 **[데모 영상 보기](https://youtu.be/vFVg0i2LPBg)**

</div>

---

**이 프로젝트는 게임 콘텐츠 자체보다, "서버 비용을 어디까지 줄이면서도 실시간 반응성과 결과의 공정성을 동시에 지킬 수 있는가"를 직접 구현하며 검증하는 데 목적을 두었습니다.**

실시간 대전 게임의 정석은 **서버 권위(server-authoritative)** 구조입니다. 하지만 서버가 모든 방의 시뮬레이션을 직접 돌리고 매 틱 상태를 브로드캐스트하면, CPU와 대역폭 비용이 동시 접속 수에 비례해 늘어납니다. 저는 반대 방향에서 출발했습니다 — **서버는 최소한만 일하고, 계산은 클라이언트가 한다.** 그 대신 서버 권위를 포기했을 때 따라오는 두 가지 문제, **반응성 저하**와 **결과 신뢰성**을 구조로 해결하는 것이 이 프로젝트의 핵심 과제였습니다.

| 설계 목표 | 선택한 구조 | 트레이드오프 / 포기한 것 |
| :-- | :-- | :-- |
| **서버 비용 최소화** | Lockstep P2P — 서버는 중계 · 검증 · 기록만 | 서버 권위(state authority) |
| **빠른 반응성** | UDP P2P 직결 + Input Delay + ACK 재전송 | 입력 지연 N틱 |
| **결과의 공정성** | 결정론적 시뮬레이션 + 결과 제출 + 입력 로그 리플레이 재검증 | 상태 조작 방지를 서버 대신 구조로 대체 |

---

## 왜 Pong인가?

복잡한 게임 콘텐츠보다 **동기화 검증에 최적화된 단순한 규칙**이 필요했습니다.

- **단순한 규칙, 명확한 승패** — 양쪽 상태를 눈으로 비교하는 것만으로 동기화 성공 여부를 즉시 검증할 수 있음
- **입력 단순, 상태 직관적** — TCP/UDP 통신 구조와 결정론적 시뮬레이션을 명확히 실험하기에 적합
- **실시간성** — 틱 단위 입력 교환 · 관전 · 리플레이를 모두 구현하고 검증 가능

---

## 아키텍처

```
[Client 1] ──TCP── [Lobby Server] ──TCP── [Client 2]
     │            (매칭 · 방 관리 · 홀펀칭 중개)          │
     │                                                 │
     └──────────────  UDP P2P (입력 교환)  ─────────────┘
                            │
                  틱별 입력(5B)만 전송
                            │
              각자 동일한 결정론적 Step() 실행
                            │
                  → 양쪽 상태가 항상 일치
```

- **로비 (TCP)** — 매칭, 방 관리, UDP 홀펀칭 중개. 신뢰성이 필요한 제어 신호만 처리
- **게임 (UDP P2P)** — 틱 단위 입력 교환, ACK/재전송. 저지연·저비용이 최우선
- **시뮬레이션 (클라이언트)** — 정수 연산 기반 결정론적 `Step()` 함수
- **기록 (서버)** — 입력 로그 수집, 결과 확정, `.rep` 저장

<p align="center">
  <img src="docs/server-role.png" width="560" alt="서버의 역할 — 매칭 / 검증 / 기록" />
</p>

---

## 1 · 서버 비용 — 서버 권위 vs Lockstep P2P

> **한줄 요약** — 서버가 매 틱 시뮬레이션을 돌리고 상태를 브로드캐스트하는 대신, 서버는 매칭·홀펀칭 중개만 하고 실제 게임 연산과 트래픽은 두 클라이언트의 P2P로 넘겼습니다. 서버는 경기 중 게임 로직을 단 한 번도 실행하지 않습니다.

**문제 · 서버 권위 구조의 비용**
서버 권위 구조에서는 서버가 (1) 모든 방의 물리 시뮬레이션을 매 틱 실행하고 (2) 계산된 상태를 매 틱 양쪽에 브로드캐스트해야 합니다. 방이 늘어날수록 CPU와 대역폭이 선형으로 증가하고, 이는 곧 서버 비용입니다.

**선택 · 계산을 클라이언트로, 서버는 중개자로**
서버는 두 클라이언트를 매칭하고, UDP **홀펀칭으로 P2P 연결만 뚫어준 뒤 빠집니다.** 이후 이동·충돌·득점 판정은 서버를 거치지 않고 두 클라이언트가 각자 계산합니다.

<p align="center">
  <img src="docs/hole-punching.png" width="620" alt="Hole Punching을 통한 P2P 연결 수립" />
</p>

**결과 · 서버가 게임 중 하는 일**
경기가 시작되면 서버의 역할은 사실상 끝납니다. 게임 트래픽(초당 수십 틱의 입력)은 전부 클라이언트 사이에서만 오가고, 서버로는 **경기 종료 시점의 결과 보고와 관전용 입력 로그만** 흐릅니다. 즉 서버 비용이 "동시 경기 수 × 틱레이트"가 아니라 "동시 접속 수"에만 걸립니다.

<details>
<summary><b>서버는 주소만 교환하고 빠진다 — 코드로 보기</b></summary>

<br/>

```go
// 서버: P2P 주소만 서로에게 알려주고 게임에서 손을 뗀다
func (s *Server) StartHolePunching(room *Room) {
    if len(room.Players) < 2 {
        return
    }
    p1 := room.Players[0]
    p2 := room.Players[1]

    // P1에게 P2의 공인 UDP 주소를, P2에게 P1의 주소를 전송
    p1.Send <- MakePacket(SceneGame, CmdStartHolePunch, SerializeUDPAddr(p2.UDPAddr))
    p2.Send <- MakePacket(SceneGame, CmdStartHolePunch, SerializeUDPAddr(p1.UDPAddr))

    // 이후 서버는 게임 로직을 실행하지 않는다
    p1.State = ClientStateGame
    p2.State = ClientStateGame
}
```

</details>

---

## 2 · 빠른 반응성 — P2P 직결 + Input Delay + ACK 재전송

> **한줄 요약** — Lockstep은 원래 "상대 입력이 도착해야 다음 틱을 진행"하기 때문에 그대로 쓰면 매 틱 네트워크를 기다리다 멈춥니다. P2P 직결로 서버 홉을 없애고, 입력을 몇 틱 미래로 예약(Input Delay)해 왕복 시간을 숨기며, UDP 손실은 ACK 재전송으로 메웠습니다.

**문제 · Lockstep은 기본적으로 "기다리는" 구조**
결정론을 유지하려면 어떤 틱을 시뮬레이션하기 전에 **양쪽 플레이어의 입력이 모두** 있어야 합니다. 순진하게 구현하면 매 틱 상대의 입력 패킷을 기다리게 되고, 여기에 서버까지 경유하면 왕복 지연이 두 배가 됩니다.

**원인 · 두 가지 지연 요소**
- **경로 지연** — 입력이 `클라 → 서버 → 클라`로 돌면 RTT가 불필요하게 커짐
- **동기 대기** — 현재 틱의 상대 입력이 아직 안 왔으면 시뮬레이션 자체가 멈춤

**해결 · 세 가지를 조합**
1. **P2P 직결** — UDP 홀펀칭으로 클라이언트끼리 직접 주고받아 서버 홉을 제거
2. **Input Delay** — 지금 누른 입력을 현재 틱이 아니라 `현재 틱 + InputDelay`에 예약해서 전송. 그 틱이 실제로 시뮬레이션될 때쯤 상대 입력이 이미 도착해 있어, **입력을 기다리느라 멈추는 상황을 구조적으로 회피**함 (기본 InputDelay = 3틱)
3. **ACK 기반 재전송** — UDP는 손실을 허용하므로, ACK 안 온 입력을 `unackedInputs`에 두고 100ms마다 재전송해 **손실 상황에서도 동기화가 깨지지 않도록** 보장

**트레이드오프**
Input Delay만큼(3틱, 60TPS 기준 약 50ms) 내 입력이 화면에 늦게 반영됩니다. 대신 그 대가로 "상대 입력을 기다리다 프레임이 멈추는" 훨씬 치명적인 문제를 없앴습니다. 반응성은 *일정한 소량 지연*과 *간헐적 큰 멈춤* 사이의 선택이었고, 전자를 택했습니다.

<p align="center">
  <img src="docs/commandqueue.png" width="620" alt="틱 기반 입력 처리와 CommandQueue" />
</p>

<details>
<summary><b>입력을 미래 틱에 예약 + ACK 재전송 — 코드로 보기</b></summary>

<br/>

```go
// 지금 누른 입력을 "현재 틱 + InputDelay"에 예약해 미리 보낸다
func (g *GameScene) handleLocalInput() {
    state := g.state
    futureTick := state.Tick + InputDelay          // ← 왕복 시간을 벌기 위한 예약
    input := keyboardAxis(ebiten.KeyArrowUp, ebiten.KeyArrowDown)

    state.PushCommand(futureTick, state.Player, input)

    if g.netplay != nil && g.netplay.IsReady() {
        g.netplay.SendLocalInput(futureTick, input)
        g.recordReplayInput(futureTick, input)
    }
}

// 현재 틱은 양쪽 입력이 모두 준비됐을 때만 진행 (결정론 유지)
func (g *GameScene) simulateCurrentTick() {
    frame, ok := g.state.CommandQueue[g.state.Tick]
    if !ok || !frame.P1Ready || !frame.P2Ready {
        return                                      // 아직 안 왔으면 이 틱은 대기
    }
    g.state.Step(Input{Player1: frame.P1Input, Player2: frame.P2Input})
    delete(g.state.CommandQueue, g.state.Tick)
}
```

```go
// UDP 손실 대응: ACK 안 온 입력을 주기적으로 재전송
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

## 3 · 결과의 공정성 — 서버 권위 없이 신뢰 확보

> **한줄 요약** — 서버가 게임 상태를 소유하지 않는다면 "누가 이겼는가"를 무엇으로 믿을 수 있을까? 답은 **결정론**입니다. 같은 입력이면 누가 계산하든 반드시 같은 결과가 나오도록 만들고, 서버는 전체 입력 로그를 기록해 **언제든 재생으로 결과를 재현·재검증**할 수 있게 했습니다.

**문제 · 권위 없는 구조의 신뢰 문제**
서버 권위 구조에서는 서버가 계산한 값이 곧 정답입니다. 하지만 이 프로젝트처럼 계산을 클라이언트에 맡기면, "한쪽이 자기 유리하게 결과를 우긴다면?"이라는 신뢰 문제가 생깁니다.

**해결 · 공정성을 떠받치는 세 겹**

1. **결정론적 시뮬레이션** — 물리·충돌·득점을 전부 **정수 연산과 고정 상수**로 구현해 부동소수점 오차를 원천 차단했습니다. 같은 입력 시퀀스는 어느 기기에서 돌려도 비트 단위로 동일한 결과를 냅니다. 즉 결과를 결정하는 것은 클라이언트의 "주장"이 아니라 **공유된 입력과 고정된 규칙**입니다.

2. **양측 결과 제출** — 게임이 끝나면 각 클라이언트가 승자·점수·종료 틱을 담은 `GameOverReport`를 서버에 제출합니다. 서버는 양측 보고가 모두 도착해야 경기를 확정합니다.

3. **입력 로그 기록 → 리플레이 재검증** — 서버는 경기 내내 오간 **모든 입력을 틱 단위로 저장**합니다. 저장된 입력 로그를 결정론적 로직에 다시 흘려보내면 원래 경기가 **완벽히 동일하게 재현**되므로, 결과에 다툼이 생겨도 사후에 진실을 재구성할 수 있습니다. 상태가 아니라 입력만 저장하면 되므로 기록 비용도 매우 작습니다.

**배운 점**

| 구분 | 서버 권위 | 이 프로젝트 (Lockstep + 결정론) |
| :-- | :-- | :-- |
| **결과의 근거** | 서버가 계산한 상태 | 공유된 입력 + 고정된 결정론 규칙 |
| **서버 비용** | 방 수 × 틱레이트에 비례 | 동시 접속 수에만 비례 |
| **공정성 보장 방식** | 서버가 실시간 강제 | 결정론 + 입력 로그 재현으로 사후 검증 |
| **한계** | 비용 · 확장성 | 클라이언트 신뢰 경계 관리 필요 |

서버 권위를 포기한다고 해서 공정성까지 포기해야 하는 것은 아니었습니다. **"결과를 신뢰한다"를 "입력을 신뢰하고, 규칙을 고정한다"로 바꾸면**, 서버가 매 틱 개입하지 않고도 결과를 검증 가능한 형태로 남길 수 있었습니다.

<details>
<summary><b>결정론적 Step + 결과 제출 — 코드로 보기</b></summary>

<br/>

```go
// 물리 로직 전체가 정수 연산 — 부동소수점 오차 없음 → 어디서 돌려도 동일 결과
func (s *State) Step(input Input) {
    s.Tick++
    s.movePaddle(&s.LeftPaddle, input.Player1)
    s.movePaddle(&s.RightPaddle, input.Player2)
    s.moveBall()
}

func (s *State) movePaddle(paddle *Rect, axis Axis) {
    paddle.Y += int(axis) * PaddleSpeed              // 정수 연산
    paddle.Y = clamp(paddle.Y, 0, ScreenHeight-PaddleHeight)
}
```

```go
// 게임 종료 시 양측이 결과를 서버에 제출
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

    g.sendPacket(network.MakePacket(1, byte(2), payload))
}
```

</details>

---

## 4 · 리플레이 · 관전 — "입력만 저장하는" 구조의 부수 효과

> **한줄 요약** — 공정성을 위해 이미 입력 로그를 기록하고 있으므로, 같은 로그를 파일로 저장하면 **리플레이**, 실시간으로 관전자에게 흘려보내면 **라이브 관전**이 됩니다. 상태가 아닌 입력만 다루기 때문에 두 기능 모두 거의 추가 비용 없이 얻어집니다.

- **리플레이 (Replay)** — 확정된 입력 로그를 `PONGREP1` 헤더의 `.rep` 파일로 저장. 재생 시 결정론적 로직에 다시 흘려보내면 원본 경기가 그대로 재현됩니다.
- **관전 (Broadcast)** — 틱별 입력 로그를 관전자에게 실시간 브로드캐스트. 관전자도 동일한 로직으로 계산하므로, 서버가 화면 상태를 보내주지 않아도 라이브로 경기를 따라볼 수 있습니다.

핵심은 **상태(state)가 아니라 입력(input)을 진실의 원천으로 삼았기 때문에** 검증·리플레이·관전이 하나의 데이터(입력 로그)에서 파생된다는 점입니다.

<details>
<summary><b>입력 로그를 .rep로 저장 — 코드로 보기</b></summary>

<br/>

```go
// 서버: 경기 내내 쌓은 입력 로그를 틱 순서대로 .rep 파일로 저장
func saveReplay(room *Room) {
    ticks := make([]uint32, 0, len(room.Recorder))
    for tick := range room.Recorder {
        ticks = append(ticks, tick)
    }
    sort.Slice(ticks, func(i, j int) bool { return ticks[i] < ticks[j] })

    payload := make([]byte, 0, 12+len(ticks)*8)
    payload = append(payload, []byte("PONGREP1")...)                 // 매직 헤더
    payload = binary.BigEndian.AppendUint32(payload, uint32(room.ID))
    payload = binary.BigEndian.AppendUint32(payload, uint32(len(ticks)))

    for _, tick := range ticks {                                    // 상태가 아니라 입력만 저장
        frame := room.Recorder[tick]
        payload = binary.BigEndian.AppendUint32(payload, tick)
        payload = append(payload, byte(int8(frame.P1)))
        payload = append(payload, byte(int8(frame.P2)))
        payload = append(payload, boolByte(frame.P1Ready))
        payload = append(payload, boolByte(frame.P2Ready))
    }
    os.WriteFile(filepath.Join("replay", fmt.Sprintf("room_%d.rep", room.ID)), payload, 0644)
}
```

</details>

---

## 바이너리 패킷 프로토콜

모든 통신은 `Scene(1B) + Command(1B) + Payload Length(4B) + Payload`의 경량 바이너리 구조를 사용합니다. 고정 6바이트 헤더를 먼저 읽고, 명시된 길이만큼 페이로드를 읽어 TCP 스트림에서도 패킷 경계를 정확히 구분합니다. 게임 입력 패킷은 페이로드가 **단 5바이트**(틱 4B + 입력 1B)로, 실시간 교환에 드는 대역폭을 최소화했습니다.

<p align="center">
  <img src="docs/packet-protocol.png" width="640" alt="바이너리 패킷 프로토콜 구조" />
</p>

```go
func MakePacket(scene byte, command byte, payload []byte) []byte {
    header := make([]byte, 6)
    header[0] = scene
    header[1] = command
    binary.BigEndian.PutUint32(header[2:6], uint32(len(payload)))
    return append(header, payload...)
}
```

---

## 기술 스택

`Go` · `UDP` · `TCP` · `P2P Hole Punching` · `Lockstep` · `Deterministic Simulation` · `Ebiten`

<div align="center">
<sub>Made by <a href="https://github.com/shin1244">shin1244</a></sub>
</div>
