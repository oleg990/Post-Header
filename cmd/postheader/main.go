// Post Header — окно программы. Сборка для Windows без консоли:
//
//	go build -ldflags "-H windowsgui" ./cmd/postheader
package main

import (
	"log"
	"os"

	"gioui.org/app"

	"postheader/internal/ui"
)

func main() {
	go func() {
		if err := ui.Run(); err != nil {
			log.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}
