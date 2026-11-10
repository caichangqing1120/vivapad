package assets

import (
	_ "embed"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/app/iconpng"
)

//go:embed vivapad-icon.png
var iconPNG []byte

func WindowIcon() app.WindowIcon {
	icon, err := iconpng.Decode(iconPNG)
	if err != nil {
		panic(err)
	}
	return icon
}
