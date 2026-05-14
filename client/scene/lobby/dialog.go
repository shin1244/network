package lobby

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type CreateRoomDialog struct {
	IsOpen     bool
	InputText  string
	ErrorText  string
	ConfirmBtn *Button
	CancelBtn  *Button
	OnConfirm  func(title string) error
}

func NewCreateRoomDialog(onConfirm func(string) error) *CreateRoomDialog {
	return &CreateRoomDialog{
		ConfirmBtn: NewButton(220, 292, 90, 36, "OK"),
		CancelBtn:  NewButton(330, 292, 90, 36, "Cancel"),
		OnConfirm:  onConfirm,
	}
}

func (d *CreateRoomDialog) Open() {
	d.IsOpen = true
	d.InputText = ""
	d.ErrorText = ""
}

func (d *CreateRoomDialog) Close() {
	d.IsOpen = false
	d.InputText = ""
	d.ErrorText = ""
}

func (d *CreateRoomDialog) Update(mouseX, mouseY int) {
	if !d.IsOpen {
		return
	}

	d.ConfirmBtn.Update(mouseX, mouseY)
	d.CancelBtn.Update(mouseX, mouseY)

	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 32 && r <= 126 {
			if len(d.InputText)+1 <= MaxRoomTitleBytes {
				d.InputText += string(r)
				d.ErrorText = ""
			} else {
				d.ErrorText = "Title must be under 255 bytes."
			}
		} else if r > 126 {
			d.ErrorText = "English letters only."
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(d.InputText) > 0 {
		d.InputText = d.InputText[:len(d.InputText)-1]
		d.ErrorText = ""
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && d.CancelBtn.Hovered {
		d.Close()
		return
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && d.ConfirmBtn.Hovered {
		title := strings.TrimSpace(d.InputText)
		if title == "" {
			d.ErrorText = "Enter a room title."
		} else if len(title) > MaxRoomTitleBytes {
			d.ErrorText = "Title must be under 255 bytes."
		} else if err := d.OnConfirm(title); err != nil {
			d.ErrorText = "Failed to create room."
		} else {
			d.Close()
		}
	}
}

func (d *CreateRoomDialog) Draw(screen *ebiten.Image) {
	if !d.IsOpen {
		return
	}

	vector.FillRect(screen, 0, 0, 640, 480, color.RGBA{0, 0, 0, 150}, false)
	vector.FillRect(screen, 150, 150, 340, 200, color.RGBA{45, 45, 55, 255}, false)
	vector.StrokeRect(screen, 150, 150, 340, 200, 2, color.RGBA{180, 180, 220, 255}, false)

	ebitenutil.DebugPrintAt(screen, "Room Title", 275, 178)
	ebitenutil.DebugPrintAt(screen, "English, under 255 bytes", 216, 204)

	vector.FillRect(screen, 190, 232, 260, 32, color.RGBA{25, 25, 30, 255}, false)
	vector.StrokeRect(screen, 190, 232, 260, 32, 1, color.RGBA{160, 160, 190, 255}, false)

	displayText := d.InputText
	if displayText == "" {
		displayText = "_"
	}
	ebitenutil.DebugPrintAt(screen, displayText, 200, 244)

	if d.ErrorText != "" {
		ebitenutil.DebugPrintAt(screen, d.ErrorText, 200, 270)
	}

	d.ConfirmBtn.Draw(screen)
	d.CancelBtn.Draw(screen)
}
