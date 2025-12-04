package scene

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

type Match struct {
	msg     []string
	msgChan chan []byte
}

func NewMatch() *Match {
	return &Match{
		msg:     []string{},
		msgChan: make(chan []byte, 1024),
	}
}

func (m *Match) Update(ctx GameContext) (Scene, error) {
	return m, nil
}

func (m *Match) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 0, 64, 255})
}

func (m *Match) MsgChan() chan []byte {
	return m.msgChan
}
