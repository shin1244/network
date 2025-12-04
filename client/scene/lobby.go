package scene

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Lobby struct {
	msg       []string
	msgChan   chan []byte
	inputText string
}

func NewLobby() *Lobby {
	return &Lobby{
		msg:     []string{},
		msgChan: make(chan []byte, 1024),
	}
}

func (l *Lobby) Update(ctx GameContext) (Scene, error) {
	select {
	case m := <-l.msgChan:
		switch m[0] {
		case MsgMatch:
			fmt.Println("Switching to Match scene")
			return NewMatch(), nil
		case MsgChat:
			l.msg = append(l.msg, string(m[1:]))
			fmt.Println("Lobby received chat message:", string(m[1:]))
			if len(l.msg) > 20 {
				l.msg = l.msg[1:]
			}
		}

	default:
	}

	l.inputText += string(ebiten.AppendInputChars(nil))
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if len(l.inputText) > 0 {
			l.inputText = l.inputText[:len(l.inputText)-1]
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if len(l.inputText) > 0 {
			payload := append([]byte{MsgChat}, []byte(l.inputText+"\n")...)
			ctx.Send(payload)

			l.inputText = ""
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		ctx.Send([]byte{MsgMatch})
	}

	return l, nil
}

func (l *Lobby) Draw(screen *ebiten.Image) {
	// 사용법 안내
	ebitenutil.DebugPrintAt(screen, "[Space]: Match Request  [Enter]: Send Chat", 10, 10)

	// 채팅 로그 출력
	y := 30
	for _, m := range l.msg {
		ebitenutil.DebugPrintAt(screen, m, 10, y)
		y += 15
	}

	// 입력창 출력 (깜빡이는 커서 효과)
	inputDisplay := "Input: " + l.inputText + "_"
	ebitenutil.DebugPrintAt(screen, inputDisplay, 10, 450) // 화면 아래쪽
}

func (l *Lobby) MsgChan() chan []byte {
	return l.msgChan
}
