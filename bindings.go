package main

import (
	"github.com/codetesla51/logos/logos"
	"github.com/hajimehoshi/ebiten/v2"
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
			kind:  "rect",
			x:     float64(toI(args[0])), y: float64(toI(args[1])),
			w:     float64(toI(args[2])), h: float64(toI(args[3])),
			color: args[4].(*logos.String).Value,
		})
		return &logos.Null{}
	})

	vm.Register("draw_circle", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind:   "circle",
			x:      float64(toI(args[0])), y: float64(toI(args[1])),
			radius: toF(args[2]),
			color:  args[3].(*logos.String).Value,
		})
		return &logos.Null{}
	})

	vm.Register("draw_line", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind:      "line",
			x:         float64(toI(args[0])), y: float64(toI(args[1])),
			x2:        float64(toI(args[2])), y2: float64(toI(args[3])),
			color:     args[4].(*logos.String).Value,
			thickness: toF(args[5]),
		})
		return &logos.Null{}
	})

	vm.Register("draw_text", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			kind:  "text",
			str:   args[0].(*logos.String).Value,
			x:     float64(toI(args[1])), y: float64(toI(args[2])),
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
			kind:     "sprite_ex",
			path:     args[0].(*logos.String).Value,
			x:        float64(toI(args[1])), y: float64(toI(args[2])),
			scale:    toF(args[3]),
			rotation: toF(args[4]), // degrees
		})
		return &logos.Null{}
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
