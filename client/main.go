package main

import (
	"log"

	"pong/network"
	"pong/scene/lobby"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	current ebiten.Game

	client *network.Client
}

func (g *Game) Update() error {
	return g.current.Update()
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.current.Draw(screen)
}

func (g *Game) Layout(w, h int) (int, int) {
	return g.current.Layout(w, h)
}

const (
	ScreenWidth  = 640
	ScreenHeight = 480
)

func main() {
	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("P2P Pong - Lobby")

	g := NewGame()
	go g.client.Readloop()
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func NewGame() *Game {
	client := network.NewClient("localhost:9000")
	if err := client.Connect(); err != nil {
		log.Printf("failed to connect to server: %v", err)
	}

	return &Game{
		current: lobby.NewLobby(client),
		client:  client,
	}
}
