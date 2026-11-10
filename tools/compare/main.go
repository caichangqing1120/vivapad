package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"vivapad/internal/compare"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("vivapad-compare", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	referencePath := flags.String("reference", "", "original Notepad++ PNG")
	actualPath := flags.String("actual", "", "VivaPad PNG")
	diffPath := flags.String("diff", "", "output difference PNG")
	tolerance := flags.Int("tolerance", 0, "allowed per-channel delta (0-255)")
	maximum := flags.Float64("max-changed-percent", -1, "fail when more pixels differ (0-100)")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *referencePath == "" || *actualPath == "" || *diffPath == "" || flags.NArg() != 0 {
		return fmt.Errorf("required: -reference original.png -actual vivapad.png -diff difference.png")
	}
	if *tolerance < 0 || *tolerance > 255 || *maximum < -1 || *maximum > 100 {
		return fmt.Errorf("tolerance must be 0-255 and max-changed-percent must be -1 or 0-100")
	}
	reference, err := readPNG(*referencePath)
	if err != nil {
		return err
	}
	actual, err := readPNG(*actualPath)
	if err != nil {
		return err
	}
	report, difference, err := compare.Compare(reference, actual, uint8(*tolerance))
	if err != nil {
		return err
	}
	if err := writePNG(*diffPath, difference); err != nil {
		return err
	}
	if err := json.NewEncoder(output).Encode(report); err != nil {
		return err
	}
	if *maximum >= 0 && report.ChangedPercent > *maximum {
		return fmt.Errorf("pixel gate failed: %.4f%% changed exceeds %.4f%%", report.ChangedPercent, *maximum)
	}
	return nil
}

func readPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	picture, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return picture, nil
}

func writePNG(path string, picture image.Image) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".vivapad-diff-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := png.Encode(file, picture); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
