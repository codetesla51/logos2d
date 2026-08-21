package main

import (
	"bytes"
	"fmt"
	"github.com/codetesla51/logos/interpreter"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codetesla51/logos/logos"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// logical game resolution (what the script sees as coordinates)
const (
	gameW = 320
	gameH = 240
)

// drawCmd is one queued drawing primitive, filled by the Logos draw_*
// builtins during on_draw and rendered by Game.Draw.
type drawCmd struct {
	kind              string // "rect" | "circle" | "line" | "text" | "sprite"
	x, y, w, h        float64
	x2, y2            float64
	radius, thickness float64
	scale, rotation   float64 // rotation in degrees
	camX, camY        float64 // camera snapshot taken when the cmd was queued
	str, path         string
	color             string
}

type Game struct {
	vm                   *interpreter.Interpreter
	world                World
	cmds                 []drawCmd
	sprites              map[string]*ebiten.Image
	failed               map[string]bool // sprite paths that failed to load (warn once)
	face                 font.Face
	keysCurr, keysPrev   map[ebiten.Key]bool
	mouseCurr, mousePrev map[ebiten.MouseButton]bool
	quitRequested        bool
	camX, camY           float64
	audioCtx             *audio.Context
	sfxPCM               map[string][]byte // decoded sound effects by path
	failedAudio          map[string]bool   // audio paths that failed to load (warn once)
	musicPlayer          *audio.Player
	scriptMod            time.Time // main.lgs mtime for hot reload
}

func newGame(vm *interpreter.Interpreter) *Game {
	g := &Game{
		vm:          vm,
		world:       *newWorld(),
		sprites:     map[string]*ebiten.Image{},
		failed:      map[string]bool{},
		face:        loadFont(),
		keysCurr:    map[ebiten.Key]bool{},
		keysPrev:    map[ebiten.Key]bool{},
		mouseCurr:   map[ebiten.MouseButton]bool{},
		mousePrev:   map[ebiten.MouseButton]bool{},
		audioCtx:    audio.NewContext(44100),
		sfxPCM:      map[string][]byte{},
		failedAudio: map[string]bool{},
	}
	if st, err := os.Stat("main.lgs"); err == nil {
		g.scriptMod = st.ModTime()
	}
	if g.face == nil {
		fmt.Println("warning: no system font found, draw_text will be skipped")
	}
	return g
}

func hexToRGBA(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return color.RGBA{byte(r), byte(g), byte(b), 255}
}

// toF accepts either a Logos integer or float where a number is expected.
func toF(o logos.Object) float64 {
	switch v := o.(type) {
	case *logos.Integer:
		return float64(v.Value)
	case *logos.Float:
		return v.Value
	}
	return 0
}

func toI(o logos.Object) int64 {
	if v, ok := o.(*logos.Integer); ok {
		return v.Value
	}
	return int64(toF(o))
}

// loadFont tries common system font locations; returns nil if none found.
func loadFont() font.Face {
	// project-local arcade font wins if present
	candidates := []string{
		"kenvector_future.ttf",
		"/usr/share/fonts/Adwaita/AdwaitaSans-Regular.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans.ttf",
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		ft, err := opentype.Parse(b)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: 12, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			continue
		}
		return face
	}
	return nil
}

// sprite loads and caches an image by path; warns once on failure.
func (g *Game) sprite(path string) *ebiten.Image {
	if img, ok := g.sprites[path]; ok {
		return img
	}
	img, _, err := ebitenutil.NewImageFromFile(path)
	if err != nil {
		if !g.failed[path] {
			fmt.Println("draw_sprite: failed to load", path, "-", err)
			g.failed[path] = true
		}
		return nil
	}
	g.sprites[path] = img
	return img
}

// pollInput snapshots held-state and swaps previous/current so pressed/
// released edges can be computed per frame. Must run once per tick before
// on_update.
func (g *Game) pollInput() {
	g.keysPrev = g.keysCurr
	g.keysCurr = make(map[ebiten.Key]bool)
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if ebiten.IsKeyPressed(k) {
			g.keysCurr[k] = true
		}
	}

	g.mousePrev = g.mouseCurr
	g.mouseCurr = make(map[ebiten.MouseButton]bool)
	for _, b := range []ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonRight, ebiten.MouseButtonMiddle} {
		if ebiten.IsMouseButtonPressed(b) {
			g.mouseCurr[b] = true
		}
	}
}

func (g *Game) keyDown(k ebiten.Key) bool {
	return g.keysCurr[k]
}

func (g *Game) keyPressed(k ebiten.Key) bool {
	return g.keysCurr[k] && !g.keysPrev[k]
}

func (g *Game) keyReleased(k ebiten.Key) bool {
	return !g.keysCurr[k] && g.keysPrev[k]
}

func (g *Game) mouseDown(b ebiten.MouseButton) bool {
	return g.mouseCurr[b]
}

func (g *Game) mousePressed(b ebiten.MouseButton) bool {
	return g.mouseCurr[b] && !g.mousePrev[b]
}

// loadAudio decodes a .wav/.mp3 file into raw PCM bytes, cached by path.
func (g *Game) loadAudio(path string) ([]byte, error) {
	if pcm, ok := g.sfxPCM[path]; ok {
		return pcm, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if !g.failedAudio[path] {
			fmt.Println("audio:", err)
			g.failedAudio[path] = true
		}
		return nil, err
	}
	defer f.Close()

	var pcm []byte
	switch {
	case strings.HasSuffix(path, ".wav"):
		s, derr := wav.DecodeWithSampleRate(g.audioCtx.SampleRate(), f)
		if derr != nil {
			return nil, derr
		}
		pcm, err = io.ReadAll(s)
	case strings.HasSuffix(path, ".mp3"):
		s, derr := mp3.DecodeWithSampleRate(g.audioCtx.SampleRate(), f)
		if derr != nil {
			return nil, derr
		}
		pcm, err = io.ReadAll(s)
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", path)
	}
	if err != nil {
		return nil, err
	}
	g.sfxPCM[path] = pcm
	return pcm, nil
}

func (g *Game) playSound(path string) {
	pcm, err := g.loadAudio(path)
	if err != nil {
		fmt.Println("play_sound:", err)
		return
	}
	p := g.audioCtx.NewPlayerFromBytes(pcm)
	p.Play() // finished players are garbage-collected by the audio context
}

func (g *Game) playMusic(path string) {
	fmt.Println("[music] 1 load")
	pcm, err := g.loadAudio(path)
	if err != nil {
		fmt.Println("play_music:", err)
		return
	}
	fmt.Println("[music] 2 loop")
	loop := audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm)))
	fmt.Println("[music] 3 newplayer")
	p, err := g.audioCtx.NewPlayer(loop)
	fmt.Println("[music] 4 created")
	if err != nil {
		fmt.Println("play_music:", err)
		return
	}
	if g.musicPlayer != nil {
		g.musicPlayer.Close()
	}
	p.Play()
	g.musicPlayer = p
}

func (g *Game) stopMusic() {
	if g.musicPlayer != nil {
		g.musicPlayer.Close()
		g.musicPlayer = nil
	}
}

// checkHotReload watches main.lgs and re-runs the script when it changes.
// Parse errors keep the old code running until the next valid save.
func (g *Game) checkHotReload() {
	st, err := os.Stat("main.lgs")
	if err != nil || !st.ModTime().After(g.scriptMod) {
		return
	}
	g.scriptMod = st.ModTime()
	source, err := os.ReadFile("main.lgs")
	if err != nil {
		return
	}
	if err := g.vm.Run(string(source)); err != nil {
		fmt.Println("[hot-reload] error (keeping old code):", err)
		return
	}
	g.world.reset()
	closureErrShown = false
	g.vm.Call("on_load")
	fmt.Println("[hot-reload] main.lgs reloaded")
}

// callHook invokes an OPTIONAL script hook; absence of the function is
// normal (e.g. a pure-ECS script has no on_draw), real errors still log.
func (g *Game) callHook(fn string, args ...interface{}) {
	if _, err := g.vm.Call(fn, args...); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not found: "+fn) {
			return
		}
		fmt.Println(fn, "error:", msg)
	}
}

func (g *Game) Update() error {
	g.pollInput()
	g.checkHotReload()
	callScript(g.vm, "on_update", 1.0/float64(ebiten.TPS()))
	g.simulate() // declarative layer: timers, motion, steering, collisions
	if g.quitRequested {
		return ebiten.Termination // clean shutdown from script's quit()
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.cmds = g.cmds[:0] // reset queue; hooks refill it via draw_* builtins

	if g.world.active {
		// ECS mode: background hook, auto-drawn entities, HUD hook.
		g.callHook("on_draw_back")
		g.drawEntities()
		g.callHook("on_draw_front")
	} else {
		// legacy imperative mode: the script draws everything itself.
		callScript(g.vm, "on_draw")
	}

	for _, c := range g.cmds {
		// world-space: every command is drawn relative to ITS camera snapshot,
		// so scripts can switch cameras mid-draw (e.g. HUD after world)
		wx := c.x - c.camX
		wy := c.y - c.camY

		// sprites carry no color; default to white rather than parsing ""
		clr := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		if len(c.color) >= 6 {
			clr = hexToRGBA(c.color)
		}
		switch c.kind {
		case "rect":
			vector.DrawFilledRect(screen, float32(wx), float32(wy), float32(c.w), float32(c.h), clr, false)
		case "circle":
			vector.DrawFilledCircle(screen, float32(wx), float32(wy), float32(c.radius), clr, false)
		case "line":
			vector.StrokeLine(screen, float32(wx), float32(wy), float32(c.x2-c.camX), float32(c.y2-c.camY), float32(c.thickness), clr, false)
		case "text":
			if g.face == nil {
				continue
			}
			text.Draw(screen, c.str, g.face, int(wx), int(wy), clr)
		case "sprite":
			img := g.sprite(c.path)
			if img == nil {
				continue
			}
			opts := &ebiten.DrawImageOptions{}
			opts.GeoM.Translate(wx, wy)
			screen.DrawImage(img, opts)
		case "sprite_ex":
			img := g.sprite(c.path)
			if img == nil {
				continue
			}
			w := float64(img.Bounds().Dx()) * c.scale
			h := float64(img.Bounds().Dy()) * c.scale
			opts := &ebiten.DrawImageOptions{}
			// rotate around the sprite's center; (x, y) is the center point
			opts.GeoM.Translate(-w/2, -h/2)
			opts.GeoM.Scale(c.scale, c.scale)
			opts.GeoM.Rotate(c.rotation * math.Pi / 180) // degrees -> radians
			opts.GeoM.Translate(wx, wy)
			screen.DrawImage(img, opts)
		case "sprite_frame":
			img := g.sprite(c.path)
			if img == nil {
				continue
			}
			fw := int(c.w) // frame width; sheet is a horizontal strip
			fh := int(c.h)
			idx := int(c.radius) // reuse field as frame index
			cols := img.Bounds().Dx() / fw
			if cols < 1 {
				cols = 1
			}
			col := ((idx % cols) + cols) % cols // wraps negative indexes too
			sub := img.SubImage(image.Rect(col*fw, 0, col*fw+fw, fh)).(*ebiten.Image)
			opts := &ebiten.DrawImageOptions{}
			opts.GeoM.Translate(wx, wy)
			screen.DrawImage(sub, opts)
		}
	}
}

func (g *Game) Layout(w, h int) (int, int) {
	return gameW, gameH
}
