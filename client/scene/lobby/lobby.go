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
)

const (
	MaxPage           = 5
	MaxPlayers        = 2
	MaxRoomTitleBytes = 254
)

type Lobby struct {
	Rooms        []Room
	Page         int
	RefreshBtn   *Button
	CreateBtn    *Button
	PrevBtn      *Button
	NextBtn      *Button
	JoinBtnIndex int

	CreateDialog *CreateRoomDialog

	Client *network.Client

	OnChangeScene func(data []byte)
}

type Room struct {
	ID        int
	Name      string
	PlayerCnt int
}

func NewLobby(client *network.Client) *Lobby {
	l := &Lobby{
		JoinBtnIndex: -1,
		Page:         0,
		RefreshBtn:   NewButton(50, 400, 120, 40, "Refresh"),
		CreateBtn:    NewButton(470, 400, 120, 40, "Create Room"),
		PrevBtn:      NewButton(220, 400, 80, 40, "<"),
		NextBtn:      NewButton(320, 400, 80, 40, ">"),
		Client:       client,
	}

	l.CreateDialog = NewCreateRoomDialog(func(title string) error {
		return l.sendCreateRoom(title)
	})

	if err := l.sendRefreshRooms(); err != nil {
		log.Printf("failed to refresh rooms: %v", err)
	}

	go l.HandleServerEvent()

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
	l.PrevBtn.Update(mouseX, mouseY)
	l.NextBtn.Update(mouseX, mouseY)
	l.updateRoomHover(mouseX, mouseY)

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
	if end > len(l.Rooms) {
		end = len(l.Rooms)
	}
	return
}

func (l *Lobby) updateRoomHover(mouseX, mouseY int) {
	l.JoinBtnIndex = -1

	start, end := l.getPageRange()
	for i := start; i < end; i++ {
		roomY := 80 + ((i - start) * 60)
		if mouseX >= 50 && mouseX <= 590 && mouseY >= roomY && mouseY <= roomY+50 {
			l.JoinBtnIndex = i
			return
		}
	}
}

func (l *Lobby) handleClick() {
	switch {
	case l.RefreshBtn.Hovered:
		if err := l.sendRefreshRooms(); err != nil {
			log.Printf("failed to refresh rooms: %v", err)
		}
	case l.CreateBtn.Hovered:
		l.CreateDialog.Open()
	case l.PrevBtn.Hovered:
		if l.Page > 0 {
			l.Page--
			l.JoinBtnIndex = -1
		}
	case l.NextBtn.Hovered:
		maxPage := (len(l.Rooms) - 1) / MaxPage
		if l.Page < maxPage {
			l.Page++
			l.JoinBtnIndex = -1
		}
	case l.JoinBtnIndex != -1:
		room := l.Rooms[l.JoinBtnIndex]
		if room.PlayerCnt < MaxPlayers {
			if err := l.sendJoinRoom(int32(room.ID)); err != nil {
				log.Printf("failed to join room: %v", err)
			}
			fmt.Printf("[%s] joined\n", room.Name)
		}
	}
}

// [Scene] [Command] [length: 0]
func (l *Lobby) sendRefreshRooms() error {
	packet := network.MakePacket(
		byte(0),
		byte(LobbyRefreshRooms),
		nil,
	)

	return l.Client.WritePacket(packet)
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

	return l.Client.WritePacket(packet)
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

	return l.Client.WritePacket(packet)
}

func (l *Lobby) handleRoomList(data []byte) {
	roomCount := int(data[0])
	offset := 1
	rooms := make([]Room, roomCount)
	for i := 0; i < roomCount; i++ {
		roomID := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4

		nameLen := int(data[offset])
		offset++

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

func (l *Lobby) HandleServerEvent() {
	for event := range l.Client.Events {
		if event.Data == nil {
			log.Println("server connection closed")
			return
		}

		scene := event.Scene
		cmd := event.SceneCommand

		if scene != byte(0) {
			return
		}

		switch cmd {
		case byte(LobbyRefreshRooms):
			l.handleRoomList(event.Data)
		case byte(LobbyJoinRoom):
			fmt.Println(event.Data)
			l.OnChangeScene(event.Data)
		}
	}
}
