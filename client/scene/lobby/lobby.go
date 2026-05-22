package lobby

import (
	"encoding/binary"
	"fmt"
	"log"
	"pong/network"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type LobbyCommand byte

const (
	LobbyCreateRoom LobbyCommand = iota
	LobbyJoinRoom
	LobbyRefreshRooms
	LobbyReplayList
	LobbyJoinReplay
	LobbyWatchRoom
	JoinGame
)

const (
	MaxPage           = 5
	MaxPlayers        = 2
	MaxRoomTitleBytes = 254

	watchButtonX = 500
	watchButtonW = 70
	watchButtonH = 30
)

type LobbyMode byte

const (
	LobbyModeRooms LobbyMode = iota
	LobbyModeReplays
)

type Lobby struct {
	Rooms         []Room
	Replays       []Replay
	Page          int
	RefreshBtn    *Button
	CreateBtn     *Button
	ReplayBtn     *Button
	PrevBtn       *Button
	NextBtn       *Button
	SelectedIndex int

	Mode LobbyMode

	CreateDialog *CreateRoomDialog

	sendPacket func([]byte) error

	OnChangeScene func(sceneID int, data []byte)
}

type Room struct {
	ID        int
	Name      string
	PlayerCnt int
}

type Replay struct {
	ID   int
	Name string
}

func JoinLobby(sendPacket func([]byte) error, onChangeScene func(sceneID int, data []byte)) *Lobby {
	l := &Lobby{
		SelectedIndex: -1,
		Mode:          LobbyModeRooms,
		Page:          0,
		RefreshBtn:    NewButton(50, 400, 120, 40, "Refresh"),
		CreateBtn:     NewButton(470, 400, 120, 40, "Create Room"),
		ReplayBtn:     NewButton(470, 350, 120, 40, "Replay"),
		PrevBtn:       NewButton(220, 400, 80, 40, "<"),
		NextBtn:       NewButton(320, 400, 80, 40, ">"),
		sendPacket:    sendPacket,
		OnChangeScene: onChangeScene,
	}

	l.CreateDialog = NewCreateRoomDialog(func(title string) error {
		return l.sendCreateRoom(title)
	})

	if err := l.sendRefreshRooms(); err != nil {
		log.Printf("failed to refresh rooms: %v", err)
	}

	return l
}

func (l *Lobby) Update() error {
	mouseX, mouseY := ebiten.CursorPosition()

	if l.CreateDialog.IsOpen {
		l.CreateDialog.Update(mouseX, mouseY)
		return nil
	}

	l.RefreshBtn.Update(mouseX, mouseY)
	l.CreateBtn.Update(mouseX, mouseY)
	l.ReplayBtn.Update(mouseX, mouseY)
	l.PrevBtn.Update(mouseX, mouseY)
	l.NextBtn.Update(mouseX, mouseY)
	l.updateListHover(mouseX, mouseY)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		l.handleClick()
	}

	return nil
}

func (l *Lobby) Draw(screen *ebiten.Image) {
	l.drawLobby(screen)
}

func (l *Lobby) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 640, 480
}

func (l *Lobby) getPageRange() (start, end int) {
	start = l.Page * MaxPage
	end = start + MaxPage
	itemCount := l.currentListLen()
	if end > itemCount {
		end = itemCount
	}
	return
}

func (l *Lobby) currentListLen() int {
	if l.Mode == LobbyModeReplays {
		return len(l.Replays)
	}

	return len(l.Rooms)
}

func (l *Lobby) resetListView(mode LobbyMode) {
	l.Mode = mode
	l.Page = 0
	l.SelectedIndex = -1
}

func (l *Lobby) updateListHover(mouseX, mouseY int) {
	l.SelectedIndex = -1

	start, end := l.getPageRange()
	for i := start; i < end; i++ {
		itemY := 80 + ((i - start) * 60)
		if mouseX >= 50 && mouseX <= 590 && mouseY >= itemY && mouseY <= itemY+50 {
			l.SelectedIndex = i
			return
		}
	}
}

func (l *Lobby) handleClick() {
	switch {
	case l.RefreshBtn.Hovered:
		l.resetListView(LobbyModeRooms)
		if err := l.sendRefreshRooms(); err != nil {
			log.Printf("failed to refresh rooms: %v", err)
		}
	case l.CreateBtn.Hovered:
		l.resetListView(LobbyModeRooms)
		l.CreateDialog.Open()
	case l.ReplayBtn.Hovered:
		l.resetListView(LobbyModeReplays)
		l.sendReplayList()
	case l.PrevBtn.Hovered:
		if l.Page > 0 {
			l.Page--
			l.SelectedIndex = -1
		}
	case l.NextBtn.Hovered:
		maxPage := (l.currentListLen() - 1) / MaxPage
		if l.Page < maxPage {
			l.Page++
			l.SelectedIndex = -1
		}
	case l.SelectedIndex != -1:
		if l.Mode == LobbyModeReplays {
			l.handleReplayClick(l.SelectedIndex)
			return
		}

		room := l.Rooms[l.SelectedIndex]
		if room.PlayerCnt >= MaxPlayers {
			if l.isWatchButtonHovered(l.SelectedIndex) {
				if err := l.sendWatchRoom(int32(room.ID)); err != nil {
					log.Printf("failed to watch room: %v", err)
				}
				fmt.Printf("[%s] watch requested\n", room.Name)
			}
			return
		}

		if room.PlayerCnt < MaxPlayers {
			if err := l.sendJoinRoom(int32(room.ID)); err != nil {
				log.Printf("failed to join room: %v", err)
			}
			fmt.Printf("[%s] joined\n", room.Name)
		}
	}
}

func (l *Lobby) isWatchButtonHovered(index int) bool {
	if l.Mode != LobbyModeRooms || index < 0 || index >= len(l.Rooms) {
		return false
	}
	if l.Rooms[index].PlayerCnt < MaxPlayers {
		return false
	}

	mouseX, mouseY := ebiten.CursorPosition()
	_, buttonY := l.watchButtonPosition(index)
	return mouseX >= watchButtonX &&
		mouseX <= watchButtonX+watchButtonW &&
		mouseY >= buttonY &&
		mouseY <= buttonY+watchButtonH
}

func (l *Lobby) watchButtonPosition(index int) (x, y int) {
	start := l.Page * MaxPage
	return watchButtonX, 80 + ((index - start) * 60) + 10
}

func (l *Lobby) handleReplayClick(index int) {
	if index < 0 || index >= len(l.Replays) {
		return
	}

	replay := l.Replays[index]
	fmt.Printf("[%s] replay selected\n", replay.Name)

	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(replay.ID))
	packet := network.MakePacket(
		byte(0),
		byte(LobbyJoinReplay),
		payload,
	)

	if err := l.sendPacket(packet); err != nil {
		log.Printf("failed to request replay: %v", err)
		return
	}
}

// [Scene] [Command] [length: 0]
func (l *Lobby) sendRefreshRooms() error {
	packet := network.MakePacket(
		byte(0),
		byte(LobbyRefreshRooms),
		nil,
	)

	return l.sendPacket(packet)
}

// [Scene] [Command] [nameLenght] [name]
func (l *Lobby) sendCreateRoom(name string) error {
	if len(name) == 0 || len(name) > MaxRoomTitleBytes {
		return fmt.Errorf(
			"room name must be between 1 and %d bytes",
			MaxRoomTitleBytes,
		)
	}

	payload := append(
		[]byte{byte(len(name))},
		[]byte(name)...,
	)

	packet := network.MakePacket(
		byte(0),
		byte(LobbyCreateRoom),
		payload,
	)

	return l.sendPacket(packet)
}

// [Scene] [Command] [length] [roomID]
func (l *Lobby) sendJoinRoom(roomID int32) error {
	payload := make([]byte, 4)

	binary.BigEndian.PutUint32(payload, uint32(roomID))

	packet := network.MakePacket(
		byte(0),
		byte(LobbyJoinRoom),
		payload,
	)

	return l.sendPacket(packet)
}

func (l *Lobby) sendWatchRoom(roomID int32) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(roomID))

	packet := network.MakePacket(
		byte(0),
		byte(LobbyWatchRoom),
		payload,
	)

	return l.sendPacket(packet)
}

func (l *Lobby) handleRoomList(data []byte) {
	if len(data) == 0 {
		l.Rooms = nil
		return
	}

	roomCount := int(data[0])
	offset := 1
	rooms := make([]Room, roomCount)
	for i := 0; i < roomCount; i++ {
		if offset+5 > len(data) {
			log.Printf("invalid room list payload: %v", data)
			break
		}

		roomID := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4

		nameLen := int(data[offset])
		offset++

		if offset+nameLen+1 > len(data) {
			log.Printf("invalid room list payload: %v", data)
			break
		}

		roomName := string(data[offset : offset+nameLen])
		offset += nameLen

		playerCnt := int(data[offset])
		offset++

		rooms[i] = Room{
			ID:        roomID,
			Name:      roomName,
			PlayerCnt: playerCnt,
		}
	}
	l.Rooms = rooms
}

func (l *Lobby) handleReplayList(data []byte) {
	if len(data) == 0 {
		l.Replays = nil
		return
	}

	replayCount := int(data[0])
	offset := 1
	replays := make([]Replay, replayCount)
	for i := 0; i < replayCount; i++ {
		if offset+5 > len(data) {
			log.Printf("invalid replay list payload: %v", data)
			break
		}

		replayID := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4

		nameLen := int(data[offset])
		offset++

		if offset+nameLen > len(data) {
			log.Printf("invalid replay list payload: %v", data)
			break
		}

		replayName := string(data[offset : offset+nameLen])
		offset += nameLen

		replays[i] = Replay{
			ID:   replayID,
			Name: replayName,
		}
	}
	l.Replays = replays
}

func (l *Lobby) HandleServerEvent(event network.Event) {
	scene := event.Scene
	cmd := event.SceneCommand

	if scene != byte(0) {
		return
	}

	switch cmd {
	case byte(LobbyRefreshRooms):
		l.resetListView(LobbyModeRooms)
		l.handleRoomList(event.Data)
	case byte(LobbyJoinRoom):
		fmt.Println(event.Data)
		l.OnChangeScene(1, event.Data)
	case byte(LobbyReplayList):
		l.resetListView(LobbyModeReplays)
		l.handleReplayList(event.Data)
	case byte(LobbyJoinReplay):
		l.OnChangeScene(2, event.Data)
	case byte(LobbyWatchRoom):
		log.Printf("Received watch room response: %v", event.Data)
		l.OnChangeScene(3, event.Data)
	}
}

func (l *Lobby) sendReplayList() {
	packet := network.MakePacket(
		byte(0),
		byte(LobbyReplayList),
		nil,
	)

	if err := l.sendPacket(packet); err != nil {
		log.Printf("failed to request replay list: %v", err)
	}
}
