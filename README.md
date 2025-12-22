# 🔫 Go P2P Lockstep Shooter

**Go**와 **Ebiten** 엔진으로 구현한 **1:1 P2P 멀티플레이어 슈팅 게임**입니다.
서버리스 구조로 UDP 홀펀칭(Hole Punching)을 통해 NAT 뒤에 있는 클라이언트끼리 직접 통신합니다.

네트워크 동기화 방식인 **결정론적 락스텝(Deterministic Lockstep)**과, UDP 위에서 데이터 유실을 방지하는 **Reliable UDP(ACK/Retransmission)** 프로토콜을 직접 구현했습니다.

---

## ✨ Key Features (핵심 기능)

### 1. 🌐 P2P Networking & NAT Traversal

* **UDP Hole Punching:** 중계 서버 없이 클라이언트끼리 직접 연결됩니다.
* **Signaling Server:** 초기 접속 시 상대방의 IP/Port를 교환하기 위한 아주 가벼운 시그널링 서버만 사용합니다.

### 2. 🔒 Deterministic Lockstep (결정론적 락스텝)

* **Input Sharing:** 유닛의 위치가 아닌 키 입력만 네트워크로 전송합니다.
* **Fixed-Point Math:** 부동소수점 오차(Floating Point Error)를 방지하기 위해 `float` 대신 `int` 기반의 **고정 소수점 연산(x1000)**을 사용하여 모든 클라이언트에서 100% 동일한 물리 연산을 보장합니다.
* **Input Delay:** 네트워크 레이턴시를 커버하기 위해 입력 후 `5 Ticks` 뒤에 실행되는 지연 처리 로직을 적용했습니다.

### 3. 🛡️ Reliable UDP (신뢰성 있는 UDP)

* **Custom Reliability Layer:** UDP의 빠른 속도와 TCP의 신뢰성을 결합했습니다.
* **SEQ & ACK:** 모든 패킷에 고유 번호(Seq)를 부여하고, 수신 확인(ACK)을 받습니다.
* **Automatic Retransmission:** 패킷 유실 시 별도의 고루틴이 백그라운드에서 자동으로 재전송하여 게임이 멈추거나 끊기지 않습니다.

### 4. ⚔️ Gameplay

* **Bullet Hell Physics:** 총알은 벽에 튕기며(Bounce), 네트워크 동기화 없이도 양쪽 화면에서 완벽하게 동일한 궤적을 그립니다.
* **Concurrency:** `Goroutines`와 `Channels`, `Mutex`를 활용하여 렌더링, 네트워크 수신, 재전송 로직을 병렬로 처리합니다.

| Player 1 | Player 2 |
| :---: | :---: |
| <img src="https://github.com/user-attachments/assets/2098bb22-f408-43f5-a93e-3ef57bc6c732" alt="p1" width="100%"> | <img src="https://github.com/user-attachments/assets/bf685b51-a2b4-4603-a759-a86b0921ab0e" alt="p2" width="100%"> |=
* 1분 이상 진행된 게임도 각 클라이언트에서 완벽하게 동일한 상태를 가집니다.

---

## 🛠️ Tech Stack

* **Language:** Go (Golang)
* **Game Engine:** [Ebiten v2](https://github.com/hajimehoshi/ebiten)
* **Protocol:** UDP (Custom Reliable Protocol on top of UDP)
* **Format:** JSON (Packet Serialization)

---

## 🎮 Controls

| Key | Action |
| --- | --- |
| **Right Click** | Move to cursor (이동) |
| **Spacebar** | Shoot (발사) |

---

## 🧩 Architecture Overview

### Packet Structure

패킷은 `Command` 구조체에 담겨 JSON으로 직렬화되어 전송됩니다.

```go
type Command struct {
    PlayerIdx int // 플레이어 식별 (1 or 2)
    ExecTick  int // 실행될 미래 시간 (Lockstep)
    Action    int // 행동 비트마스크 (Move | Shoot)
    DestX, Y  int // 목표 좌표
    Seq       int // 패킷 순서 번호 (Reliability)
}

```

### Synchronization Flow

1. **Input:** 유저가 키를 입력하면 `CurrentTick + Delay`를 계산하여 패킷 생성.
2. **Send:** 패킷을 `PendingMap`에 저장하고 상대에게 전송.
3. **Receive:** 상대방 패킷을 받으면 `CommandQueue`에 저장하고 즉시 `ACK` 발송.
4. **Execute:** `CurrentTick`에 해당하는 두 플레이어의 명령이 모두 도착했는지 확인 후 물리 엔진 업데이트. (하나라도 없으면 대기 - **Lockstep**)
