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
	g.handleServerEvents()
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
	if sm.client.IsConnected() {
		go sm.client.Readloop()
	}

	if err := ebiten.RunGame(sm); err != nil {
		log.Fatal(err)
	}
}

func NewSceneManager() *SceneManager {
	client := network.NewClient("localhost:9000")
	if err := client.Connect(); err != nil {
		log.Printf("failed to connect to server: %v", err)
	}

	gameScene := &SceneManager{
		client: client,
	}

	l := lobby.JoinLobby(client.WritePacket, gameScene.ChangeScene)

	gameScene.current = l
	return gameScene
}

type serverEventHandler interface {
	HandleServerEvent(network.Event)
}

func (g *SceneManager) handleServerEvents() {
	handler, ok := g.current.(serverEventHandler) // server event handler가 현재 씬에서 구현되어 있는지 확인
	if !ok {
		return
	}

	for {
		select {
		case event := <-g.client.Events:
			if event.Data == nil {
				log.Println("server connection closed")
				return
			}
			handler.HandleServerEvent(event)
		default:
			return
		}
	}
}

func (g *SceneManager) ChangeScene(sceneID int, data []byte) {
	if sceneID == 1 {
		// g.current = game.NewGameScene(g.client.WritePacket, g.ChangeScene) // 게임 씬으로 전환
	} else {
		log.Printf("Unknown scene command: %d", sceneID)
	}
}
