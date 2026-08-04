<div align="center">

# Lockstep Pong

UDP P2P 락스텝으로 구현한 실시간 1v1 Pong 게임으로<br/>
**서버 비용 절감**과 **통신 지연 절감**, **공정성**에 중점을 둔 프로젝트 입니다.

<br/>

<img width="400" height="148" alt="게임 화면" src="https://github.com/user-attachments/assets/0b16b70f-8593-445a-9837-e1f5b47ca3bb" />


<br/>

🎬 **[데모 영상 보기](https://youtu.be/vFVg0i2LPBg)**

</div>

---

## 목표

서버 비용을 최대한 줄일 수 있는 실시간 대전 구조를 만들어보고 싶었습니다.

보통의 서버 권위 구조는 서버가 모든 경기 상태를 계산해서 플레이어에게 계속 브로드캐스트합니다. 안정적이지만 경기가 늘면 연산과 네트워크 비용도 같이 늘어납니다. 그래서 첫 목표를 세웠습니다.

**낮은 서버 비용** — 서버가 경기 상태를 매 틱 계산하거나 브로드캐스트하지 않기

실시간 대전인 만큼 입력도 최대한 빨리 전달되어야 했습니다.

**통신 지연 절감** — 입력이 서버를 거치지 않고 상대에게 바로 도달하기

두 목표 모두 서버를 거치지 않는 P2P 구조라면 해결할 수 있을 것이라 생각했고, Lockstep을 기반으로 전체 구조를 설계했습니다.

하지만 이렇게 되자 서버는 경기 상태를 알 수 없게 되었고, 클라이언트가 보낸 경기 결과를 그대로 믿을 수밖에 없는 문제가 생겼습니다.

그래서 목표를 하나 더 추가했습니다.

**공정성** — 서버가 경기 결과를 독립적으로 검증할 수 있을 것

이 세 목표를 구현하면서 부딪힌 문제들을 순서대로 정리했습니다.

---

## 왜 Pong인가?

**동기화 검증에 최적화된 단순한 규칙**이 필요했습니다.

- **단순한 규칙** — 양쪽 상태를 눈으로 비교하는 것만으로 동기화 성공 여부를 즉시 검증할 수 있음
- **직관적** — TCP/UDP 통신 구조와 결정론적 시뮬레이션을 실험하기에 적합
- **실시간성** — 실시간 입력 동기화, 관전, 리플레이까지 구현하기에 적합

---

## UDP P2P를 선택하다

낮은 서버 비용과 통신 지연 절감, 둘 다 게임 트래픽이 서버를 거치지 않으면 해결되는 문제였습니다. 그래서 P2P 구조로 구현을 시작했습니다. 서버는 매칭과 UDP 홀펀칭만 담당하고, 연결이 완료되면 더 이상 게임 데이터는 전달하지 않습니다.

전송 프로토콜은 TCP 대신 UDP를 사용하고, ACK와 재전송 등 게임에 필요한 기능만 직접 구현했습니다.

```go
// 서버: 두 피어에게 서로의 주소만 알려주고 게임에서 손을 뗀다
func (s *Server) StartHolePunching(room *Room) {
    p1, p2 := room.Players[0], room.Players[1]

    p1.Send <- MakePacket(SceneGame, CmdStartHolePunch, SerializeUDPAddr(p2.UDPAddr))
    p2.Send <- MakePacket(SceneGame, CmdStartHolePunch, SerializeUDPAddr(p1.UDPAddr))

    // 이후 서버는 이 경기의 게임 로직을 실행하지 않는다
    p1.State = ClientStateGame
    p2.State = ClientStateGame
}
```
이 구조 덕분에 서버는 경기 상태를 계산하거나 브로드캐스트하지 않아도 되고, 게임 입력도 서버를 거치지 않게 되었습니다.

하지만 UDP는 입력 하나만 유실되어도 그 차이가 점점 커져 두 클라이언트가 서로 다른 게임을 진행하게 됩니다. 두 클라이언트가 같은 게임을 플레이하고 있다는 걸 어떻게 보장할지가 다음 문제였습니다.

---

## 락스텝 동기화

앞의 문제를 풀 방법은 평소 즐겨 하던 게임 스타크래프트에서 찾았습니다. 락스텝(Lockstep) 방식입니다.

예전부터 수백 개의 유닛이 동시에 움직이는 RTS가 어떻게 당시 컴퓨터와 네트워크 환경에서도 원활하게 멀티플레이를 지원할 수 있었는지 궁금했습니다.

얼마 전 스타크래프트가 배틀넷으로 클라이언트들을 연결한 뒤, 게임 상태가 아니라 플레이어의 입력만 주고받고 각자 같은 로직을 같은 순서로 실행한다는 사실을 알게 되었습니다.

제가 만들고 있던 게임도 입력만 주고받는 구조였기 때문에, 이 방식을 그대로 적용해 보기로 했습니다.

이를 위해 시뮬레이션을 다음처럼 구성했습니다.

- 공과 패들의 좌표가 아니라 `위 / 아래 / 정지` 입력만 전송
- 물리와 충돌은 모두 정수 연산으로 구현
- 양쪽 입력이 모두 도착한 틱만 실행

<p align="center"> <img src="docs/commandqueue.png" width="620" alt="틱 기반 입력 처리와 CommandQueue" /> </p>

UDP는 도착을 보장하지 않으므로 보낸 입력은 ACK가 올 때까지 unackedInputs에 두고 100ms마다 재전송하도록 만들었습니다. 입력이 유실되더라도 ACK를 받을 때까지 재전송하므로 결국 같은 입력을 공유하게 됩니다.

락스텝은 양쪽 입력이 모두 들어와야 다음 틱으로 넘어가니 매 틱 상대 패킷을 기다리다 멈추는 문제가 발생했습니다.

이 문제는 Input Delay로 풀어보았습니다. 지금 누른 입력이 바로 현재 틱에 적용되는 것이 아니라 현재 틱 + 3에 예약해서 보냅니다. 그 틱이 실제로 시뮬레이션될 때쯤엔 상대 입력이 이미 도착해 있어서 게임이 뚝뚝 끊기는 현상을 막을수 있었습니다.

**트레이드오프**가 있다면 약간의 입력 지연은 생기지만 게임이 멈췄다가 한 번에 진행되는 현상보다 플레이하기 훨씬 자연스러웠습니다.

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

## 실시간 리플레이 · 관전 — 스트리트 파이터 6에서 가져오다

게임이 도는 걸 확인한 뒤, 다음 목표는 **진행 중인 경기를 실시간으로 관전**하는 것이었습니다. 그런데 관전자는 경기 중간에 들어옵니다. 어떻게 현재 상황까지 따라잡게 할까?

**스트리트 파이터 6**의 방식에서 답을 가져왔습니다 — 상태를 통째로 전송하는 대신, **처음부터의 조작(입력)을 빠르게 다시 재생**해 현재 시점까지 따라붙는 방식입니다.

마침 이 구조와 완벽히 맞았습니다. 락스텝 덕분에 **이미 입력만으로 경기 전체를 재현**할 수 있으니:

- **리플레이** — 확정된 입력 로그를 `PONGREP1` 헤더의 `.rep` 파일로 저장. 재생 시 결정론 로직에 다시 흘리면 원본 경기가 그대로 재현됨
- **관전** — 서버가 틱별 입력 로그를 관전자에게 브로드캐스트. 관전자는 받은 입력을 **배속으로 빨리 감기**해 현재 진행 상황까지 따라잡고, 따라잡은 뒤엔 실시간 속도로 전환

관전자에게도 **화면(상태)이 아니라 입력만** 흐르므로 서버가 부담하는 대역폭이 작습니다.

<details>
<summary><b>관전자의 빨리 감기 + 입력 로그 저장 — 코드로 보기</b></summary>

<br/>

```go
// 관전자: 진행 상황에 뒤처지면 배속으로 따라잡고, 따라잡으면 실시간 속도로
func (o *ObserverScene) updateSpeed() {
    if o.state.Tick >= o.latestReceivedTick {
        o.speed = observerLiveSpeed
        return
    }
    lag := int(o.latestReceivedTick) - int(o.state.Tick)
    if o.speed == observerLiveSpeed && lag > observerCatchUpLagTicks {
        o.speed = observerCatchUpSpeed   // 많이 뒤처짐 → 4배속 빨리 감기
        return
    }
    if o.speed == observerCatchUpSpeed && lag <= observerLiveLagTicks {
        o.speed = observerLiveSpeed       // 거의 따라잡음 → 실시간 속도
    }
}
```

```go
// 서버: 경기 내내 쌓은 입력 로그를 틱 순서대로 .rep 파일로 저장 (상태가 아니라 입력만)
func saveReplay(room *Room) {
    ticks := make([]uint32, 0, len(room.Recorder))
    for tick := range room.Recorder {
        ticks = append(ticks, tick)
    }
    sort.Slice(ticks, func(i, j int) bool { return ticks[i] < ticks[j] })

    payload := append([]byte(nil), []byte("PONGREP1")...)               // 매직 헤더
    payload = binary.BigEndian.AppendUint32(payload, uint32(room.ID))
    payload = binary.BigEndian.AppendUint32(payload, uint32(len(ticks)))

    for _, tick := range ticks {
        frame := room.Recorder[tick]
        payload = binary.BigEndian.AppendUint32(payload, tick)
        payload = append(payload, byte(int8(frame.P1)), byte(int8(frame.P2)))
        payload = append(payload, boolByte(frame.P1Ready), boolByte(frame.P2Ready))
    }
    os.WriteFile(filepath.Join("replay", fmt.Sprintf("room_%d.rep", room.ID)), payload, 0644)
}
```

</details>

---

## 공정성 문제, 그리고 2XKO의 방식

게임은 완성됐습니다. 하지만 가장 중요한 문제가 남아 있었습니다.

> **문제 ②** — 서버가 게임 상태를 소유하지 않는다. 그렇다면 한쪽 클라이언트가 "내가 이겼다"고 조작된 결과를 보내도, **서버는 그게 거짓인지 알 방법이 없다.** P2P 락스텝은 반응성과 비용은 잡았지만, 결과의 공정성에 구멍이 있었다.

처음엔 결정론만으로 충분하다고 생각했습니다. 같은 입력이면 같은 결과이고, 입력 로그를 저장하니 나중에 재생해 확인할 수 있으니까요. 하지만 이건 **사후에 사람이 다시 돌려봐야** 하는 방식이고, 서버가 스스로 "이 결과가 맞다/틀리다"를 판정하지는 못합니다.

이 지점에서 **[2XKO가 온라인 플레이를 다루는 방식](https://2xko.riotgames.com/ko-kr/news/dev/how-2xko-handles-online-play/)** 을 참고했습니다. 클라이언트끼리는 빠르게 플레이하되, **서버가 별도로 게임을 검증**해 신뢰를 확보하는 접근입니다.

**적용 · 서버가 입력으로 경기를 독립 재현한다**
서버는 이미 관전·리플레이를 위해 **양쪽의 입력 로그를 전부 받고 있었습니다.** 그래서 여기에 검증을 얹는 건 자연스러웠습니다:

1. 게임 로직을 클라이언트/서버가 함께 쓰는 **공유 모듈(`pongsim`)로 추출** → 서버가 클라이언트와 **비트 단위로 동일한** 시뮬레이션을 돌릴 수 있음 (로직이 한 곳에만 존재하므로 두 시뮬레이션이 어긋날 수 없음)
2. 경기가 끝나면 서버가 기록된 입력만으로 경기를 **처음부터 재시뮬레이션**해 정답(승자·점수·종료 틱)을 독립적으로 계산
3. 이 정답과 각 클라이언트가 제출한 `GameOverReport`를 **대조** → 불일치 시 desync/조작으로 판정하고 경고

**트레이드오프 · 서버가 게임을 계산하게 됐지만, 브로드캐스트 비용은 여전히 0**
이 검증을 위해 결국 **서버가 게임을 계산**하게 됐습니다 — 완전히 피하려던 그 비용의 일부가 돌아온 셈입니다. 하지만 결정적인 차이가 있습니다:

- 서버는 여전히 **매 틱 상태를 브로드캐스트하지 않습니다** → 실제 청구 비용의 주범인 **대역폭(egress)은 0 유지**
- 검증 sim은 경기 **종료 시점에 한 번**, 임계 경로 밖에서 실행 → **반응성에는 영향 없음**
- Pong의 sim은 정수 연산이라 계산 비용 자체가 매우 저렴

즉 **"비싼 브로드캐스트는 제거한 채로, 검증에 꼭 필요한 계산만"** 되살린 지점입니다.

<details>
<summary><b>서버 검증 시뮬레이션 — 코드로 보기</b></summary>

<br/>

```go
// 서버: 기록된 입력만으로 경기를 처음부터 재현해 "정답"을 계산 (클라이언트와 동일한 pongsim 코어)
func VerifyRoom(room *Room) VerifyResult {
    state := sim.NewState(sim.Player1)

    // 기록된 양쪽 입력을 커맨드 큐에 적재
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

    // 양쪽 입력이 준비된 틱만 순서대로 진행 (클라이언트와 동일한 규칙)
    for state.Winner == 0 && state.Tick <= maxTick {
        frame := state.CommandQueue[state.Tick]
        if frame == nil || !frame.P1Ready || !frame.P2Ready {
            break
        }
        state.Step(sim.Input{Player1: frame.P1Input, Player2: frame.P2Input})
    }

    return VerifyResult{
        Winner: state.Winner, LeftScore: state.LeftScore,
        RightScore: state.RightScore, GameOverTick: state.GameOverTick,
        Completed: state.Winner != 0,
    }
}

// 서버 정답과 클라이언트 보고가 어긋나면 불일치(조작 의심)로 판정
func compareReport(report GameOverReport, result VerifyResult) (reason string, ok bool) {
    if int32(result.Winner) != report.Winner {
        return "winner 불일치", false
    }
    if result.LeftScore != int(report.LeftScore) || result.RightScore != int(report.RightScore) {
        return "score 불일치", false
    }
    return "", true
}
```

</details>

---

## 아키텍처

지금까지의 조각들을 하나로 모은 전체 그림입니다.

- **로비 (TCP)** — 매칭, 방 관리, UDP 홀펀칭 중개. 신뢰성이 필요한 제어 신호만 처리
- **게임 (UDP P2P)** — 틱 단위 입력 교환, ACK/재전송. 저지연·저비용이 최우선
- **시뮬레이션 (`pongsim` 공유 모듈)** — 정수 연산 기반 결정론적 `Step()`. **클라이언트와 서버가 같은 코드를 실행**
- **검증·기록 (서버)** — 입력 로그로 경기를 독립 재현해 결과 대조, `.rep` 저장. 단 **상태 브로드캐스트는 하지 않음**

<p align="center">
  <img src="docs/server-role.png" width="560" alt="서버의 역할 — 매칭 / 검증 / 기록" />
</p>

**모듈 구조**

```
PONG/
├─ sim/       # pongsim — 결정론 시뮬레이션 코어 (클라·서버 공유)
├─ client/    # Ebiten 그래픽 클라이언트 (게임·리플레이·관전 씬)
└─ server/    # TCP 로비 + UDP 중개 + 검증/기록 서버
```

## 정리 — 두 마리 토끼는 잡혔나

| 목표 | 결과 | 어떻게 |
| :-- | :-- | :-- |
| **빠른 반응성** | ✅ | UDP P2P 직결(서버 홉 제거) + Input Delay로 대기 은닉 |
| **낮은 서버 비용** | ✅ (대역폭 0) | 상태 브로드캐스트 없음. 서버는 매칭·중개·검증만 |
| **결과의 공정성** | ✅ (뒤늦게 합류) | 서버가 입력으로 경기를 독립 재현해 결과 대조 |

각각의 해법은 **스타크래프트(락스텝), 스트리트 파이터 6(빨리 감기 관전), 2XKO(서버 검증)** 에서 가져와 하나의 서버 안에 접목한 것입니다. 새 기술의 발명이 아니라, 검증된 아이디어들을 조합했을 때 실제로 목표가 달성되는지 확인하는 것이 이 프로젝트의 목적이었습니다.


