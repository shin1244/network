package main

import (
	"bufio"
	"client/scene"
	"net"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	conn   net.Conn
	reader *bufio.Reader
	scene  scene.Scene
}

const (
	MsgChat  uint8 = 1
	MsgMatch uint8 = 2
)

type Scene interface {
	Update(g ebiten.Game) (scene.Scene, error)
	Draw(screen *ebiten.Image)
}

func (g *Game) Update() error {
	next, err := g.scene.Update(g)
	if err != nil {
		return err
	}
	g.scene = next

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.scene.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 320, 240
}

func (g *Game) Send(data []byte) error {
	_, err := g.conn.Write(data)
	return err
}

func main() {
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Ebiten Example")

	game, err := NewGame()
	if err != nil {
		panic(err)
	}

	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}

func NewGame() (*Game, error) {
	conn, err := net.Dial("tcp", "localhost:9909")
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	game := &Game{
		conn:   conn,
		reader: reader,
		scene:  scene.NewLobby(),
	}
	go game.handleServerMessage(reader)
	return game, nil
}

func (g *Game) handleServerMessage(reader *bufio.Reader) {
	for {
		t, _ := reader.ReadByte()

		switch t {
		case MsgChat:
			body, _ := reader.ReadBytes('\n')
			g.scene.MsgChan() <- append([]byte{t}, body...)
		case MsgMatch:
			g.scene.MsgChan() <- []byte{t}
		}
	}
}
