# 🎮 Go Game Network Practice

Go 언어와 Ebiten 엔진을 사용하여 구현한 **TCP/UDP 하이브리드 게임 서버/클라이언트** 연습 프로젝트입니다.
대기실(Lobby)과 매치메이킹은 **TCP**로, 인게임 데이터 전송은 **UDP Relay** 방식을 사용하여 구현했습니다.

## 🚀 Key Features

### 1. Hybrid Architecture (TCP + UDP)
- **TCP (Port 9909):** 안정성이 중요한 로비 입장, 채팅, 매치메이킹 요청 처리.
- **UDP (Port 8888):** 반응속도가 중요한 인게임 패킷 릴레이 (Deterministic Lockstep 준비).
- **Goroutine & Channel:** 각 연결을 별도의 고루틴으로 처리하고, 채널을 통해 메시지를 브로드캐스트하여 동시성 제어.

### 2. Matchmaking System
- **`sync.Cond` 기반 대기열:** 불필요한 폴링(Polling) 없이, 대기자가 2명이 모일 때까지 CPU 자원을 쓰지 않고 대기(Wait)하다가 신호(Signal)가 오면 매칭 성사.
- **Toggle 방식:** 매칭 요청 시 대기열에 추가하고, 다시 요청 시 취소하는 토글 로직 구현.
- **Deadlock Prevention:** `sync.Mutex`를 사용하여 동시 접근 시 발생할 수 있는 데이터 레이스와 데드락 방지.

### 3. Client (Ebiten)
- **Scene Management:** `Lobby`와 `Game` 씬을 분리하고, 인터페이스(`GameContext`)를 통해 순환 참조(Import Cycle) 없이 서버 통신 로직 구현.
- **Non-blocking Network:** 네트워크 수신 루프와 게임 렌더링 루프(`Update/Draw`)를 분리하여 끊김 없는 화면 처리.

---

## 🛠 Tech Stack

- **Language:** Go (Golang)
- **Game Library:** [Ebiten v2](https://github.com/hajimehoshi/ebiten)
- **Protocol:** TCP, UDP
- **Architecture:** Client-Server (Relay)

---

## 📂 Project Structure

```bash
├── client/          # 클라이언트 코드
│   ├── scene/       # 로비, 게임 씬 로직 (Lobby, GameScene)
│   └── main.go      # 클라이언트 진입점 (Ebiten 실행)
├── server/          # 서버 코드
│   ├── match/       # 매치메이킹 큐 로직 (sync.Cond 사용)
│   ├── users/       # 접속 유저 관리 (Thread-safe Map)
│   └── main.go      # 서버 진입점 (TCP/UDP 리스너)
└── README.md
