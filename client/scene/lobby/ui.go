package lobby

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Button struct {
	X, Y, W, H int
	Text       string
	Hovered    bool
	Visible    bool
}

func NewButton(x, y, w, h int, text string) *Button {
	return &Button{X: x, Y: y, W: w, H: h, Text: text, Visible: true}
}

func (b *Button) Update(mouseX, mouseY int) {
	if !b.Visible {
		b.Hovered = false
		return
	}
	b.Hovered = b.IsMouseOver(mouseX, mouseY)
}

func (b *Button) IsMouseOver(mouseX, mouseY int) bool {
	return mouseX >= b.X && mouseX <= b.X+b.W && mouseY >= b.Y && mouseY <= b.Y+b.H
}

func (b *Button) Draw(screen *ebiten.Image) {
	if !b.Visible {
		return
	}
	btnColor := color.RGBA{100, 100, 200, 255}
	if b.Hovered {
		btnColor = color.RGBA{150, 150, 250, 255}
	}
	vector.FillRect(screen, float32(b.X), float32(b.Y), float32(b.W), float32(b.H), btnColor, false)
	ebitenutil.DebugPrintAt(screen, b.Text, b.X+18, b.Y+12)
}

func (l *Lobby) drawLobby(screen *ebiten.Image) {
	screen.Fill(color.RGBA{30, 30, 30, 255})
	ebitenutil.DebugPrintAt(screen, "--- P2P PONG LOBBY ---", 250, 20)

	start, end := l.getPageRange()
	for i, room := range l.Rooms[start:end] {
		realIndex := start + i
		roomY := 80 + (i * 60)

		rectColor := color.RGBA{60, 60, 60, 255}
		if realIndex == l.JoinBtnIndex {
			rectColor = color.RGBA{90, 90, 90, 255}
		}

		vector.FillRect(screen, 50, float32(roomY), 540, 50, rectColor, false)

		info := fmt.Sprintf("[%d] %s  |  Players: %d/%d", room.ID, room.Name, room.PlayerCnt, MaxPlayers)
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

	l.RefreshBtn.Draw(screen)
	l.CreateBtn.Draw(screen)
	l.PrevBtn.Draw(screen)
	l.NextBtn.Draw(screen)
	l.CreateDialog.Draw(screen)
}
