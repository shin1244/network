package scene

import "github.com/hajimehoshi/ebiten/v2"

type Scene interface {
	Update(g GameContext) (Scene, error)
	Draw(screen *ebiten.Image)
	MsgChan() chan []byte
}

type GameContext interface {
	Send(data []byte) error
}

const (
	MsgChat  uint8 = 1
	MsgMatch uint8 = 2
)
