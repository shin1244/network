package main

import (
	"encoding/binary"
	"fmt"
	"log"

	"pong/network"
	"pong/scene/game"
	"pong/scene/lobby"

	"github.com/hajimehoshi/ebiten/v2"
)

type SceneManager struct {
	current ebiten.Game

	client *network.Client

	clientID int32
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

	m := &SceneManager{
		client: client,
	}

	l := lobby.JoinLobby(client.WritePacket, m.ChangeScene)

	m.current = l
	return m
}

type serverEventHandler interface {
	HandleServerEvent(network.Event)
}

func (g *SceneManager) handleServerEvents() {
	for {
		select {
		case event := <-g.client.Events:
			if event.Data == nil {
				log.Println("server connection closed")
				return
			}

			if g.handleGlobalServerEvent(event) {
				continue
			}

			handler, ok := g.current.(serverEventHandler)
			if ok {
				handler.HandleServerEvent(event)
			}

		default:
			return
		}
	}
}

func (g *SceneManager) ChangeScene(sceneID int, data []byte) {
	if sceneID == 1 {
		g.current = game.NewGameScene(g.client.WritePacket, g.ChangeScene, g.clientID, data) // 게임 씬으로 전환
	} else {
		log.Printf("Unknown scene command: %d", sceneID)
	}
}

// 서버에 처음 접속했을 때 클라이언트 ID를 받는 이벤트를 처리하는 함수
func (g *SceneManager) handleGlobalServerEvent(event network.Event) bool {
	if event.Scene == 0 && event.SceneCommand == byte(3) {
		if len(event.Data) < 4 {
			log.Printf("invalid client id payload: %v", event.Data)
			return true
		}

		g.clientID = int32(binary.BigEndian.Uint32(event.Data))
		fmt.Printf("Received client ID: %d\n", g.clientID)
		return true
	}

	return false
}
