package main

import (
	"log"

	"pong/network"
	"pong/scene/lobby"

	"github.com/hajimehoshi/ebiten/v2"
)

type SceneManager struct {
	current ebiten.Game

	client *network.Client
}

func (g *SceneManager) Update() error {
	return g.current.Update()
}

func (g *SceneManager) Draw(screen *ebiten.Image) {
	g.current.Draw(screen)
}

func (g *SceneManager) Layout(w, h int) (int, int) {
	return g.current.Layout(w, h)
}

const (
	ScreenWidth  = 640
	ScreenHeight = 480
)

func main() {
	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("P2P Pong - Lobby")

	sm := NewSceneManager()
	go sm.client.Readloop()

	if err := ebiten.RunGame(sm); err != nil {
		log.Fatal(err)
	}
}

func NewSceneManager() *SceneManager {
	client := network.NewClient("localhost:9000")
	if err := client.Connect(); err != nil {
		log.Printf("failed to connect to server: %v", err)
	}
	l := lobby.NewLobby(client)
	l.OnChangeScene = func(scene []byte) {
		if scene[0] == 1 {
			log.Println("Switching to game scene (not implemented)")
		} else {
			log.Printf("Unknown scene command: %d", scene[1])
		}
	}

	return &SceneManager{
		current: l,
		client:  client,
	}
}
