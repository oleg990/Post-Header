//go:build windows

package platform

import (
	"errors"
	"image"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")

	pOpenClipboard    = user32.NewProc("OpenClipboard")
	pCloseClipboard   = user32.NewProc("CloseClipboard")
	pEmptyClipboard   = user32.NewProc("EmptyClipboard")
	pGetClipboardData = user32.NewProc("GetClipboardData")
	pSetClipboardData = user32.NewProc("SetClipboardData")
	pRegisterCbFormat = user32.NewProc("RegisterClipboardFormatW")
	pGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	pGlobalFree       = kernel32.NewProc("GlobalFree")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pGlobalSize       = kernel32.NewProc("GlobalSize")
	pRtlMoveMemory    = kernel32.NewProc("RtlMoveMemory")
	pGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
)

const (
	cfUnicodeText = 13
	cfDIB         = 8
	gmemMoveable  = 0x0002
)

// Копируем память через RtlMoveMemory, а не через unsafe.Pointer(uintptr):
// так go vet спокоен, а указатель на Go-память живёт только в рамках вызова.

func openClipboard() error {
	for i := 0; i < 10; i++ { // буфер может быть занят другой программой
		if r, _, _ := pOpenClipboard.Call(0); r != 0 {
			return nil
		}
		time.Sleep(30 * time.Millisecond)
	}
	return errors.New("буфер обмена занят другой программой")
}

// ClipboardText возвращает текст из буфера обмена.
func ClipboardText() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return "", err
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", errors.New("в буфере нет текста")
	}
	p, _, _ := pGlobalLock.Call(h)
	if p == 0 {
		return "", errors.New("не удалось прочитать буфер")
	}
	defer pGlobalUnlock.Call(h)
	size, _, _ := pGlobalSize.Call(h)
	buf := make([]uint16, size/2)
	if len(buf) == 0 {
		return "", nil
	}
	pRtlMoveMemory.Call(uintptr(unsafe.Pointer(&buf[0])), p, uintptr(len(buf)*2))
	return syscall.UTF16ToString(buf), nil
}

// CopyImage кладёт картинку в буфер сразу в двух форматах: DIB (понимают все)
// и PNG (Telegram и браузеры берут его без потери качества).
func CopyImage(img image.Image) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	if err := setClipboard(cfDIB, dib(img)); err != nil {
		return err
	}
	name, _ := syscall.UTF16PtrFromString("PNG")
	if f, _, _ := pRegisterCbFormat.Call(uintptr(unsafe.Pointer(name))); f != 0 {
		setClipboard(f, pngBytes(img))
	}
	return nil
}

func setClipboard(format uintptr, data []byte) error {
	h, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if h == 0 {
		return errors.New("не хватает памяти для буфера обмена")
	}
	p, _, _ := pGlobalLock.Call(h)
	if p == 0 {
		pGlobalFree.Call(h)
		return errors.New("не удалось записать в буфер")
	}
	pRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	pGlobalUnlock.Call(h)
	if r, _, _ := pSetClipboardData.Call(format, h); r == 0 {
		pGlobalFree.Call(h)
		return errors.New("Windows не принял картинку в буфер")
	}
	return nil // память теперь принадлежит буферу обмена
}

type openFileName struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	reserved2     uint32
	flagsEx       uint32
}

// OpenImageFile показывает стандартный диалог Windows «Открыть» для картинок.
// Пустая строка — пользователь нажал «Отмена».
func OpenImageFile(title string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	filter := utf16z("Изображения (*.jpg; *.jpeg; *.png)\x00*.jpg;*.jpeg;*.png\x00Все файлы\x00*.*\x00")
	file := make([]uint16, 1024)
	t, _ := syscall.UTF16PtrFromString(title)
	ofn := openFileName{
		filter:  &filter[0],
		file:    &file[0],
		maxFile: uint32(len(file)),
		title:   t,
		flags:   0x00080000 | 0x00001000 | 0x00000800 | 0x00000008, // EXPLORER | FILEMUSTEXIST | PATHMUSTEXIST | NOCHANGEDIR
	}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		return "", nil
	}
	return syscall.UTF16ToString(file), nil
}

// utf16z кодирует строку фильтра: внутри у неё нули-разделители,
// поэтому syscall.UTF16FromString (он их запрещает) не подходит.
func utf16z(s string) []uint16 {
	return append(utf16.Encode([]rune(s)), 0)
}

// OpenFolder открывает папку в проводнике.
func OpenFolder(dir string) error {
	return exec.Command("explorer", dir).Start()
}

// FontDirs — где искать шрифты для выбора в настройках.
func FontDirs() []string {
	dirs := []string{`C:\Windows\Fonts`}
	if win := os.Getenv("WINDIR"); win != "" {
		dirs[0] = win + `\Fonts`
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		dirs = append(dirs, local+`\Microsoft\Windows\Fonts`)
	}
	return dirs
}

// RunInstaller запускает установщик обновления без вопросов; программа
// после этого должна закрыться, а установщик запустит её снова.
func RunInstaller(path string) error {
	return exec.Command(path, "/SILENT", "/SUPPRESSMSGBOXES", "/NORESTART").Start()
}
