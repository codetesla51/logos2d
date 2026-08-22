package main

import (
	"fmt"
	"github.com/codetesla51/logos/interpreter"
	"github.com/codetesla51/logos/logos"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"math"
	"math/rand/v2"
	"os"
	"strings"
)

// parseKey maps a Logos string to an Ebiten key.
func parseKey(name string) (ebiten.Key, bool) {
	keys := map[string]ebiten.Key{
		"right": ebiten.KeyRight, "left": ebiten.KeyLeft,
		"up": ebiten.KeyUp, "down": ebiten.KeyDown,
		"space": ebiten.KeySpace, "enter": ebiten.KeyEnter, "escape": ebiten.KeyEscape,
		"tab": ebiten.KeyTab, "shift": ebiten.KeyShift, "ctrl": ebiten.KeyControl,
		"w": ebiten.KeyW, "a": ebiten.KeyA, "s": ebiten.KeyS, "d": ebiten.KeyD,
		"m": ebiten.KeyM, "p": ebiten.KeyP,
	}
	k, ok := keys[name]
	return k, ok
}

// cliFlag reports whether "-name" was passed on the command line,
// letting scripts read launch options (e.g. ./logos2d demo/main.lgs -auto).
func cliFlag(name string) bool {
	want := "-" + name
	for _, a := range os.Args[1:] {
		if a == want {
			return true
		}
	}
	return false
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
func registerBindings(vm *interpreter.Interpreter, g *Game) {
	vm.Register("draw_rect", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			camX: g.camX, camY: g.camY,
			kind: "rect",
			x:    float64(toI(args[0])), y: float64(toI(args[1])),
			w: float64(toI(args[2])), h: float64(toI(args[3])),
			color: args[4].(*logos.String).Value,
		})
		return &logos.Null{}
	})

	vm.Register("draw_circle", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			camX: g.camX, camY: g.camY,
			kind: "circle",
			x:    float64(toI(args[0])), y: float64(toI(args[1])),
			radius: toF(args[2]),
			color:  args[3].(*logos.String).Value,
		})
		return &logos.Null{}
	})

	vm.Register("draw_line", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			camX: g.camX, camY: g.camY,
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
			camX: g.camX, camY: g.camY,
			kind: "text",
			str:  args[0].(*logos.String).Value,
			x:    float64(toI(args[1])), y: float64(toI(args[2])),
			color: args[3].(*logos.String).Value,
		})
		return &logos.Null{}
	})

	vm.Register("draw_sprite", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			camX: g.camX, camY: g.camY,
			kind: "sprite",
			path: args[0].(*logos.String).Value,
			x:    float64(toI(args[1])), y: float64(toI(args[2])),
		})
		return &logos.Null{}
	})

	vm.Register("draw_sprite_ex", func(args ...logos.Object) logos.Object {
		g.cmds = append(g.cmds, drawCmd{
			camX: g.camX, camY: g.camY,
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

	vm.Register("len", func(args ...logos.Object) logos.Object {
		switch v := args[0].(type) {
		case *logos.Array:
			return &logos.Integer{Value: int64(len(v.Elements))}
		case *logos.String:
			return &logos.Integer{Value: int64(len(v.Value))}
		case *logos.Table:
			return &logos.Integer{Value: int64(len(v.Pairs))}
		}
		return &logos.Integer{Value: 0}
	})

	vm.Register("abs", func(args ...logos.Object) logos.Object {
		v := toF(args[0])
		if v < 0 {
			v = -v
		}
		return &logos.Float{Value: v}
	})

	vm.Register("cli_flag", func(args ...logos.Object) logos.Object {
		name, ok := args[0].(*logos.String)
		if !ok {
			return &logos.Bool{Value: false}
		}
		return &logos.Bool{Value: cliFlag(name.Value)}
	})

	// ---------------- declarative ECS layer ----------------
	// Entities are opaque integer IDs. Fixed props (x/y/vx/vy/rot/spin/
	// scale/hp/sprite) live in struct fields; any other spawn key lands in
	// the entity's Data map for the script's own use.

	entGet := func(e *Entity, key string) interpreter.Object {
		switch key {
		case "x":
			return &logos.Float{Value: e.X}
		case "y":
			return &logos.Float{Value: e.Y}
		case "vx":
			return &logos.Float{Value: e.VX}
		case "vy":
			return &logos.Float{Value: e.VY}
		case "rot":
			return &logos.Float{Value: e.Rot}
		case "spin":
			return &logos.Float{Value: e.Spin}
		case "scale":
			return &logos.Float{Value: e.Scale}
		case "hp":
			return &logos.Integer{Value: e.HP}
		case "sprite":
			return &logos.String{Value: e.Sprite}
		case "group":
			return &logos.String{Value: e.Group}
		}
		if v, ok := e.Data[key]; ok {
			return v
		}
		return &logos.Null{}
	}

	setNum := func(e *Entity, key string, f float64) {
		switch key {
		case "x":
			e.X = f
		case "y":
			e.Y = f
		case "vx":
			e.VX = f
		case "vy":
			e.VY = f
		case "rot":
			e.Rot = f
		case "spin":
			e.Spin = f
		case "scale":
			e.Scale = f
		case "w":
			e.HW = f / 2 // full extent prop -> half extents around center
		case "h":
			e.HH = f / 2
		case "ttl":
			e.TTL = int64(f)
		case "max_hp":
			e.MaxHP = int64(f)
		}
	}

	isCoreKey := func(key string) bool {
		switch key {
		case "x", "y", "vx", "vy", "rot", "spin", "scale", "w", "h", "ttl", "max_hp":
			return true
		}
		return false
	}

	entSet := func(e *Entity, key string, v logos.Object) {
		switch val := v.(type) {
		case *logos.Float:
			if key == "hp" {
				e.HP = int64(val.Value)
				e.HasHP = true
				return
			}
			setNum(e, key, val.Value)
			if !isCoreKey(key) {
				e.Data[key] = val
			}
		case *logos.Integer:
			if key == "hp" {
				e.HP = val.Value
				e.HasHP = true
				return
			}
			// CRITICAL: integer literals (vy: -6, w: 19) must still reach
			// the numeric fields — coerce instead of dumping into Data.
			setNum(e, key, float64(val.Value))
			if !isCoreKey(key) {
				e.Data[key] = val
			}
		case *logos.String:
			if key == "sprite" {
				e.Sprite = val.Value
			} else {
				e.Data[key] = val
			}
		default:
			e.Data[key] = v
		}
	}

	getEnt := func(args []logos.Object) *Entity {
		id := toI(args[0])
		return g.world.entity(id)
	}

	vm.Register("create", func(args ...logos.Object) logos.Object {
		g.world.active = true
		group := args[0].(*logos.String).Value
		sprite := args[1].(*logos.String).Value
		e := &Entity{
			Group:  group,
			Sprite: sprite,
			X:      toF(args[2]),
			Y:      toF(args[3]),
			HP:     1, // default: alive unless props say otherwise
			HasHP:  true,
			TTL:    -1, // default: no expiry
			Data:   map[string]interpreter.Object{},
		}
		if props, ok := args[4].(*logos.Table); ok {
			for k, v := range props.Pairs {
				key := strings.TrimPrefix(k, "STRING:")
				if key == "hp" {
					e.HasHP = true
				}
				entSet(e, key, v)
			}
		}
		if e.Scale == 0 {
			e.Scale = 1
		}
		if e.MaxHP == 0 {
			e.MaxHP = e.HP
		}
		id := g.world.mustSpawn(e)
		return &logos.Integer{Value: id}
	})

	vm.Register("kill", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			g.killEntity(e)
		}
		return &logos.Null{}
	})

	vm.Register("damage", func(args ...logos.Object) logos.Object {
		died := false
		if e := getEnt(args); e != nil {
			e.HP -= int64(toF(args[1]))
			if died = e.HasHP && e.HP <= 0; died {
				g.killEntity(e)
			}
		}
		return &logos.Bool{Value: died}
	})

	vm.Register("heal", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			e.HP += int64(toF(args[1]))
			if e.MaxHP > 0 && e.HP > e.MaxHP {
				e.HP = e.MaxHP
			}
		}
		return &logos.Null{}
	})

	vm.Register("max_hp", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			e.MaxHP = int64(toF(args[1]))
			e.HasHP = true
		}
		return &logos.Null{}
	})

	vm.Register("invuln_after", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			e.InvUntil = g.world.Tick + int64(toF(args[1]))
		}
		return &logos.Null{}
	})

	vm.Register("on_death", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			if fn, ok := args[1].(*logos.Function); ok {
				e.DeathFn = fn
			}
		}
		return &logos.Null{}
	})

	vm.Register("ttl_left", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			return &logos.Integer{Value: e.TTL}
		}
		return &logos.Integer{Value: -1}
	})

	vm.Register("seek", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			steerToward(e, toF(args[1]), toF(args[2]), toF(args[3]))
		}
		return &logos.Null{}
	})

	vm.Register("nearest", func(args ...logos.Object) logos.Object {
		group := args[0].(*logos.String).Value
		tx, ty := toF(args[1]), toF(args[2])
		best := int64(-1)
		bestD := math.MaxFloat64
		for _, e := range g.world.group(group) {
			dx := e.X - tx
			dy := e.Y - ty
			if d := dx*dx + dy*dy; d < bestD {
				bestD = d
				best = e.ID
			}
		}
		return &logos.Integer{Value: best}
	})

	// run_behavior(id, tree) — first-class declarative behavior tree.
	// Condition node: table{cond: "hp_below" | "player_near", val: N,
	//                       "then": <node>, "else": <node>}
	// Action node:    table{type: "chase" | "flee", spd: F, y: F}
	// Branches may nest condition nodes (recurses). Optional action fields:
	// spd (default 2.0), y = fixed altitude line instead of the player's y.
	// Intended to be called every tick from an ent_on_tick closure; each
	// call ends in exactly one steerToward (same math as seek).
	vm.Register("run_behavior", func(args ...logos.Object) logos.Object {
		g.world.active = true
		if e := getEnt(args); e != nil {
			if tree, ok := args[1].(*logos.Table); ok {
				runBehaviorNode(g, e, tree)
			}
		}
		return &logos.Null{}
	})

	vm.Register("group_each", func(args ...logos.Object) logos.Object {
		fn, ok := args[1].(*logos.Function)
		if ok {
			for _, e := range g.world.group(args[0].(*logos.String).Value) {
				g.callClosure(fn, []interpreter.Object{&logos.Integer{Value: e.ID}})
			}
		}
		return &logos.Null{}
	})

	vm.Register("after_n", func(args ...logos.Object) logos.Object {
		fn, ok := args[2].(*logos.Function)
		if ok {
			n := int(toF(args[0]))
			g.world.timers = append(g.world.timers, &Timer{
				Interval: n,
				Count:    int(toF(args[1])),
				Fn:       fn,
				Left:     n,
				Repeat:   true,
			})
			g.world.active = true
		}
		return &logos.Null{}
	})

	vm.Register("knockback", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			fx, fy, force := toF(args[1]), toF(args[2]), toF(args[3])
			dx, dy := e.X-fx, e.Y-fy
			d := math.Sqrt(dx*dx + dy*dy)
			if d > 0.0001 {
				e.VX += dx / d * force
				e.VY += dy / d * force
			}
		}
		return &logos.Null{}
	})

	vm.Register("area_damage", func(args ...logos.Object) logos.Object {
		tx, ty := toF(args[0]), toF(args[1])
		radius, dmg := toF(args[2]), toF(args[3])
		group := args[4].(*logos.String).Value
		r2 := radius * radius
		for _, e := range g.world.group(group) {
			dx := e.X - tx
			dy := e.Y - ty
			if dx*dx+dy*dy <= r2 {
				e.HP -= int64(dmg)
				if e.HasHP && e.HP <= 0 {
					g.killEntity(e)
				}
			}
		}
		return &logos.Null{}
	})

	vm.Register("shake", func(args ...logos.Object) logos.Object {
		g.shakePow = toF(args[0])
		g.shakeLeft = int(toF(args[1]))
		return &logos.Null{}
	})

	vm.Register("flash", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			hexv := strings.TrimPrefix(args[1].(*logos.String).Value, "#")
			var r, gc, b int
			fmt.Sscanf(hexv, "%02x%02x%02x", &r, &gc, &b)
			mx := float64(r)
			if float64(gc) > mx {
				mx = float64(gc)
			}
			if float64(b) > mx {
				mx = float64(b)
			}
			if mx < 1 {
				mx = 1
			}
			k := 2.0 / mx // dominant channel overbrightens for a strong wash
			e.FlashR = float64(r) * k
			e.FlashG = float64(gc) * k
			e.FlashB = float64(b) * k
			e.FlashUntil = g.world.Tick + int64(toF(args[2]))
		}
		return &logos.Null{}
	})

	vm.Register("hp_bar", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			e.BarDX = toF(args[1])
			e.BarDY = toF(args[2])
			e.BarW = toF(args[3])
			e.HasBar = true
			if e.MaxHP == 0 {
				e.MaxHP = e.HP
			}
		}
		return &logos.Null{}
	})

	vm.Register("ent_get", func(args ...logos.Object) logos.Object {
		e := getEnt(args)
		if e == nil {
			return &logos.Null{}
		}
		return entGet(e, args[1].(*logos.String).Value)
	})

	vm.Register("ent_set", func(args ...logos.Object) logos.Object {
		e := getEnt(args)
		if e != nil {
			entSet(e, args[1].(*logos.String).Value, args[2])
		}
		return &logos.Null{}
	})

	vm.Register("group_count", func(args ...logos.Object) logos.Object {
		return &logos.Integer{Value: g.world.count(args[0].(*logos.String).Value)}
	})

	vm.Register("group_ids", func(args ...logos.Object) logos.Object {
		ids := make([]interpreter.Object, 0, 8)
		for _, e := range g.world.group(args[0].(*logos.String).Value) {
			ids = append(ids, &logos.Integer{Value: e.ID})
		}
		return &logos.Array{Elements: ids}
	})

	vm.Register("set_world_paused", func(args ...logos.Object) logos.Object {
		if bl, ok := args[0].(*logos.Bool); ok {
			g.world.paused = bl.Value
		}
		return &logos.Null{}
	})

	vm.Register("reset_world", func(args ...logos.Object) logos.Object {
		g.world.reset()
		return &logos.Null{}
	})

	storeClosure := func(arg logos.Object) *logos.Function {
		if fn, ok := arg.(*logos.Function); ok {
			g.world.active = true
			return fn
		}
		return nil
	}

	vm.Register("collide", func(args ...logos.Object) logos.Object {
		fn := storeClosure(args[2])
		if fn != nil {
			g.world.rules = append(g.world.rules, CollideRule{
				A:  args[0].(*logos.String).Value,
				B:  args[1].(*logos.String).Value,
				Fn: fn,
			})
		}
		return &logos.Null{}
	})

	vm.Register("every", func(args ...logos.Object) logos.Object {
		fn := storeClosure(args[1])
		if fn != nil {
			g.world.timers = append(g.world.timers, &Timer{
				Interval: int(toI(args[0])), Left: int(toI(args[0])), Repeat: true, Fn: fn,
			})
		}
		return &logos.Null{}
	})

	vm.Register("after", func(args ...logos.Object) logos.Object {
		fn := storeClosure(args[1])
		if fn != nil {
			g.world.timers = append(g.world.timers, &Timer{
				Interval: int(toI(args[0])), Left: int(toI(args[0])), Repeat: false, Fn: fn,
			})
		}
		return &logos.Null{}
	})

	vm.Register("ent_on_tick", func(args ...logos.Object) logos.Object {
		if e := getEnt(args); e != nil {
			e.TickFn = storeClosure(args[1])
		}
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
			camX: g.camX, camY: g.camY,
			kind: "sprite_frame",
			path: args[0].(*logos.String).Value,
			x:    float64(toI(args[1])), y: float64(toI(args[2])),
			w: float64(toI(args[3])), h: float64(toI(args[4])),
			radius: float64(toI(args[5])), // frame index (reuses field)
		})
		return &logos.Null{}
	})

	vm.Register("math_sin", func(args ...logos.Object) logos.Object {
		// radians in, float out
		return &logos.Float{Value: math.Sin(toF(args[0]))}
	})

	vm.Register("math_cos", func(args ...logos.Object) logos.Object {
		return &logos.Float{Value: math.Cos(toF(args[0]))}
	})

	vm.Register("text_width", func(args ...logos.Object) logos.Object {
		// pixel width of a string in the current font (for centering)
		if g.face == nil {
			return &logos.Integer{Value: 0}
		}
		return &logos.Integer{Value: int64(text.BoundString(g.face, args[0].(*logos.String).Value).Dx())}
	})

	vm.Register("atan2", func(args ...logos.Object) logos.Object {
		// returns degrees; pairs with draw_sprite_ex for aiming math
		return &logos.Float{Value: math.Atan2(toF(args[0]), toF(args[1])) * 180 / math.Pi}
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

// ---------------- behavior trees (first-class AI primitive) ----------------

// tblField reads a string-keyed field from a script table; nil if absent.
// Logos stores table keys as "STRING:<name>".
func tblField(t *logos.Table, key string) logos.Object {
	if v, ok := t.Pairs["STRING:"+key]; ok {
		return v
	}
	return nil
}

// tblStr reads a string field; ok=false when missing or not a string.
func tblStr(t *logos.Table, key string) (string, bool) {
	if s, ok := tblField(t, key).(*logos.String); ok {
		return s.Value, true
	}
	return "", false
}

// tblNum reads a numeric field (int or float); ok=false when missing.
func tblNum(t *logos.Table, key string) (float64, bool) {
	switch v := tblField(t, key).(type) {
	case *logos.Integer:
		return float64(v.Value), true
	case *logos.Float:
		return v.Value, true
	}
	return 0, false
}

// steerToward is the shared seek math: set velocity toward a point,
// arriving exactly instead of orbiting it.
func steerToward(e *Entity, x, y, speed float64) {
	dx := x - e.X
	dy := y - e.Y
	d := math.Sqrt(dx*dx + dy*dy)
	if d > 0.0001 {
		if d < speed {
			speed = d
		}
		e.VX = dx / d * speed
		e.VY = dy / d * speed
	} else {
		e.VX, e.VY = 0, 0
	}
}

// nearestOf returns the closest living "player"-group entity to e.
func nearestOf(g *Game, e *Entity) *Entity {
	var best *Entity
	bestD := math.MaxFloat64
	for _, o := range g.world.group("player") {
		dx, dy := o.X-e.X, o.Y-e.Y
		if d := dx*dx + dy*dy; d < bestD {
			bestD = d
			best = o
		}
	}
	return best
}

// runBehaviorNode evaluates one behavior-tree node for an entity:
// action nodes dispatch straight to steering; condition nodes test and
// recurse into their "then"/"else" branch. Unknown conds/types no-op.
func runBehaviorNode(g *Game, e *Entity, t *logos.Table) {
	if typ, ok := tblStr(t, "type"); ok {
		runBehaviorAction(g, e, t, typ)
		return
	}
	cond, _ := tblStr(t, "cond")
	val, hasVal := tblNum(t, "val")
	fired := false
	switch cond {
	case "hp_below":
		fired = hasVal && e.HasHP && float64(e.HP) < val
	case "player_near":
		if hasVal {
			if p := nearestOf(g, e); p != nil {
				dx, dy := p.X-e.X, p.Y-e.Y
				fired = dx*dx+dy*dy < val*val
			}
		}
	}
	branch := "else"
	if fired {
		branch = "then"
	}
	if n, ok := tblField(t, branch).(*logos.Table); ok {
		runBehaviorNode(g, e, n)
	}
}

// runBehaviorAction turns an action node into a single seek-style steer.
func runBehaviorAction(g *Game, e *Entity, act *logos.Table, typ string) {
	p := nearestOf(g, e)
	if p == nil {
		return // nobody to chase or flee from
	}
	spd := 2.0
	if v, ok := tblNum(act, "spd"); ok {
		spd = v
	}
	switch typ {
	case "chase":
		ty := p.Y
		if alt, ok := tblNum(act, "y"); ok {
			ty = alt // hold a fixed altitude line instead of diving
		}
		steerToward(e, p.X, ty, spd)
	case "flee":
		// mirror point away from the player, clamped on-screen
		fx := e.X + (e.X-p.X)*2
		fy := e.Y + (e.Y-p.Y)*2
		if alt, ok := tblNum(act, "y"); ok {
			fy = alt
		}
		if fx < 14 {
			fx = 14
		}
		if fx > gameW-14 {
			fx = gameW - 14
		}
		if fy < 20 {
			fy = 20
		}
		if fy > gameH-20 {
			fy = gameH - 20
		}
		steerToward(e, fx, fy, spd)
	}
}
