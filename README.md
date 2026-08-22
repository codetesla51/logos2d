# logos2d

A tiny 2D game engine where **the game is written in Logos** and Go/Ebiten is
only the runtime underneath. Logos is the scripting language (think LÖVE/Lua or
Godot's GDScript); Go is a thin host that exposes drawing, input, audio, and a
declarative entity system as builtins.

- All game logic lives in a `main.lgs` script.
- Scripters never touch Go. The engine is a fixed set of **builtins** (global
  functions) plus a handful of **lifecycle hooks** the script implements.
- The shipped game is **VOID RUNNER** (`demo/void_runner/main.lgs`) — a vertical arcade
  shooter.

---

## Quick start

```sh
# build (default target uses Wayland; avoids a known GLFW stuck-key bug under XWayland)
make
# or
go run . demo/void_runner/main.lgs

# windowed run convenience
make run

# headless autopilot (bot plays itself; useful for testing)
go run . demo/void_runner/main.lgs -auto
```

Requirements: Go ≥ 1.22, Ebiten v2, system OpenGL. Audio is `.wav`/`.ogg`/`.mp3`.

Verification (no window needed):

```sh
go test -run TestHeadlessDemo2
```

It chdirs into `demo`, loads the script, runs `on_load`, starts the game, ticks
3000 frames, and pokes every draw hook — failing on any parse/runtime error.

---

## VOID RUNNER (the demo game)

A shoot-'em-up. Fly your ship up the void, shoot rocks and enemy fighters, grab
power-ups, survive escalating waves, then kill the **VOID RUNNER** boss for a win
screen.

### Controls

| Input | Action |
|-------|--------|
| `A` / `D` or `←` / `→` | move left / right (inertia + banking tilt) |
| `SPACE` | fire |
| `B` | toggle autopilot bot (also enabled with `-auto`) |
| `P` | pause / resume |
| `M` | toggle music |
| `ENTER` / `SPACE` / left-click | start / restart from menu, game-over, win |

### Game states

`menu → play → pause` and `play → over` / `play → win`. The world is **frozen**
(via `set_world_paused`) on menu/pause/over/win; only `on_update` input handling
and the draw hooks keep running.

### Entities & groups

- `player` — you. 3 HP, inertia movement, invulnerable 60 ticks after a hit (blinks).
- `meteors` — 4 variants (brown/gray/red/white). 2 HP, knockback, 1/3 chance to drop a power-up.
- `ships` — enemy fighters, 3 HP, 3 personalities (see below).
- `ebullets` — enemy lasers (can be shot down by your bullets).
- `bullets` / `rockets` — your projectiles.
- `powerups` — dropped or trickled; max 2 live at once.
- `boss` — the VOID RUNNER (appears after `BOSS_SCORE` and a survival gate).

### Enemy ship personalities (`spawn_ship`)

- **gunner** (red) — descends, fires straight down on a timer.
- **weaver** (blue) — sine weave, occasional aimed shot.
- **hunter** (green) — seeks toward your column, fires when close.

All use smooth lerp movement (no frame "blinking"), cap at 3 live (4 once
difficulty > 1.6), and never spawn instantly (`ship_armed` delay; waves tighten
as difficulty climbs).

### Weapons / power-ups

Picked up as distinct **pill** sprites with floating text labels above them:

| Pill | Kind | Effect |
|------|------|--------|
| green | `rapid` | faster fire (short cooldown) |
| red | `twin` | twin-shot |
| blue | `shield` | temporary shield bubble (absorbs one hit, then pops) |
| red star | `heart` | +1 life (max 3) |
| yellow | `rocket` | rockets — slow, heavy splash (does **2** dmg to ships, 99 splash to meteors) |

Power-ups last `WEAPON_DURATION` (600 ticks); `heart`/`shield` are
instantaneous/percent.

### Boss & win

The VOID RUNNER appears once you reach `BOSS_SCORE` (1200) **and** survive
`frame_t >= 3600` (~60s), so the combo multiplier can't summon it instantly. It
sweeps side-to-side, fires an aimed shot down your column, and at <50% HP goes
**enraged** (3-way spread). Killing it scores +800 and shows the win screen
("VOID RUNNER DESTROYED" / "the void goes quiet at last" / final score /
blinking "PRESS ENTER TO RUN AGAIN").

### Scoring & combo

Each kill feeds a combo streak (`score_kill`): faster kills = bigger multiplier,
capped at x9, decaying after 150 idle ticks. Meteors = 20, ships = 15, boss = 800,
plus a small survival trickle (+5 every 60 ticks). A combo multiplier makes score
climb fast — which is why the boss has a survival gate, not just a score gate.



---

## The Logos language (essentials)

Logos is a small scripting language. You only need a handful of constructs to
write a game. Full language docs live in the `github.com/codetesla51/logos`
repo; the parts this engine relies on:

```logos
// variables (global by default; `let` is required)
let hp = 3
let name = "blue"

// numbers are int/float; math is what you'd expect
let d = 1.0 + frame_t / 6000.0

// tables = the object type. Keys are strings; read with `t.key` or `t["key"]`
let p = table{x: 160, y: 212, w: 20}
p.x = p.x + 1

// arrays
let parts = []
parts = push(parts, table{x: 1, y: 2})

// functions (closures capture their environment — used heavily for AI/timers)
fn add(a, b) {
    return a + b
}

// if / else
if hp <= 0 {
    state = "over"
} else {
    state = "play"
}

// for-in over arrays / ranges
for i in range(0, 10, 1) { ... }
for s in stars { s.y = s.y + 1 }

// anonymous function literals (passed to collide/every/ent_on_tick)
collide("bullets", "meteors", fn(b, m) {
    kill(b)
    damage(m, 1)
})
```

Gotcha (bitten us more than once): a `let` declared **inside an `if` block** in a
top-level function does **not** bind to the following statement (the next line
sees an undefined identifier). Declare `let` at the function's top level, or
inline the value. `let` inside `for` loops and inside closures is fine.

---

## Lifecycle & hooks

The engine calls these script functions at fixed points. All are optional except
the ones the game actually uses; missing hooks are silently skipped (except
`on_load`, which is expected at startup).

| Hook | When | Purpose |
|------|------|---------|
| `on_load()` | once at startup, and on every hot reload | preload sprites, `register_rules()`, init state |
| `start_game()` | script-defined; called to (re)begin a run | `reset_world()`, spawn the player, set `state = "play"` |
| `on_update(dt)` | every tick, **before** the world simulates | input, spawning, state machine, scoring. Return value ignored. |
| `on_draw_back()` | in `Draw`, before entities | starfield, parallax decor, background |
| `on_draw_front()` | in `Draw`, after entities | HUD, juice (booms/particles), overlays, win/over text |
| `draw_hud()` | convention — called from `on_draw_front` | score, lives, combo, weapon |

The engine `Draw` pipeline is:

```
if world.active (any ECS builtin used):
    call on_draw_back()
    drawEntities()            // engine auto-draws every live entity + HP bars
    call on_draw_front()
else:
    call on_draw()            // legacy: script draws everything itself
```

`dt` passed to `on_update` is `1/60` (the engine is fixed at 60 TPS). Most games
ignore it and count frames with their own `frame_t`.

### Hot reload

Editing `main.lgs` while the game runs **auto-reloads**: the script is re-`Run`,
`on_load` is re-called, and the world is reset. A parse error keeps the old code
running (logged). This is great for iteration but means a half-finished save can
inject broken code into a live session — edit a scratch copy or close the game
first when someone is playing.

---

## Engine facts

- **Logical resolution is 320×240** (`gameW`/`gameH`). The script sees these
  coordinates; `Layout()` returns them. `window_width()`/`window_height()` return
  320/240.
- **Fixed 60 TPS** (`ebiten.SetTPS(60)`) — gameplay speed is deterministic, not
  tied to monitor refresh.
- **World-space drawing**: every `draw_*` command is rendered relative to the
  camera snapshot taken when it was queued. Draw the world, then
  `set_camera(0, 0)` before HUD text so it stays put.
- **Draw order = queue order** (painter's algorithm / z-order). Commands queued
  earlier draw underneath later ones.
- **Sprites load lazily** (or via `preload_sprite`). A missing/blank sprite
  prints a one-time warning and is skipped — it will not crash the game.
- **Audio**: `play_sound` (one-shot SFX) and `play_music` (looping). Music is
  decoded on a background goroutine after the first tick, so calling it from
  `on_load` is safe. Formats: `.wav` / `.ogg` / `.mp3` (OGG recommended — small
  on disk, native decode).
- **Font**: project-local `kenvector_future.ttf` wins; else a system sans
  fallback. If no font is found, `draw_text` is skipped (warned once). Use
  `text_width()` to center text.
- **Sandbox**: the VM has file I/O, network, shell, and `exit` disabled. Scripts
  cannot touch the filesystem or spawn processes — only the registered builtins.
- **Entities are auto-drawn** by `drawEntities()`. To hide one without killing
  it, set its `Data["hidden"] = true`.

---

## Builtins reference

Everything below is a global function callable from `main.lgs`. Signatures use
the Logos types `int`, `float`, `str`, `bool`, `table`, `array`, `id` (opaque
entity id, an `int`), and `fn` (a function/closure value). Coordinates are in the
320×240 logical space unless noted.

### Drawing

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `draw_rect` | `(x, y, w, h, color)` | Filled rectangle; `(x,y)` is **top-left**. `color` is `"#rrggbb"`. |
| `draw_circle` | `(x, y, r, color)` | Filled circle; `(x,y)` is the **center**, `r` is radius (float). |
| `draw_line` | `(x1, y1, x2, y2, color, thickness)` | Line from `(x1,y1)` to `(x2,y2)`; `thickness` float. |
| `draw_text` | `(str, x, y, color)` | Left-aligned text at `(x,y)` (top-left baseline). Use `text_width()` to center. |

Colors are hex strings like `"#f1c40f"`. An empty/`""` color defaults to white.

### Sprites

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `draw_sprite` | `(path, x, y)` | Draw at `(x,y)` **top-left**, native size. |
| `draw_sprite_ex` | `(path, x, y, scale, rotation_degrees)` | `(x,y)` is the **center**; rotates around center (degrees, clockwise); `scale` float (0.5 = half). |
| `draw_sprite_frame` | `(path, x, y, fw, fh, index)` | Draw frame `index` from a **horizontal strip** sheet (frames laid left→right). `(x,y)` top-left, `fw`/`fh` frame size, `index` wraps. |

Sprites carry no tint by default. `draw_sprite_ex` accepts a tint via a separate
engine path only when a tint is set; in practice use `flash()` on entities for
hit-flash.

### Camera & window

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `set_camera` | `(x, y)` | Set world camera offset. All subsequent `draw_*` are offset by it until changed. |
| `camera_x` / `camera_y` | `() -> int` | Current camera offset. |
| `window_width` / `window_height` | `() -> int` | Always 320 / 240. |
| `set_title` | `(str)` | Set the OS window title. |
| `quit` | `()` | Request a clean engine termination (`ebiten.Termination`). |

### Input

Keyboard names: `right`, `left`, `up`, `down`, `space`, `enter`, `escape`, `tab`,
`shift`, `ctrl`, `w`, `a`, `s`, `d`, `m`, `p`. Mouse buttons: `left`, `right`,
`middle`.

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `key_down` | `(name) -> bool` | True while held. |
| `key_pressed` | `(name) -> bool` | True only on the frame the key went down (edge). |
| `key_released` | `(name) -> bool` | True only on the frame the key went up (edge). |
| `mouse_pos` | `() -> table{x, y}` | Cursor in logical 320×240 pixels. |
| `mouse_down` | `(button) -> bool` | Held. |
| `mouse_pressed` | `(button) -> bool` | Edge (pressed this frame). |

(Note: there is no `mouse_released` builtin — only down/pressed.)

### Audio

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `play_sound` | `(path)` | One-shot SFX (`.wav`/`.ogg`/`.mp3`), decoded and cached. |
| `play_music` | `(path)` | Looping music; safely deferred to the first update tick. |
| `stop_music` | `()` | Stop and release the music player. |

### Math & geometry

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `math_sin` / `math_cos` | `(radians) -> float` | Trig. Angles in **radians**. |
| `atan2` | `(y, x) -> float` | Returns **degrees** (pairs with `draw_sprite_ex` rotation). |
| `abs` | `(x) -> float` | Absolute value. |
| `distance` | `(x1, y1, x2, y2) -> float` | Euclidean distance. |
| `random` | `(min, max) -> int` | Inclusive random integer; swaps args if `max < min`. |
| `point_in_rect` | `(px, py, x, y, w, h) -> bool` | Point inside AABB (top-left `x,y`). |
| `rects_overlap` | `(x1,y1,w1,h1, x2,y2,w2,h2) -> bool` | AABB overlap test. |
| `text_width` | `(str) -> int` | Pixel width of `str` in the current font (for centering). |
| `len` | `(array|str|table) -> int` | Element/char/pair count. |

### Entity system (ECS)

Entities are opaque integer ids. Fixed properties live in struct fields; any
other key you pass in `create` lands in the entity's `Data` map for your own use.
The engine auto-draws every live entity each frame (sprite + optional HP bar +
hit-flash + shake jitter), so you never draw entities yourself.

#### Spawn / death

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `create` | `(group, sprite, x, y, props_table) -> id` | Spawn an entity. `props` keys: `x,y,vx,vy,rot,spin,scale,w,h,ttl,hp,max_hp` plus any custom key. `w`/`h` become **half-extents** (`HW=w/2`); pass the sprite's full width/height for an accurate hitbox. Defaults: `hp=1, HasHP=true, ttl=-1` (infinite), `scale=1`, `MaxHP=spawn hp`. |
| `kill` | `(id)` | Mark dead immediately; fires `on_death` once. |
| `damage` | `(id, n) -> bool` | `HP -= n`; kills (and returns `true`) if `HasHP` and `HP <= 0`. |
| `heal` | `(id, n)` | `HP += n`, capped at `MaxHP` (if set). |
| `max_hp` | `(id, n)` | Set `MaxHP` and `HasHP=true`. |
| `invuln_after` | `(id, ticks)` | Ignore collisions until `world.Tick + ticks`. Used to debounce player hits. |
| `on_death` | `(id, fn)` | One-shot handler `fn(id)`, fired by `kill`/`damage`/ttl expiry (NOT by offscreen cull). |
| `ttl_left` | `(id) -> int` | Remaining time-to-live ticks (`-1` if unset). |

#### Read / write entity state

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `ent_get` | `(id, key) -> value` | Read `x,y,vx,vy,rot,spin,scale,hp,sprite,group` or any `Data` key. Returns `Null` if missing. |
| `ent_set` | `(id, key, value)` | Set a core or `Data` field. Setting `hp` also sets `HasHP=true`. |

#### Per-entity fx & steering

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `hp_bar` | `(id, dx, dy, w)` | Attach an HP bar at offset `(dx,dy)` (relative to entity center), width `w`. Auto-drawn, green→orange→red by %. |
| `flash` | `(id, "#rrggbb", ticks)` | Tint the sprite (hit-flash) until `Tick + ticks`. |
| `shake` | `(power, ticks)` | Screen shake; jitters entity draw positions, decays each frame. |
| `knockback` | `(id, fx, fy, force)` | Push the entity away from point `(fx,fy)`. |
| `seek` | `(id, x, y, speed)` | Set velocity toward `(x,y)` at `speed` (arrives exactly if within `speed`, i.e. no orbiting). |
| `run_behavior` | `(id, tree)` | First-class declarative behavior tree (see below). Call every tick from an `ent_on_tick` closure. |
| `ent_on_tick` | `(id, fn)` | Per-entity steering closure `fn(id)`, called every tick after motion. Use for AI. |

#### Behavior trees (`run_behavior`)

Declarative AI nodes; the engine evaluates the tree each call and finishes
with exactly one seek-style steer toward the nearest `player` entity.

```
# condition node: test, then recurse into one branch
run_behavior(id, table{cond: "hp_below", val: 2,
                       "then": table{type: "flee", spd: 2.4},
                       "else": table{type: "chase", spd: 2.6, y: 60}})
# action node: chase = steer to the player (or altitude line y)
#               flee  = steer to a mirrored away-point, clamped on-screen
```

- Conditions: `hp_below` (`val`), `player_near` (`val` = distance). Branches
  may nest more condition nodes.
- Action fields: `spd` (default `2.0`), optional `y` fixed altitude line
  instead of tracking the player's y.
- Keys `"then"`/`"else"` must stay QUOTED — `else` is a reserved word in
  Logos, so bare `else:` / `.else` won't parse.

#### Queries

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `nearest` | `(group, x, y) -> id` | Id of the closest live entity in `group` (`-1` if none). |
| `group_each` | `(group, fn)` | Call `fn(id)` for each live entity in `group`. |
| `group_count` | `(group) -> int` | Number of live entities in `group`. |
| `group_ids` | `(group) -> array` | Array of live entity ids in `group`. |

#### Area & world

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `area_damage` | `(x, y, radius, dmg, group)` | Damage every entity in `group` within `radius` (squared distance). Kills those that drop to ≤0 HP. Great for explosions/splashes. |
| `reset_world` | `()` | Clear all entities + timers. **Keeps** `collide` rules (they're declared in `on_load`). |
| `set_world_paused` | `(bool)` | Freeze/unfreeze the whole simulation (timers, motion, collisions, steering). Used for menu/pause/over/win. |

### Rules & timers

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `collide` | `("a", "b", fn(a_id, b_id))` | Fire `fn` whenever an `a` overlaps a `b`. Pairs where either side is invulnerable are skipped. Register in `on_load` (persists across `reset_world`). |
| `every` | `(interval, fn)` | Repeat `fn()` every `interval` ticks, forever. |
| `after` | `(interval, fn)` | Call `fn()` once after `interval` ticks. |
| `after_n` | `(interval, count, fn)` | Call `fn()` `count` times, every `interval` ticks, then stop. |
| `ent_on_tick` | `(id, fn)` | See per-entity steering above. |

Timers registered *from inside* a firing timer are handled safely (the engine
snapshots the timer list before invoking callbacks, so new timers don't corrupt
the iteration).

### Utility (remaining)

| Builtin | Signature | Notes |
|---------|-----------|-------|
| `cli_flag` | `(name) -> bool` | True if `-name` was passed on the command line (e.g. `-auto`). |
| `preload_sprite` | `(path) -> bool` | Load + decode a sprite now; returns `true` on success. Call in `on_load` to avoid first-frame hitch. |

---

## How collisions actually work

- Hitboxes are **axis-aligned boxes centered on `(x,y)`**. `overlaps(a,b)` is:
  `abs(a.X-b.X) < a.HW+b.HW && abs(a.Y-b.Y) < a.HH+b.HH`.
- `HW = w/2`, `HH = h/2` from the `create` props. **Set `w`/`h` to the sprite's
  full pixel size** — tiny values (e.g. 4×4) make bullets pass *through* enemies
  because the boxes never overlap. (This was a real, confusing bug.)
- A `collide` rule's closure runs once per **overlapping pair per frame**, unless
  either entity is invulnerable (`InvUntil > Tick`) — so a bullet won't "multi-hit"
  a ship every frame; and `invuln_after(pid, 60)` stops the player taking repeated
  hits while blinking.
- Entities that drift far offscreen are **silently culled** (margin 90px right/
  left/bottom, 270px above). Culling does **not** fire `on_death` — don't rely on
  death logic for things that fly off the edge.
- `on_death` fires exactly once, from `kill()`, `damage()` (lethal), or TTL expiry.

---

## Gotchas & lessons

1. **`let` inside `if` doesn't bind** the next statement in a top-level function.
   Declare `let` at the function top, or inline the value. (`let` in `for` loops
   and closures is fine.)
2. **Default entity HP is 1** — a fresh `create` with no `hp` dies on the first
   `damage`. Give ships/bosses `hp: N`.
3. **`overlaps` is centered** — size `w`/`h` to the sprite, not a guess.
4. **`area_damage(..., 99, "ships")` one-shots** whatever it hits (99 > any HP).
   Use a smaller number if you want ships to survive a splash.
5. **`frame_t` keeps counting** during menu/pause/over (only the *world* pauses).
   Difficulty/`diff()` tied to `frame_t` will creep even on non-play screens —
   cosmetic, but be aware.
6. **`mouse_released` is not a builtin** — only `mouse_down` / `mouse_pressed`.
7. **Hot reload injects edits into a live game** — edit a scratch copy or have the
   player close the game first, or you'll ship half-finished code into their run.
8. **Bosses/big entities need a big `w`/`h`** or they're unhittable; attach an
   `hp_bar` with a negative `dy` (above the sprite).

---

## Project layout

```
main.go        wires VM + Game + bindings, RunGame (~25 lines)
engine.go      Game struct, drawCmd queue, Update/Draw/Layout, input polling,
               font/sprite/audio helpers, hot reload
vm.go          newVM() (sandboxed), loadScript(), callScript()
bindings.go    ALL vm.Register builtins (this is the API surface)
ecs.go         World: entities, timers, collide rules, simulate(), drawEntities()
memory.md      agent working notes (bug stories, status, conventions)
README.md      this file
demo/
  main.lgs     the VOID RUNNER game script (all gameplay lives here)
  *.png,*.wav,*.ogg   game assets (gitignored — copy them in; don't commit)
headless_check_test.go  go test -run TestHeadlessDemo2  (verification harness)
```

Build targets (from `Makefile`): `make` (Wayland, default), `make x11`,
`make run`. Engine entry: `go run . <scriptdir>/main.lgs [-auto]`.

---

## Verifying changes

After editing `demo/void_runner/main.lgs`, run the headless harness (no window needed):

```sh
go test -run TestHeadlessDemo2
```

It loads the script, runs `on_load`, calls `start_game`, ticks 3000 frames with
the bot, then pokes `on_draw_back` / `on_draw_front` / `draw_hud`. Any parse or
runtime error fails the test — this is how syntax/logic regressions surface
without a human at the keyboard.

For real feel (difficulty curve, boss timing, input), a human must play it:
launch `go run . demo/void_runner/main.lgs` and exercise menu → play → pause → over → win.


