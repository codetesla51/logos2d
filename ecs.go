package main

// ecs.go — the declarative entity layer.
//
// The engine owns the world: entities, timers, collision rules. The script
// only DECLARES things (spawn/collide/every/after) and this pipeline runs
// them every tick: timers → motion → per-entity steering → collisions →
// death/cull. Entities are opaque integer IDs from the script's perspective;
// closures passed to collide/every/ent_on_tick are invoked via callClosure.

import (
	"fmt"
	"math/rand"

	"github.com/codetesla51/logos/interpreter"
)

// Entity is one thing in the world. Script-visible props live in fixed
// fields; anything else a script puts in spawn props lands in Data.
type Entity struct {
	ID                     int64
	Group                  string
	Sprite                 string
	X, Y                   float64
	VX, VY                 float64
	Rot                    float64 // degrees
	Spin                   float64 // degrees per tick
	Scale                  float64
	HW, HH                 float64 // half extents of hitbox (box centered on X, Y)
	HP                     int64
	HasHP                  bool                  // only hp-declaring entities can die by hp
	MaxHP                  int64                 // 0 = unset (defaults to spawn HP)
	TickFn                 *interpreter.Function // optional per-entity steering closure
	TickDead               bool                  // steering closure errored — stop calling it
	DeathFn                *interpreter.Function // optional one-shot death handler
	deathDone              bool
	TTL                    int64   // ticks left before auto-death; -1 = infinite
	InvUntil               int64   // world.Tick until which collisions ignore this entity
	FlashR, FlashG, FlashB float64 // hit-flash tint (applied while Tick < FlashUntil)
	FlashUntil             int64
	BarDX, BarDY, BarW     float64 // attached HP bar offset/width
	HasBar                 bool
	Data                   map[string]interpreter.Object // custom script keys (e.g. "kind")
	Dead                   bool
}

// Timer is an every()/after() registration.
type Timer struct {
	Interval int
	Count    int // >0: fire exactly this many times, then stop
	Fn       *interpreter.Function
	Left     int
	Repeat   bool
	FnDead   bool // closure errored at least once — stop calling it
}

// CollideRule fires Fn(aID, bID) whenever an A-entity overlaps a B-entity.
type CollideRule struct {
	A, B string
	Fn   *interpreter.Function
}

// World is all declarative state. Lives on Game.
type World struct {
	entities []*Entity
	byID     map[int64]*Entity
	timers   []*Timer
	rules    []CollideRule
	nextID   int64
	Tick     int64
	active   bool // true once the script uses any ECS builtin
	paused   bool // script freezes the world during menus/pause/game-over
}

func newWorld() *World {
	return &World{byID: map[int64]*Entity{}}
}

// reset clears runtime state (entities + timers) but KEEPS collide rules:
// rules are declarations made in on_load and survive restarts.
func (w *World) reset() {
	w.entities = nil
	w.byID = map[int64]*Entity{}
	w.timers = nil
}

func (w *World) entity(id int64) *Entity {
	return w.byID[id]
}

func (w *World) mustSpawn(e *Entity) int64 {
	w.nextID++
	e.ID = w.nextID
	w.entities = append(w.entities, e)
	w.byID[e.ID] = e
	return e.ID
}

func (w *World) kill(id int64) {
	if e := w.byID[id]; e != nil {
		e.Dead = true
	}
}

// killEntity marks an entity dead and fires its on_death handler exactly
// once. Damage, kill(), and ttl expiry all route through here; drifting
// off-screen does NOT (culled entities die silently).
func (g *Game) killEntity(e *Entity) {
	if e.Dead {
		return
	}
	e.Dead = true
	if e.DeathFn != nil && !e.deathDone {
		e.deathDone = true
		g.callClosure(e.DeathFn, []interpreter.Object{&interpreter.Integer{Value: e.ID}})
	}
}

func (w *World) count(group string) int64 {
	n := int64(0)
	for _, e := range w.entities {
		if !e.Dead && e.Group == group {
			n++
		}
	}
	return n
}

func (w *World) group(name string) []*Entity {
	out := make([]*Entity, 0, 8)
	for _, e := range w.entities {
		if !e.Dead && e.Group == name {
			out = append(out, e)
		}
	}
	return out
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// overlaps reports axis-aligned overlap of two centered hitboxes.
func overlaps(a, b *Entity) bool {
	return absF(a.X-b.X) < a.HW+b.HW && absF(a.Y-b.Y) < a.HH+b.HH
}

var closureErrShown = false

// callClosure invokes a stored script function object with raw Object args.
// Closures keep their captured environment across calls (spike-verified).
func (g *Game) callClosure(fn *interpreter.Function, args []interpreter.Object) {
	if fn == nil {
		return
	}
	env := interpreter.NewEnclosedEnvironment(fn.Env)
	for idx, p := range fn.Parameters {
		var v interpreter.Object = interpreter.NULL
		if idx < len(args) {
			v = args[idx]
		}
		env.Set(p.Value, v)
	}
	res := g.vm.Eval(fn.Body, env)
	if rv, ok := res.(*interpreter.ReturnValue); ok {
		res = rv.Value
	}
	if e, ok := res.(*interpreter.Error); ok {
		if !closureErrShown {
			fmt.Println("[ecs] closure error:", e.Message)
			closureErrShown = true
		}
		// disable timers AND per-entity steering closures using this broken
		// function so we don't fail silently every tick forever
		for _, t := range g.world.timers {
			if t.Fn == fn {
				t.FnDead = true
			}
		}
		for _, e := range g.world.entities {
			if e.TickFn == fn {
				e.TickDead = true
			}
		}
	}
}

// simulate runs one full ECS tick. Call after on_update returns.
func (g *Game) simulate() {
	w := &g.world
	if w.paused {
		return
	}

	w.Tick++

	// 1. timers — iterate a SNAPSHOT: callbacks may register new timers
	// (after/after_n/every from inside handlers), and appending to the
	// same backing array we're iterating corrupts the pending list.
	snapshot := make([]*Timer, len(w.timers))
	copy(snapshot, w.timers)
	w.timers = nil // registrations made during callbacks collect here
	var keep []*Timer
	for _, t := range snapshot {
		if t.FnDead {
			continue
		}
		t.Left--
		if t.Left <= 0 {
			g.callClosure(t.Fn, nil)
			more := t.Repeat && !t.FnDead
			if more && t.Count > 0 {
				t.Count--
				more = t.Count > 0
			}
			if more {
				t.Left = t.Interval
				keep = append(keep, t)
			}
		} else {
			keep = append(keep, t)
		}
	}
	w.timers = append(keep, w.timers...)

	// 2. motion
	for _, e := range w.entities {
		e.X += e.VX
		e.Y += e.VY
		e.Rot += e.Spin
	}

	// 2.5 time-to-live expiry (routes through the death pipeline so
	// on_death handlers fire for timed effects too)
	for _, e := range w.entities {
		if !e.Dead && e.TTL > 0 {
			e.TTL--
			if e.TTL == 0 {
				g.killEntity(e)
			}
		}
	}

	// 3. per-entity steering hooks
	for _, e := range w.entities {
		if e.TickFn != nil && !e.TickDead && !e.Dead {
			g.callClosure(e.TickFn, []interpreter.Object{&interpreter.Integer{Value: e.ID}})
		}
	}

	// 4. collision rules (skipped entirely while either side is invulnerable,
	// so overlaps debounce instead of firing every frame)
	for _, r := range w.rules {
		as := w.group(r.A)
		bs := w.group(r.B)
		if len(as) == 0 || len(bs) == 0 {
			continue
		}
		for _, a := range as {
			if a.Dead || a.InvUntil > w.Tick {
				continue
			}
			for _, b := range bs {
				if b.Dead || b.InvUntil > w.Tick {
					continue
				}
				if overlaps(a, b) {
					g.callClosure(r.Fn, []interpreter.Object{
						&interpreter.Integer{Value: a.ID},
						&interpreter.Integer{Value: b.ID},
					})
				}
				if a.Dead {
					break
				}
			}
		}
	}

	// 5. death by hp (through the pipeline so on_death fires) + silent cull
	// far offscreen (generous top margin: spawners place things just above
	// the screen and let them fall in)
	const margin = 90.0
	for _, e := range w.entities {
		if e.HasHP && e.HP <= 0 {
			g.killEntity(e)
			continue
		}
		if e.X < -margin || e.X > gameW+margin ||
			e.Y > gameH+margin || e.Y < -margin*3 {
			e.Dead = true
		}
	}

	// 6. compact
	kept := w.entities[:0]
	for _, e := range w.entities {
		if !e.Dead {
			kept = append(kept, e)
		} else {
			delete(w.byID, e.ID)
		}
	}
	w.entities = kept
}

// drawEntities queues every living entity for rendering, plus any attached
// fx: screen-shake jitter, hit-flash tint, HP bars.
func (g *Game) drawEntities() {
	for _, e := range g.world.entities {
		scale := e.Scale
		if scale == 0 {
			scale = 1
		}
		// "hidden" data key lets scripts blink/fade entities without killing them
		if h, ok := e.Data["hidden"]; ok {
			if b, ok := h.(*interpreter.Bool); ok && b.Value {
				continue
			}
		}
		x, y := e.X, e.Y
		if g.shakeLeft > 0 {
			x += (rand.Float64() - 0.5) * g.shakePow
			y += (rand.Float64() - 0.5) * g.shakePow
		}
		cmd := drawCmd{
			camX: g.camX, camY: g.camY,
			kind:     "sprite_ex",
			path:     e.Sprite,
			x:        x,
			y:        y,
			scale:    scale,
			rotation: e.Rot,
		}
		if e.FlashUntil > g.world.Tick {
			cmd.tintR, cmd.tintG, cmd.tintB = e.FlashR, e.FlashG, e.FlashB
		}
		g.cmds = append(g.cmds, cmd)

		if e.HasBar && e.MaxHP > 0 {
			pct := float64(e.HP) / float64(e.MaxHP)
			if pct < 0 {
				pct = 0
			}
			if pct > 1 {
				pct = 1
			}
			bx := e.X + e.BarDX - e.BarW/2
			by := e.Y + e.BarDY
			col := "#2ecc71"
			if pct <= 0.25 {
				col = "#e74c3c"
			} else if pct <= 0.55 {
				col = "#f39c12"
			}
			g.cmds = append(g.cmds,
				drawCmd{camX: g.camX, camY: g.camY, kind: "rect",
					x: bx - 1, y: by - 1, w: e.BarW + 2, h: 5, color: "#1a1e26"},
				drawCmd{camX: g.camX, camY: g.camY, kind: "rect",
					x: bx, y: by, w: e.BarW * pct, h: 3, color: col},
			)
		}
	}
}
