//go:build !windows

package platform

import (
	"errors"
	"image"
	"os"
	"os/exec"
)

// Заглушки для разработки на Linux/macOS: программа рассчитана на Windows.

func ClipboardText() (string, error) {
	out, err := exec.Command("xclip", "-o", "-selection", "clipboard").Output()
	if err != nil {
		return "", errors.New("буфер обмена доступен только в Windows")
	}
	return string(out), nil
}

func CopyImage(img image.Image) error {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png")
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return errors.New("копирование картинки доступно только в Windows")
	}
	in.Write(pngBytes(img))
	in.Close()
	return cmd.Wait()
}

// OpenImageFile на других ОС берёт путь из POSTHEADER_PHOTO (для проверки).
func OpenImageFile(title string) (string, error) {
	return os.Getenv("POSTHEADER_PHOTO"), nil
}

func OpenFolder(dir string) error {
	return exec.Command("xdg-open", dir).Start()
}

func FontDirs() []string {
	return []string{"/usr/share/fonts"}
}

var _ = dib // используется только в Windows-версии

func RunInstaller(path string) error {
	return errors.New("обновление ставится только в Windows: " + path)
}

// ChooseFolder на других ОС берёт путь из POSTHEADER_DIR (для проверки).
func ChooseFolder(title string) (string, error) {
	return os.Getenv("POSTHEADER_DIR"), nil
}
