package lobby

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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

func NewLobby() *Lobby {
	l := &Lobby{
		JoinBtnIndex: -1,
		Page:         0,
		RefreshBtn:   Button{X: 50, Y: 400, W: 120, H: 40, Text: "Refresh"},
		CreateBtn:    Button{X: 470, Y: 400, W: 120, H: 40, Text: "Create Room"},
		PrevBtn:      Button{X: 220, Y: 400, W: 80, H: 40, Text: "<"},
		NextBtn:      Button{X: 320, Y: 400, W: 80, H: 40, Text: ">"},
	}
	l.loadMockRooms()
	return l
}

func (l *Lobby) loadMockRooms() {
	l.Rooms = []Room{
		{ID: 1, Name: "Room 1 (TEST)", PlayerCnt: 1},
		{ID: 2, Name: "Room 2 (TEST)", PlayerCnt: 2},
		{ID: 3, Name: "Room 3 (TEST)", PlayerCnt: 1},
		{ID: 4, Name: "Room 4 (TEST)", PlayerCnt: 1},
		{ID: 5, Name: "Room 5 (TEST)", PlayerCnt: 1},
		{ID: 6, Name: "Room 6 (TEST)", PlayerCnt: 1},
		{ID: 7, Name: "Room 7 (TEST)", PlayerCnt: 1},
		{ID: 8, Name: "Room 8 (TEST)", PlayerCnt: 1},
	}
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
			l.loadMockRooms()

		} else if l.CreateBtn.Hovered {

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
				fmt.Printf("[%s] 입장\n", room.Name)
			}
		}
	}

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
