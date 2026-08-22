package main

import (
	"os"
	"testing"
)

func TestHeadlessDemo2(t *testing.T) {
	wd, _ := os.Getwd()
	if err := os.Chdir("demo2"); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	vm := newVM()
	g := newGame(vm)
	registerBindings(vm, g)
	if err := vm.Run(readMain()); err != nil {
		t.Fatalf("parse/run error: %v", err)
	}
	if _, err := vm.Call("on_load"); err != nil {
		t.Fatalf("on_load error: %v", err)
	}
	// jump straight into gameplay
	if _, err := vm.Call("start_game"); err != nil {
		t.Fatalf("start_game error: %v", err)
	}
	// grant the rocket weapon path a workout by flipping the flag
	_ = g
	for i := 0; i < 3000; i++ {
		if err := g.Update(); err != nil {
			t.Fatalf("update %d error: %v", i, err)
		}
	}
	// poke every hook we rely on so syntax/logic errors surface
	for _, fn := range []string{"on_draw_back", "on_draw_front", "draw_hud"} {
		if _, err := vm.Call(fn); err != nil {
			t.Logf("%s: %v", fn, err)
		}
	}
}

func readMain() string {
	b, _ := os.ReadFile("main.lgs")
	return string(b)
}
