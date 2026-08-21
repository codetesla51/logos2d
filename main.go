package main

import (
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	vm := newVM()
	g := newGame(vm)
	registerBindings(vm, g)

	loadScript(vm)

	ebiten.SetWindowSize(640, 480)
	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
}
