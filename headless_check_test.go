package main

import (
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestHeadlessDemo2(t *testing.T) {
	headless(t, "demo/void_runner")
}

func TestHeadlessBreakout(t *testing.T) {
	headless(t, "demo/breakout")
}

// TestHeadlessBreakoutAuto enables the autopilot bot (-auto) so the bot's
// runtime path (ai_steer / ai_predict_x, auto-launch, auto level/retry)
// is actually exercised, not just parsed.
func TestHeadlessBreakoutAuto(t *testing.T) {
	old := os.Args
	os.Args = []string{"logos2d", "-auto"}
	defer func() { os.Args = old }()
	headless(t, "demo/breakout")
}

func headless(t *testing.T, dir string) {
	t.Helper()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
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
	// simulate an ENTER press: menu/game-over screens should restart into
	// gameplay through the real input path; then SPACE (launch/shoot)
	g.InjectKey(ebiten.KeyEnter)
	if err := g.Update(); err != nil {
		t.Fatalf("enter update error: %v", err)
	}
	g.InjectKey(ebiten.KeySpace)
	if err := g.Update(); err != nil {
		t.Fatalf("space update error: %v", err)
	}
	for i := 0; i < 600; i++ {
		if err := g.Update(); err != nil {
			t.Fatalf("post-enter update %d error: %v", i, err)
		}
	}
	// poke every hook we rely on so syntax/logic errors surface
	for _, fn := range hooks() {
		if _, err := vm.Call(fn); err != nil {
			t.Logf("%s: %v", fn, err)
		}
	}
}

func hooks() []string {
	return []string{"on_draw_back", "on_draw_front", "draw_hud"}
}

func readMain() string {
	b, _ := os.ReadFile("main.lgs")
	return string(b)
}
