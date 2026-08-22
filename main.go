package main

import (
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	// Optional first arg: path to a .lgs script. We chdir into its folder so
	// asset paths inside the script resolve relative to the script's home.
	if len(os.Args) > 1 {
		dir := filepath.Dir(os.Args[1])
		if dir != "." {
			if err := os.Chdir(dir); err != nil {
				panic(err)
			}
		}
	}

	vm := newVM()
	g := newGame(vm)
	registerBindings(vm, g)

	loadScript(vm)

	ebiten.SetWindowSize(960, 720)
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
}
