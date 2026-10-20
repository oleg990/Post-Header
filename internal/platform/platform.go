// Package platform — то, что зависит от Windows: буфер обмена, диалог выбора
// файла, папки пользователя. На других ОС — заглушки, чтобы окно можно было
// запускать и проверять при разработке.
package platform

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

// PicturesDir — папка «Изображения\PostHeader» для готовых постов.
func PicturesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Pictures", "PostHeader")
}

// dib собирает картинку в формате CF_DIB: BITMAPINFOHEADER + BGRA снизу вверх.
func dib(img image.Image) []byte {
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	w, h := b.Dx(), b.Dy()
	var buf bytes.Buffer
	hdr := struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		ClrUsed, ClrImportant  uint32
	}{Size: 40, Width: int32(w), Height: int32(h), Planes: 1, BitCount: 32, SizeImage: uint32(w * h * 4)}
	binary.Write(&buf, binary.LittleEndian, hdr)
	row := make([]byte, w*4)
	for y := h - 1; y >= 0; y-- {
		src := rgba.Pix[y*rgba.Stride : y*rgba.Stride+w*4]
		for x := 0; x < w; x++ {
			row[x*4+0] = src[x*4+2]
			row[x*4+1] = src[x*4+1]
			row[x*4+2] = src[x*4+0]
			row[x*4+3] = 255
		}
		buf.Write(row)
	}
	return buf.Bytes()
}

func pngBytes(img image.Image) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
