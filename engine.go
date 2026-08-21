package main

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/codetesla51/logos/logos"
	"github.com/hajimehoshi/ebiten/v2"
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
	str, path         string
	color             string
}

type Game struct {
	vm                          *logos.VM
	cmds                        []drawCmd
	sprites                     map[string]*ebiten.Image
	failed                      map[string]bool // sprite paths that failed to load (warn once)
	face                        font.Face
	keysCurr, keysPrev          map[ebiten.Key]bool
	mouseCurr, mousePrev        map[ebiten.MouseButton]bool
	quitRequested               bool
}

func newGame(vm *logos.VM) *Game {
	g := &Game{
		vm:        vm,
		sprites:   map[string]*ebiten.Image{},
		failed:    map[string]bool{},
		face:      loadFont(),
		keysCurr:  map[ebiten.Key]bool{},
		keysPrev:  map[ebiten.Key]bool{},
		mouseCurr: map[ebiten.MouseButton]bool{},
		mousePrev: map[ebiten.MouseButton]bool{},
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
	candidates := []string{
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

func (g *Game) Update() error {
	g.pollInput()
	callScript(g.vm, "on_update", 1.0/float64(ebiten.TPS()))
	if g.quitRequested {
		return ebiten.Termination // clean shutdown from script's quit()
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.cmds = g.cmds[:0] // reset queue; on_draw refills it via draw_* builtins
	g.vm.Call("on_draw")

	for _, c := range g.cmds {
		// sprites carry no color; default to white rather than parsing ""
		clr := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		if len(c.color) >= 6 {
			clr = hexToRGBA(c.color)
		}
		switch c.kind {
		case "rect":
			vector.DrawFilledRect(screen, float32(c.x), float32(c.y), float32(c.w), float32(c.h), clr, false)
		case "circle":
			vector.DrawFilledCircle(screen, float32(c.x), float32(c.y), float32(c.radius), clr, false)
		case "line":
			vector.StrokeLine(screen, float32(c.x), float32(c.y), float32(c.x2), float32(c.y2), float32(c.thickness), clr, false)
		case "text":
			if g.face == nil {
				continue
			}
			text.Draw(screen, c.str, g.face, int(c.x), int(c.y), clr)
		case "sprite":
			img := g.sprite(c.path)
			if img == nil {
				continue
			}
			opts := &ebiten.DrawImageOptions{}
			opts.GeoM.Translate(c.x, c.y)
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
			opts.GeoM.Translate(c.x, c.y)
			screen.DrawImage(img, opts)
		}
	}
}

func (g *Game) Layout(w, h int) (int, int) {
	return gameW, gameH
}
