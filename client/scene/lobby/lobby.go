package lobby

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"log"
	"pong/network"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type LobbyCommand byte

const (
	LobbyCreateRoom LobbyCommand = iota
	LobbyJoinRoom
	LobbyRefreshRooms
)

const (
	MaxPage    = 5
	MaxPlayers = 2
)

type Lobby struct {
	Rooms        []Room
	Page         int
	RefreshBtn   Button
	CreateBtn    Button
	PrevBtn      Button
	NextBtn      Button
	JoinBtnIndex int

	Client *network.Client
}

type Room struct {
	ID        int
	Name      string
	PlayerCnt int
}

type Button struct {
	X, Y, W, H int
	Text       string
	Hovered    bool
}

func (b *Button) IsMouseOver(mouseX, mouseY int) bool {
	return mouseX >= b.X && mouseX <= b.X+b.W && mouseY >= b.Y && mouseY <= b.Y+b.H
}

func (l *Lobby) getPageRange() (start, end int) {
	start = l.Page * MaxPage
	end = start + MaxPage

	if end > len(l.Rooms) {
		end = len(l.Rooms)
	}
	return
}

func NewLobby(client *network.Client) *Lobby {
	l := &Lobby{
		JoinBtnIndex: -1,
		Page:         0,
		RefreshBtn:   Button{X: 50, Y: 400, W: 120, H: 40, Text: "Refresh"},
		CreateBtn:    Button{X: 470, Y: 400, W: 120, H: 40, Text: "Create Room"},
		PrevBtn:      Button{X: 220, Y: 400, W: 80, H: 40, Text: "<"},
		NextBtn:      Button{X: 320, Y: 400, W: 80, H: 40, Text: ">"},

		Client: client,
	}
	return l
}

func (l *Lobby) Update() error {
	mouseX, mouseY := ebiten.CursorPosition()

	// 1. 버튼 호버(Hover) 상태 업데이트
	l.RefreshBtn.Hovered = l.RefreshBtn.IsMouseOver(mouseX, mouseY)
	l.CreateBtn.Hovered = l.CreateBtn.IsMouseOver(mouseX, mouseY)
	l.PrevBtn.Hovered = l.PrevBtn.IsMouseOver(mouseX, mouseY)
	l.NextBtn.Hovered = l.NextBtn.IsMouseOver(mouseX, mouseY)

	l.JoinBtnIndex = -1

	start, end := l.getPageRange()
	for i := start; i < end; i++ {
		roomY := 80 + ((i - start) * 60)
		if mouseX >= 50 && mouseX <= 590 && mouseY >= roomY && mouseY <= roomY+50 {
			l.JoinBtnIndex = i
			break
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {

		if l.RefreshBtn.Hovered {
			if err := l.refreshRooms(); err != nil {
				log.Printf("failed to refresh rooms: %v", err)
			}

		} else if l.CreateBtn.Hovered {
			if err := l.createRoom("Test Room"); err != nil {
				log.Printf("failed to create room: %v", err)
			}

		} else if l.PrevBtn.Hovered {
			if l.Page > 0 {
				l.Page--
				l.JoinBtnIndex = -1 // 선택 초기화
			}

		} else if l.NextBtn.Hovered {
			maxPage := (len(l.Rooms) - 1) / MaxPage
			if l.Page < maxPage {
				l.Page++
				l.JoinBtnIndex = -1
			}

		} else if l.JoinBtnIndex != -1 {
			room := l.Rooms[l.JoinBtnIndex]
			if room.PlayerCnt < MaxPlayers {
				if err := l.joinRoom(int32(room.ID)); err != nil {
					log.Printf("failed to join room: %v", err)
				}
				fmt.Printf("[%s] 입장\n", room.Name)
			}
		}
	}
	l.HandleServerEvent()

	return nil
}

func (l *Lobby) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{30, 30, 30, 255})
	ebitenutil.DebugPrintAt(screen, "--- P2P PONG LOBBY ---", 250, 20)

	start, end := l.getPageRange()

	if end > len(l.Rooms) {
		end = len(l.Rooms)
	}

	for i, room := range l.Rooms[start:end] {
		realIndex := start + i
		roomY := 80 + (i * 60)

		rectColor := color.RGBA{60, 60, 60, 255}

		// 현재 선택된 방 하이라이트
		if realIndex == l.JoinBtnIndex {
			rectColor = color.RGBA{90, 90, 90, 255}
		}

		// 방 카드 배경
		vector.FillRect(screen, 50, float32(roomY), 540, 50, rectColor, false)

		// 방 정보 텍스트
		info := fmt.Sprintf("[%d] %s  |  Players: %d/%d",
			room.ID, room.Name, room.PlayerCnt, MaxPlayers)

		if room.PlayerCnt >= MaxPlayers {
			info += " (FULL)"
		}

		ebitenutil.DebugPrintAt(screen, info, 70, roomY+18)
	}

	totalPage := (len(l.Rooms)-1)/MaxPage + 1
	if len(l.Rooms) == 0 {
		totalPage = 1
	}

	pageText := fmt.Sprintf("Page %d / %d", l.Page+1, totalPage)
	ebitenutil.DebugPrintAt(screen, pageText, 270, 360)

	drawButton := func(b Button) {
		btnColor := color.RGBA{100, 100, 200, 255}
		if b.Hovered {
			btnColor = color.RGBA{150, 150, 250, 255}
		}
		vector.FillRect(screen, float32(b.X), float32(b.Y), float32(b.W), float32(b.H), btnColor, false)
		ebitenutil.DebugPrintAt(screen, b.Text, b.X+20, b.Y+12)
	}

	drawButton(l.RefreshBtn)
	drawButton(l.CreateBtn)
	drawButton(l.PrevBtn)
	drawButton(l.NextBtn)
}

func (l *Lobby) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 640, 480
}

func (l *Lobby) refreshRooms() error {
	client := l.Client
	return client.WritePacket([]byte{byte(LobbyRefreshRooms)})
}

func (l *Lobby) createRoom(name string) error {
	packet := append([]byte{byte(LobbyCreateRoom)}, []byte(name)...)
	return l.Client.WritePacket(packet)
}

func (l *Lobby) joinRoom(roomID int32) error {
	packet := make([]byte, 5)
	packet[0] = byte(LobbyJoinRoom)
	binary.BigEndian.PutUint32(packet[1:5], uint32(roomID))
	return l.Client.WritePacket(packet)
}

func (l *Lobby) HandleServerEvent() {
	select {
	case event := <-l.Client.Events:
		fmt.Printf("서버 이벤트 수신: %v\n", event)
		if event.Data == nil {
			log.Println("서버와의 연결이 끊어졌습니다.")
			return
		}

		scene := event.Data[0]
		cmd := event.Data[1]

		if scene != byte(0) {
			return
		}

		switch cmd {
		case byte(LobbyRefreshRooms):
			roomCount := int(event.Data[2])
			rooms := make([]Room, roomCount)

			offset := 3
			for i := 0; i < roomCount; i++ {
				roomID := int(binary.BigEndian.Uint32(event.Data[offset : offset+4]))
				offset += 4
				roomNameLen := int(event.Data[offset])
				offset++
				roomName := string(event.Data[offset : offset+roomNameLen])
				offset += roomNameLen
				playerCnt := int(event.Data[offset])
				offset++

				rooms[i] = Room{
					ID:        roomID,
					Name:      roomName,
					PlayerCnt: playerCnt,
				}
			}
			l.Rooms = rooms
		}
	default:
		return
	}
}
