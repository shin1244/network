package main

import (
	"log"

	"pong/scene/lobby"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	current ebiten.Game
}

const (
	ScreenWidth  = 640
	ScreenHeight = 480
)

func main() {
	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("P2P Pong - Lobby")

	g := NewGame()
	if err := ebiten.RunGame(g.current); err != nil {
		log.Fatal(err)
	}
}

func NewGame() *Game {
	return &Game{
		current: lobby.NewLobby(),
	}
}
