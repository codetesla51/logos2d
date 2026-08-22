# logos2d

> A tiny 2D game engine where **the game is written in Logos** and Go + Ebiten
> are only the runtime underneath.

![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)
![Ebiten](https://img.shields.io/badge/Built%20with-Ebiten-00ADD8)
![Platform](https://img.shields.io/badge/platform-Linux%20%2F%20macOS%20%2F%20Windows-4d4d4d)

`logos2d` is a small, opinionated engine for building arcade-style 2D games.
You write all gameplay — spawning, collisions, AI, HUD, state machines — in a
`main.lgs` script. The Go/Ebiten layer is a thin, fixed host that exposes
drawing, input, audio, and a **declarative entity system** as builtins.
Scripters never touch Go.

Two complete games ship in `demo/`: **VOID RUNNER** (a vertical shoot-'em-up)
and **BREAKOUT** (brick-breaker with pickups, random layouts, and level
progression).

> [!NOTE]
> This repo is the engine *and* its demos. The engine is the handful of Go
> files (`engine.go`, `ecs.go`, `bindings.go`, `vm.go`, `main.go`); everything
> you'd call "a game" lives under `demo/<name>/main.lgs`.

---

## Quick start

```sh
# Build (default target uses Wayland — avoids a known GLFW stuck-key bug
# under XWayland). The binary is ./logos2d.
make

# Run a demo (engine takes one arg: a path to a script; it chdirs into that
# folder so asset paths resolve relatively)
make run           # VOID RUNNER
make breakout      # BREAKOUT
go run . demo/void_runner/main.lgs -auto   # headless autopilot (bot plays)

# Build with the X11 backend instead of Wayland
make x11
```

**Requirements:** Go 1.22+, [Ebiten v2](https://ebiten.org/), and a system with
OpenGL. Audio accepts `.wav` / `.ogg` / `.mp3` (OGG recommended).

**Verify without a window** (loads both demos, runs `on_load`, starts the game,
ticks thousands of frames, pokes every draw hook):

```sh
go test ./...
```

---

## Features

- **Declarative ECS.** `create` / `collide` / `every` / `ent_on_tick` declare
  behavior; a fixed-order pipeline runs it every tick.
- **Immediate-mode drawing.** `draw_rect` / `draw_circle` / `draw_line` /
  `draw_text` / `draw_sprite*` queue commands; the engine auto-draws every
  entity (sprite + optional HP bar + hit-flash + shake).
- **Sandboxed scripting.** The Logos VM has file I/O, network, shell, and
  `exit` disabled — a script can only call registered builtins, never the OS.
- **Fixed determinism.** 320×240 logical space, 60 TPS, windowed at 640×480.
  Gameplay speed is independent of monitor refresh.
- **Behavior trees.** `run_behavior(id, tree)` is a first-class declarative AI
  builtin (conditions `hp_below` / `player_near`, actions `chase` / `flee`).
- **Hot reload.** Edit `main.lgs` while the game runs and it reloads live;
  parse errors keep the last good version running.
- **Headless harness.** A `Game.InjectKey` hook lets tests drive menus and
  launches with no window — see `headless_check_test.go`.

---

## Demos

### VOID RUNNER — `demo/void_runner/`

A vertical arcade shooter: fly up the void, shoot rocks and enemy fighters,
grab power-up pills, survive escalating waves, then kill the **VOID RUNNER**
boss for a win screen.

| Input | Action |
|-------|--------|
| `A` / `D` or `←` / `→` | move left / right (inertia + banking tilt) |
| `SPACE` | fire |
| `B` | toggle autopilot bot (also enabled with `-auto`) |
| `P` | pause / resume |
| `M` | toggle music |
| `ENTER` / `SPACE` / left-click | start / restart from menu, game-over, win |

Entities: `player` (3 HP, blinks 60 ticks after a hit), `meteors` (4 variants,
may drop power-ups), `ships` (3 personalities: gunner / weaver / hunter),
`boss` (sweeps, aimed shots, enrages < 50% HP), plus bullets, rockets, and
power-up pills (`rapid` / `twin` / `shield` / `heart` / `rocket`). Score feeds
a combo multiplier (capped x9) that decays when idle.

### BREAKOUT — `demo/breakout/`

A brick-breaker with a pre-launch serve, special bricks that drop pickups,
randomized layouts every level, and level progression.

| Input | Action |
|-------|--------|
| `←` / `→` or `A` / `D` | move paddle |
| `SPACE` | launch the served ball |
| `ENTER` | start / restart, or advance to the next level after a win |
| `B` | toggle autopilot bot (also enabled with `-auto`) |

The autopilot mirrors VOID RUNNER's bot: it tracks the lowest ball, predicts
where it will cross the paddle line (folding wall bounces), and steers under
it — auto-launching, clearing levels, and retrying on game over. Run
`go run . demo/breakout/main.lgs -auto` to watch it play itself.

Bricks are randomized per level; **star bricks** drop one of four pickups that
fall for the paddle to catch:

| Pickup | Effect |
|--------|--------|
| `LASER` | balls auto-fire at the nearest brick |
| `MULTI` | spawns two extra balls |
| `POWER` | balls smash through and deal double damage |
| `COLOR` | repaints the balls |

Clear all bricks to win → `ENTER` advances to the next level (keeps score and
lives; ball speed scales up, capped). Running out of balls costs a life; zero
lives ends the run.

---

## How it works

Four layers, with a hard boundary at `bindings.go`:

```
Ebiten       60 FPS loop, window, raw GPU draw, input, audio
  │
Logos VM     parse + eval .lgs, holds fns/lets, runs closures
  │
bindings.go  translate Logos values ↔ engine; mutate World; queue draws   ← API surface
  │
engine.go    Game struct, Update()/Draw(), input poll, hot reload, audio
ecs.go       World, Entity, simulate(), collide rules, drawEntities()
  │
demo/*.lgs   on_load / on_update / on_draw_* / behavior trees = all game logic
```

A script declares **what** (entities, collision rules, timers, per-entity
steering) and the engine's `simulate()` pipeline runs it in a fixed order each
tick: advance `Tick` → fire timers → integrate motion (`x += vx`) → expire
TTL → run per-entity steering closures → resolve collisions → death/HP cull →
compact. Script-side decisions (bounce math, state machine, HUD) are still
written as ordinary imperative code — `logos2d` is a declarative ECS with an
imperative scripting surface, not "everything is declarative."

### Lifecycle hooks

The engine calls these script functions at fixed points. Missing hooks are
silently skipped; `on_load` is expected at startup (and re-called on every hot
reload).

| Hook | When | Purpose |
|------|------|---------|
| `on_load()` | once at startup / each reload | preload sprites, `load_font`, `register_rules()`, init state |
| `start_game()` | script-defined | `reset_world()`, spawn, set `state = "play"` |
| `on_update(dt)` | every tick, **before** the world simulates | input, spawning, state machine, scoring |
| `on_draw_back()` | in `Draw`, before entities | background, starfield, parallax |
| `on_draw_front()` | in `Draw`, after entities | HUD, particles, overlays, win/over text |
| `on_draw()` | legacy (only if no ECS builtins used) | script draws everything itself |

`dt` is always `1/60`. The `Draw` pipeline picks `on_draw_back → entities →
on_draw_front` when any ECS builtin has run, else the legacy `on_draw`.

---

## Scripting in Logos

Logos is a small, curly-braced scripting language. You need only a handful of
constructs to write a game:

```logos
let hp = 3                       // `let` is required; globals by default
let p = table{x: 160, y: 212}    // tables are the object type; keys are strings
p.x = p.x + 1                    // read with p.x or p["x"]

fn add(a, b) { return a + b }    // closures capture their environment

if hp <= 0 { state = "over" } else { state = "play" }

for i in range(0, 10, 1) { ... }
for s in stars { s.y = s.y + 1 }

// anonymous closures are passed to collide / every / ent_on_tick
collide("bullets", "meteors", fn(b, m) {
    kill(b)
    damage(m, 1)
})
```

> [!WARNING]
> A `let` declared **inside an `if` block** in a top-level function does not
> bind to the following statement. Declare `let` at the function top level, or
> inline the value. (`let` inside `for` loops and closures is fine.)

> [!NOTE]
> `else` is a reserved word. When building behavior-tree tables, keep the keys
> **quoted**: `table{cond:"hp_below", val:2, "then":…, "else":…}`.

---

## Builtins reference

All are global functions callable from `main.lgs`. Coordinates are in the
320×240 logical space. Types: `int`, `float`, `str`, `bool`, `table`,
`array`, `id` (opaque int entity id), `fn`.

### Drawing

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `draw_rect` | `(x, y, w, h, color)` | Filled rect; `(x,y)` top-left; `color` `"#rrggbb"`. |
| `draw_circle` | `(x, y, r, color)` | Filled circle; `(x,y)` center, `r` radius. |
| `draw_line` | `(x1, y1, x2, y2, color, thickness)` | Line; `thickness` float. |
| `draw_text` | `(str, x, y, color)` | Left-aligned; use `text_width()` to center. Skipped until a font loads. |
| `draw_sprite` | `(path, x, y)` | Top-left, native size. |
| `draw_sprite_ex` | `(path, x, y, scale, rot_deg)` | Center; rotates (deg, CW). |
| `draw_sprite_frame` | `(path, x, y, fw, fh, index)` | Frame `index` from a horizontal strip; wraps. |
| `load_font` | `(path, size) -> bool` | Load TTF (cached per path+size); becomes active face. |
| `text_width` | `(str) -> int` | Pixel width in current font (0 if none). |
| `preload_sprite` | `(path) -> bool` | Decode a sprite now; call in `on_load`. |

### Camera & window

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `set_camera` | `(x, y)` | World camera offset for all subsequent `draw_*`. |
| `camera_x` / `camera_y` | `() -> int` | Current camera offset. |
| `window_width` / `window_height` | `() -> int` | Always 320 / 240. |
| `set_title` | `(str)` | OS window title. |
| `quit` | `()` | Request clean engine termination. |

### Input

Keyboard names: `right left up down space enter escape tab shift ctrl w a s d m p`.
Mouse buttons: `left right middle`.

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `key_down` | `(name) -> bool` | Held. |
| `key_pressed` | `(name) -> bool` | Edge (down this tick). |
| `key_released` | `(name) -> bool` | Edge (up this tick). |
| `mouse_pos` | `() -> table{x, y}` | Cursor in logical pixels. |
| `mouse_down` | `(button) -> bool` | Held. |
| `mouse_pressed` | `(button) -> bool` | Edge. |

> [!NOTE]
> There is **no** `mouse_released` builtin — only `mouse_down` / `mouse_pressed`.

### Audio

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `play_sound` | `(path)` | One-shot SFX (`.wav`/`.ogg`/`.mp3`), cached. |
| `play_music` | `(path)` | Looping music; decoded on a background goroutine after the first tick. |
| `stop_music` | `()` | Stop and release the music player. |

### Math & geometry

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `math_sin` / `math_cos` | `(rad) -> float` | Trig (radians). |
| `atan2` | `(y, x) -> float` | Returns **degrees** (pairs with `draw_sprite_ex`). |
| `abs` | `(x) -> float` | Absolute value. |
| `distance` | `(x1,y1,x2,y2) -> float` | Euclidean distance. |
| `random` | `(min, max) -> int` | Inclusive int; swaps args if `max < min`. |
| `point_in_rect` | `(px,py,x,y,w,h) -> bool` | Point inside AABB (top-left). |
| `rects_overlap` | `(x1,y1,w1,h1, x2,y2,w2,h2) -> bool` | AABB overlap. |
| `len` | `(array\|str\|table) -> int` | Count. |
| `cli_flag` | `(name) -> bool` | True if `-name` was passed on the command line. |

### Entity system (ECS)

Entities are opaque `int` ids. Fixed props live in struct fields; any other key
passed to `create` lands in the entity's `Data` map. The engine auto-draws every
live entity each frame, so you never draw entities yourself.

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `create` | `(group, sprite, x, y, props) -> id` | Spawn. `props` (a `table{}`, required) keys: `vx,vy,rot,spin,scale,w,h,ttl,hp,max_hp` + custom. `w`/`h` are **full** extents (halved into half-extents). Defaults `hp=1, HasHP=true, ttl=-1`. |
| `kill` | `(id)` | Mark dead; fires `on_death` once. |
| `damage` | `(id, n) -> bool` | `HP -= n`; kills (returns `true`) if `HasHP && HP<=0`. |
| `heal` | `(id, n)` | `HP += n`, capped at `MaxHP`. |
| `max_hp` | `(id, n)` | Set `MaxHP`, `HasHP=true`. |
| `invuln_after` | `(id, ticks)` | Ignore collisions until `Tick + ticks` (debounce). |
| `on_death` | `(id, fn)` | One-shot `fn(id)`; fired by `kill`/`damage`/TTL, **not** off-screen cull. |
| `ttl_left` | `(id) -> int` | Remaining TTL ticks (`-1` if unset). |
| `ent_get` | `(id, key) -> value` | Read core or `Data` field (`Null` if missing). |
| `ent_set` | `(id, key, value)` | Set core or `Data` field (setting `hp` sets `HasHP`). |
| `ent_on_tick` | `(id, fn)` | Per-entity steering closure `fn(id)`, called each tick after motion. |
| `hp_bar` | `(id, dx, dy, w)` | HP bar at offset `(dx,dy)`, width `w`; green→orange→red. |
| `flash` | `(id, "#rrggbb", ticks)` | Hit-flash tint until `Tick + ticks`. |
| `shake` | `(power, ticks)` | Screen shake; jitters entity draws, decays per frame. |
| `knockback` | `(id, fx, fy, force)` | Push away from point `(fx,fy)`. |
| `seek` | `(id, x, y, speed)` | Steer toward `(x,y)` at `speed` (arrives exactly; no orbiting). |
| `run_behavior` | `(id, tree)` | Declarative behavior tree (see below). |
| `nearest` | `(group, x, y) -> id` | Closest live entity in `group` (`-1` if none). |
| `group_count` | `(group) -> int` | Live count. |
| `group_ids` | `(group) -> array` | Live ids. |
| `group_each` | `(group, fn)` | `fn(id)` for each live entity. |
| `area_damage` | `(x, y, radius, dmg, group)` | Damage all in `group` within `radius` (squared). |
| `collide` | `("a","b", fn(aId,bId))` | Fire `fn` on A×B overlap; invulnerable pairs skipped. Register in `on_load` (persists across `reset_world`). |
| `every` | `(interval, fn)` | Repeat `fn` every `interval` ticks. |
| `after` | `(interval, fn)` | `fn` once after `interval` ticks. |
| `after_n` | `(interval, count, fn)` | `fn` `count` times, every `interval`, then stop. |
| `reset_world` | `()` | Clear entities + timers; **keeps** `collide` rules. |
| `set_world_paused` | `(bool)` | Freeze/unfreeze the whole simulation. |

#### Behavior trees (`run_behavior`)

Declarative AI; the engine evaluates the tree each call and finishes with exactly
one steer toward the nearest `player` entity.

```
run_behavior(id, table{cond:"hp_below", val:2,
                       "then": table{type:"flee", spd:2.4},
                       "else": table{type:"chase", spd:2.6, y:60}})
```

- Conditions: `hp_below` (`val`), `player_near` (`val` = distance). Branches
  may nest.
- Actions: `chase` steers to the player (or a fixed `y` altitude line); `flee`
  steers to a mirrored away-point, clamped on-screen. `spd` defaults to `2.0`.
- `"then"` / `"else"` **must stay quoted** (`else` is reserved).

### How collisions work

- Hitboxes are **axis-aligned boxes centered on `(x,y)`**:
  `abs(a.X-b.X) < a.HW+b.HW && abs(a.Y-b.Y) < a.HH+b.HH`, with `HW=w/2`. Pass
  the sprite's **full** pixel size for `w`/`h` — tiny values let fast bullets
  tunnel through enemies.
- A `collide` closure fires once per overlapping pair per frame unless either
  side is invulnerable, so a bullet won't multi-hit a ship every frame.
- Entities that drift far off-screen are **silently culled** (no `on_death`).
- `on_death` fires exactly once (from `kill`, lethal `damage`, or TTL expiry).

---

## Project layout

```
main.go                  wires VM + Game + bindings, then RunGame (~25 lines)
engine.go                Game struct, drawCmd queue, Update/Draw/Layout, input,
                          font/sprite/audio helpers, hot reload
vm.go                    newVM() (sandboxed), loadScript(), callScript()
bindings.go              ALL vm.Register builtins — this is the API surface
ecs.go                   World, Entity, simulate(), collide rules, drawEntities()
headless_check_test.go   go test ./...  (verification: TestHeadlessDemo2 / -Breakout)
memory.md                agent working notes (bug stories, status, conventions)
demo/
  void_runner/main.lgs   the VOID RUNNER game script + assets
  breakout/main.lgs      the BREAKOUT game script + assets
```

---

## Limitations

- **No vectors/gravity/constraints.** Balls move at constant speed; "physics"
  is hand-written scalar math.
- **No volume control** on `play_sound` / `play_music` (full gain only).
- **No sprite tinting** beyond the `flash()` hit-flash; no normal/round
  hitboxes (AABB only).
- **Brute-force collisions** — O(A×B) per group pair per tick. Fine for
  hundreds of entities, not thousands.
- **Behavior trees are minimal** — 2 conditions × 2 actions, no sequences,
  selectors, or blackboards.
- **No persistence** — nothing writes to disk (sandbox forbids file I/O), no
  high-score table.
- **Hot reload** resets script state to defaults on each reload.

---

## Topics

Suggested GitHub repository topics (set these when the repo is published):

`game-engine` · `ebiten` · `golang` · `2d-game-engine` · `logos` ·
`game-dev` · `ecs` · `scripting-language` · `breakout` · `shoot-em-up`

---

## Credits

`logos2d` would not exist without the **Logos** scripting language and virtual
machine that it hosts:

- **Logos** — <https://github.com/codetesla51/logos> (v0.4.9)

Logos provides the parser, interpreter, and sandbox; `logos2d` adds the Ebiten
runtime and the game-engine builtins on top of it.

## License

Released under the [MIT License](LICENSE) — copyright © 2026 Oladele Usman.
Built on the [Logos](https://github.com/codetesla51/logos) scripting language
(v0.4.9), which carries its own separate license.
