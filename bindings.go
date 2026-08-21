package main

import (
	"github.com/codetesla51/logos/logos"
	"github.com/hajimehoshi/ebiten/v2"
	"math"
	"math/rand/v2"
)

// parseKey maps a Logos string to an Ebiten key.
func parseKey(name string) (ebiten.Key, bool) {
	keys := map[string]ebiten.Key{
		"right": ebiten.KeyRight, "left": ebiten.KeyLeft,
		"up": ebiten.KeyUp, "down": ebiten.KeyDown,
		"space": ebiten.KeySpace, "enter": ebiten.KeyEnter, "escape": ebiten.KeyEscape,
		"tab": ebiten.KeyTab, "shift": ebiten.KeyShift, "ctrl": ebiten.KeyControl,
		"w": ebiten.KeyW, "a": ebiten.KeyA, "s": ebiten.KeyS, "d": ebiten.KeyD,
	}
	k, ok := keys[name]
	return k, ok
}

// parseMouseButton maps "left"/"right"/"middle" to an Ebiten mouse button.
func parseMouseButton(name string) (ebiten.MouseButton, bool) {
	buttons := map[string]ebiten.MouseButton{
		"left": ebiten.MouseButtonLeft, "right": ebiten.MouseButtonRight,
		"middle": ebiten.MouseButtonMiddle,
	}
	b, ok := buttons[name]
	return b, ok
}

// registerBindings wires every Logos-callable builtin to the game state.
func registerBindings(vm *logos.VM, g *Game) {
	vm.Register("draw_rect", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind: "rect",
			x:    float64(toI(args[0])), y: float64(toI(args[1])),
			w: float64(toI(args[2])), h: float64(toI(args[3])),
			color: args[4].(*logos.String).Value,
		})
		return &logos.Null{}
	})

	vm.Register("draw_circle", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind: "circle",
			x:    float64(toI(args[0])), y: float64(toI(args[1])),
			radius: toF(args[2]),
			color:  args[3].(*logos.String).Value,
		})
		return &logos.Null{}
	})

	vm.Register("draw_line", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind: "line",
			x:    float64(toI(args[0])), y: float64(toI(args[1])),
			x2: float64(toI(args[2])), y2: float64(toI(args[3])),
			color:     args[4].(*logos.String).Value,
			thickness: toF(args[5]),
		})
		return &logos.Null{}
	})

	vm.Register("draw_text", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind: "text",
			str:  args[0].(*logos.String).Value,
			x:    float64(toI(args[1])), y: float64(toI(args[2])),
			color: args[3].(*logos.String).Value,
		})
		return &logos.Null{}
	})

	vm.Register("draw_sprite", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind: "sprite",
			path: args[0].(*logos.String).Value,
			x:    float64(toI(args[1])), y: float64(toI(args[2])),
		})
		return &logos.Null{}
	})

	vm.Register("draw_sprite_ex", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind: "sprite_ex",
			path: args[0].(*logos.String).Value,
			x:    float64(toI(args[1])), y: float64(toI(args[2])),
			scale:    toF(args[3]),
			rotation: toF(args[4]), // degrees
		})
		return &logos.Null{}
	})

	vm.Register("window_width", func(args ...logos.Object) logos.Object {
		return &logos.Integer{Value: gameW}
	})

	vm.Register("window_height", func(args ...logos.Object) logos.Object {
		return &logos.Integer{Value: gameH}
	})

	vm.Register("set_title", func(args ...logos.Object) logos.Object {
		ebiten.SetWindowTitle(args[0].(*logos.String).Value)
		return &logos.Null{}
	})

	vm.Register("quit", func(args ...logos.Object) logos.Object {
		g.quitRequested = true // Update() turns this into ebiten.Termination
		return &logos.Null{}
	})

	vm.Register("set_camera", func(args ...logos.Object) logos.Object {
		g.camX = toF(args[0])
		g.camY = toF(args[1])
		return &logos.Null{}
	})

	vm.Register("camera_x", func(args ...logos.Object) logos.Object {
		return &logos.Integer{Value: int64(g.camX)}
	})

	vm.Register("camera_y", func(args ...logos.Object) logos.Object {
		return &logos.Integer{Value: int64(g.camY)}
	})

	vm.Register("random", func(args ...logos.Object) logos.Object {
		minV := int64(toF(args[0]))
		maxV := int64(toF(args[1]))
		if maxV < minV {
			minV, maxV = maxV, minV
		}
		return &logos.Integer{Value: minV + rand.Int64N(maxV-minV+1)}
	})

	vm.Register("play_sound", func(args ...logos.Object) logos.Object {
		g.playSound(args[0].(*logos.String).Value)
		return &logos.Null{}
	})

	vm.Register("play_music", func(args ...logos.Object) logos.Object {
		g.playMusic(args[0].(*logos.String).Value)
		return &logos.Null{}
	})

	vm.Register("stop_music", func(args ...logos.Object) logos.Object {
		g.stopMusic()
		return &logos.Null{}
	})

	// preload_sprite loads and decodes an image NOW instead of on first draw.
	vm.Register("preload_sprite", func(args ...logos.Object) logos.Object {
		return &logos.Bool{Value: g.sprite(args[0].(*logos.String).Value) != nil}
	})

	// draw_sprite_frame draws frame `index` from a horizontal-strip sheet.
	vm.Register("draw_sprite_frame", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind: "sprite_frame",
			path: args[0].(*logos.String).Value,
			x:    float64(toI(args[1])), y: float64(toI(args[2])),
			w: float64(toI(args[3])), h: float64(toI(args[4])),
			radius: float64(toI(args[5])), // frame index (reuses field)
		})
		return &logos.Null{}
	})

	vm.Register("rects_overlap", func(args ...logos.Object) logos.Object {
		x1, y1, w1, h1 := toF(args[0]), toF(args[1]), toF(args[2]), toF(args[3])
		x2, y2, w2, h2 := toF(args[4]), toF(args[5]), toF(args[6]), toF(args[7])
		// AABB test: overlap requires separation check to fail on BOTH axes
		overlap := x1 < x2+w2 && x2 < x1+w1 && y1 < y2+h2 && y2 < y1+h1
		return &logos.Bool{Value: overlap}
	})

	vm.Register("point_in_rect", func(args ...logos.Object) logos.Object {
		px, py := toF(args[0]), toF(args[1])
		x, y, w, h := toF(args[2]), toF(args[3]), toF(args[4]), toF(args[5])
		inside := px >= x && px <= x+w && py >= y && py <= y+h
		return &logos.Bool{Value: inside}
	})

	vm.Register("distance", func(args ...logos.Object) logos.Object {
		dx := toF(args[2]) - toF(args[0])
		dy := toF(args[3]) - toF(args[1])
		return &logos.Float{Value: math.Sqrt(dx*dx + dy*dy)}
	})

	vm.Register("key_down", func(args ...logos.Object) logos.Object {
		key, ok := parseKey(args[0].(*logos.String).Value)
		if !ok {
			return &logos.Bool{Value: false}
		}
		return &logos.Bool{Value: g.keyDown(key)}
	})

	vm.Register("key_pressed", func(args ...logos.Object) logos.Object {
		key, ok := parseKey(args[0].(*logos.String).Value)
		if !ok {
			return &logos.Bool{Value: false}
		}
		return &logos.Bool{Value: g.keyPressed(key)}
	})

	vm.Register("key_released", func(args ...logos.Object) logos.Object {
		key, ok := parseKey(args[0].(*logos.String).Value)
		if !ok {
			return &logos.Bool{Value: false}
		}
		return &logos.Bool{Value: g.keyReleased(key)}
	})

	vm.Register("mouse_pos", func(args ...logos.Object) logos.Object {
		x, y := ebiten.CursorPosition()
		return &logos.Table{Pairs: map[string]logos.Object{
			"STRING:x": &logos.Integer{Value: int64(x)},
			"STRING:y": &logos.Integer{Value: int64(y)},
		}}
	})

	vm.Register("mouse_down", func(args ...logos.Object) logos.Object {
		b, ok := parseMouseButton(args[0].(*logos.String).Value)
		if !ok {
			return &logos.Bool{Value: false}
		}
		return &logos.Bool{Value: g.mouseDown(b)}
	})

	vm.Register("mouse_pressed", func(args ...logos.Object) logos.Object {
		b, ok := parseMouseButton(args[0].(*logos.String).Value)
		if !ok {
			return &logos.Bool{Value: false}
		}
		return &logos.Bool{Value: g.mousePressed(b)}
	})
}
