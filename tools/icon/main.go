package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/tools/appbundle"
	"github.com/vivaui/viva/tools/iconcompile"
	"github.com/vivaui/viva/ui"
)

func main() {
	appPath := flag.String("app", "", "also install the original PNG renditions into a packaged macOS app")
	flag.Parse()
	if err := generateIcons(*appPath); err != nil {
		log.Fatal(err)
	}
}

func generateIcons(appPath string) error {
	files, err := filepath.Glob("assets/icons/*.svg")
	if err != nil {
		return err
	}
	icons := make(map[string]ui.Icon, len(files)+2)
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".svg")
		icon, err := iconcompile.Compile(name, source)
		if err != nil {
			return err
		}
		icon.Stroke = 4.0 / 3.0
		icons[name] = icon
		if name == "up" {
			for direction, angle := range map[string]int{"left": -90, "right": 90} {
				content := string(source)
				start, end := strings.IndexByte(content, '>'), strings.LastIndex(content, "</svg>")
				if start < 0 || end <= start {
					return fmt.Errorf("%s: missing SVG root", path)
				}
				rotated := fmt.Sprintf("<svg viewBox=\"0 0 16 16\" fill=\"none\"><g transform=\"rotate(%d 8 8)\">%s</g></svg>", angle, content[start+1:end])
				icons[direction], err = iconcompile.Compile(direction, []byte(rotated))
				if err != nil {
					return err
				}
				rotatedIcon := icons[direction]
				rotatedIcon.Stroke = 4.0 / 3.0
				icons[direction] = rotatedIcon
			}
		}
	}
	generated, err := iconcompile.Generate("figmaicons", icons, "")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("internal/figmaicons", 0755); err != nil {
		return err
	}
	if err := os.WriteFile("internal/figmaicons/icons_generated.go", generated, 0644); err != nil {
		return err
	}
	if appPath != "" {
		icon, err := originalICNS("assets")
		if err != nil {
			return err
		}
		path := filepath.Join(appPath, "Contents", "Resources", appbundle.IconFileName)
		if err := os.WriteFile(path, icon, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("Compiled %d Figma icons; application PNG originals are unchanged.\n", len(icons))
	return nil
}

func originalICNS(sourceDir string) ([]byte, error) {
	originals := make(map[int][]byte, 4)
	for _, size := range []int{32, 64, 128, 1024} {
		name := fmt.Sprintf("vivapad-icon-%d.png", size)
		if size == 1024 {
			name = "vivapad-icon.png"
		}
		path := filepath.Join(sourceDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if config.Width != size || config.Height != size {
			return nil, fmt.Errorf("%s: expected %d×%d PNG", path, size, size)
		}
		originals[size] = data
	}
	master, err := png.Decode(bytes.NewReader(originals[1024]))
	if err != nil {
		return nil, err
	}
	encoded, _, err := appbundle.EncodeICNS(master)
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	for offset := 8; offset < len(encoded); {
		length := int(binary.BigEndian.Uint32(encoded[offset+4 : offset+8]))
		kind := string(encoded[offset : offset+4])
		payload := encoded[offset+8 : offset+length]
		size := map[string]int{"ic11": 32, "ic12": 64, "ic07": 128, "ic10": 1024}[kind]
		if original, ok := originals[size]; ok {
			payload = original
		}
		body.WriteString(kind)
		binary.Write(&body, binary.BigEndian, uint32(len(payload)+8))
		body.Write(payload)
		offset += length
	}
	var result bytes.Buffer
	result.WriteString("icns")
	binary.Write(&result, binary.BigEndian, uint32(body.Len()+8))
	result.Write(body.Bytes())
	return result.Bytes(), nil
}
