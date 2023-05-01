package main

// TODO:
// - [x] Background Music 'M' To Mute
// - [x] Jomy package
// - [x] Draw a game background
// - [x] Show highest level achieved
// - [x] Make a main menu background
// - [x] performance testing and wasm
// - [x] make pegs stand out better
// - [ ] Submit???

import (
	"fmt"
	"time"
	"math"
	"math/rand"
	"embed"
	"unicode"

	"github.com/jakecoffman/cp"

	"github.com/hajimehoshi/go-mp3"

	"github.com/unitoftime/flow/phy2"
	"github.com/unitoftime/flow/asset"

	"github.com/unitoftime/glitch"
	"github.com/unitoftime/glitch/shaders"
)

//go:embed assets/*
var EmbeddedFilesystem embed.FS

const (
	Gravity = -9.81
)

func main() {
	glitch.Run(run)
}

func run() {
	win, err := glitch.NewWindow(1920, 1080, "Boxlin", glitch.WindowConfig{
		Vsync: true,
		Fullscreen: false,
		// Samples: 4,
	})
	if err != nil {
		panic(err)
	}

	load := asset.NewLoad(EmbeddedFilesystem)
	spritesheet, err := load.Spritesheet("assets/spritesheet.json", false)
	if err != nil {
		panic(err)
	}

	font, err := load.Font("assets/ThaleahFat.ttf", 64)
	if err != nil {
		panic(err)
	}
	runes := make([]rune, unicode.MaxASCII - 32)
	for i := range runes {
		runes[i] = rune(32 + i)
	}
	atlas := glitch.NewAtlas(font, runes, true, 0)

	healthText := atlas.Text(" Health: 10")
	menuText := atlas.Text(" Press Space To Play!")
	muteText := atlas.Text(" Press M To Mute")
	recordText := atlas.Text("High Score: 0")

	shader, err := glitch.NewShader(shaders.SpriteShader)
	if err != nil { panic(err) }
	pass := glitch.NewRenderPass(shader)

	camera := glitch.NewCameraOrtho()

	levelBounds := glitch.R(0, 0, 900, 700).CenterAt(glitch.Vec2{}).Moved(glitch.Vec2{0, -100})

	game := NewGame(win, levelBounds, spritesheet)

	game.mode = "menu"

	go func() {
		game.player = NewAudioPlayer()
		bgMusic := LoadMp3(load, "assets/bg.mp3")
		game.player.Play(bgMusic)
	}()
	// game.hitSound = LoadMp3(load, "assets/hit.mp3")
	// game.player.Play(game.hitSound)

	game.ResetLevel()

	packingLine, err := spritesheet.Get("background-0.png")
	// packingLine, err := spritesheet.Get("packing-line-0.png")
	// border := 0.0
	// packingLine, err := spritesheet.GetNinePanel("packing-line-0.png", glitch.R(border, border, border, border))
	if err != nil { panic(err) }

	for !win.Closed() {
		camera.SetOrtho2D(win.Bounds())
		camPos := win.Bounds().Center()
		camera.SetView2D(-camPos[0], -camPos[1], 1.0, 1.0)

		mouseX, mouseY := win.MousePosition()
		game.mousePos = camera.Unproject(glitch.Vec3{mouseX, mouseY, 0})

		if win.JustPressed(glitch.KeyM) {
			game.player.TogglePlayPause()
		}

		if game.mode == "menu" {
			if win.JustPressed(glitch.KeySpace) {
				game.ResetGame()
				game.mode = "game"
			}
			// if win.JustPressed(glitch.KeyEscape) {
			// 	win.Close()
			// }
		} else if game.mode == "game" {
			if win.JustPressed(glitch.KeyEscape) {
				game.mode = "menu"
			}

			// Limit mouse pos within the level bounds
			if game.mousePos[0] < game.activeBounds.Min[0] {
				game.mousePos[0] = game.activeBounds.Min[0]
			} else if game.mousePos[0] > game.activeBounds.Max[0] {
				game.mousePos[0] = game.activeBounds.Max[0]
			}

			if game.heldShape != nil {
				game.heldShape.Body().SetPosition(cp.Vector{game.mousePos[0], game.dropHeight})
				game.heldShape.Body().SetVelocity(0, 0)
				game.heldShape.Body().SetAngularVelocity(0)
				game.heldShape.Body().SetAngle(0)

				if win.JustPressed(glitch.MouseButtonLeft) {
					game.heldShape.Body().SetVelocity(0, -20)
					game.heldShape = nil
					game.lastDropTime = time.Now()
				}
			}

			game.space.Step(128 * time.Millisecond.Seconds())

			if game.heldShape == nil {
				if time.Since(game.lastDropTime) > 100 * time.Millisecond {
					game.heldShape = game.GetNextPackage()
				}
			}

			if len(game.packages) <= 0 && game.heldShape == nil {
				stillActive := false
				game.space.EachBody(func(body *cp.Body) {
					// fmt.Println("Idle", body.IdleTime())
					// Don't search if something is still active
					if body.IdleTime() < 0.1 {
						stillActive = true
					}
				})
				if !stillActive {
					game.idleCounter++
				}

				// Force timeout if it never stabilizes
				timeoutEndLevel := false
				if time.Since(game.lastDropTime) > 10 * time.Second {
					timeoutEndLevel = true
				}

				if game.idleCounter > 100 || timeoutEndLevel {
					// Check end conditions
					areaBB := cp.BB{
						L: game.acceptBounds.Min[0],
						B: game.acceptBounds.Min[1],
						R: game.acceptBounds.Max[0],
						T: game.acceptBounds.Max[1],
					}

					healthLost := 0
					game.space.EachShape(func(shape *cp.Shape) {
						sprite := shape.Body().UserData.(Sprite)
						if !sprite.isPackage { return } // Skip if not a package

						if !areaBB.Contains(shape.BB()) {
							healthLost++
						}
					})

					game.health -= healthLost
					if game.health <= 0 {
						if game.difficulty > game.record {
							game.record = game.difficulty
						}
						game.mode = "menu"
					}

					game.difficulty++

					game.ResetLevel()
				}
			}
		}

		pass.Clear()

		if game.mode == "menu" {
			rect := glitch.R(-300, 0, 300, 100)
			menuText.DrawRect(pass, rect, glitch.White)
			muteText.DrawRect(pass,
				glitch.R(-win.Bounds().W()/2, -win.Bounds().H()/2, -win.Bounds().W()/2 + 300, win.Bounds().H()/2 + 300),
				glitch.White)

			{
				theta := float64(time.Now().UnixMilli()) / 1000
				textOscillation := 5 * math.Sin(7 * theta)
				recordText.Set(fmt.Sprintf("High Score: %d", game.record))
				recordText.DrawRect(pass,
					rect.Moved(glitch.Vec2{130, -200 + textOscillation}),
					glitch.FromUint8(0xfa, 0xcb, 0x3e, 0xff))
			}
		} else if game.mode == "game" {
			packingLine.RectDraw(pass, game.levelBounds)
			// {
			// 	mat := glitch.Mat4Ident
			// 	mat.Scale(4, 4, 1)
			// 	packingLine.Draw(pass, mat)
			// }

			game.space.EachBody(func(body *cp.Body) {
				DrawBody(pass, body)
			})

			game.DrawNextPackages(pass, 8)

			{
				healthText.Set(fmt.Sprintf(" Health: %d", game.health))
				mat := glitch.Mat4Ident
				mat.Translate(-win.Bounds().W()/2, -win.Bounds().H()/2, 0)
				healthText.Draw(pass, mat)
			}
		}

		// glitch.Clear(win, glitch.Black)
		// glitch.Clear(win, glitch.FromUint8(0x48, 0x3b, 0x3a, 0xff))
		glitch.Clear(win, glitch.FromUint8(0x6b, 0x6b, 0x6b, 0xff))

		pass.SetUniform("projection", camera.Projection)
		pass.SetUniform("view", camera.View)
		pass.Draw(win)

		win.Update()
	}
}

type Game struct {
	mode string
	record int

	win *glitch.Window
	spritesheet *asset.Spritesheet
	space *cp.Space
	difficulty int

	mousePos glitch.Vec3

	dropHeight float64
	levelBounds, activeBounds, pegBounds, acceptBounds glitch.Rect

	health int
	idleCounter int

	heldShape *cp.Shape
	packages []string

	lastDropTime time.Time
	allPegs []phy2.Pos

	// Audio
	player *AudioPlayer
	hitSound *mp3.Decoder
}

func NewGame(win *glitch.Window, levelBounds glitch.Rect, spritesheet *asset.Spritesheet) *Game {
	game := &Game{
		win: win,
		spritesheet: spritesheet,
		health: 10,
		difficulty: 0,

		levelBounds: levelBounds,
	}

	return game
}

func (g *Game) ResetGame() {
	g.health = 10
	g.difficulty = 0
	g.ResetLevel()
}

func (g *Game) ResetLevel() {
	g.space = cp.NewSpace()
	g.space.Iterations = 8
	// g.space.IdleSpeedThreshold = 0.1
	g.space.SleepTimeThreshold = 1

	// g.space.UseSpatialHash(2.0, 10)
	g.space.SetGravity(cp.Vector{0, Gravity})

	// handler := g.space.NewCollisionHandler(cp.WILDCARD_COLLISION_TYPE, cp.WILDCARD_COLLISION_TYPE)
	// handler.BeginFunc = func(arb *cp.Arbiter, space *cp.Space, userData interface{}) bool {
	// 	fmt.Println("Collision")
	// 	// g.player.Play(g.hitSound)
	// 	return true
	// }
	// handler.SeparateFunc = func(arb *cp.Arbiter, space *cp.Space, userData interface{}) {
	// 	// g.player.Play(g.hitSound)
	// 	fmt.Println("Sep")
	// }

	// Walls
	{
		// fmt.Println(levelBounds)
		thickness := 25.0
		walls := []glitch.Rect{
			glitch.R(g.levelBounds.Min[0], g.levelBounds.Min[1],
				g.levelBounds.Max[0], g.levelBounds.Min[1] + thickness),
			glitch.R(g.levelBounds.Min[0], g.levelBounds.Min[1],
				g.levelBounds.Min[0] + thickness, g.levelBounds.Max[1]),
			glitch.R(g.levelBounds.Max[0] - thickness, g.levelBounds.Min[1],
				g.levelBounds.Max[0], g.levelBounds.Max[1]),
		}

		for _, wall := range walls {
			ninePanel, err := g.spritesheet.GetNinePanel("wall-0.png", glitch.R(8, 8, 8, 8))
			if err != nil { panic(err) }

			s := NewSprite(nil)
			s.ninePanel = ninePanel
			s.rect = glitch.R(-wall.W()/2, -wall.H()/2, wall.W()/2, wall.H()/2)
			// s.scale = glitch.Vec2{wall.W() / s.ninePanel.Bounds().W(), wall.H() / s.ninePanel.Bounds().H()}

			shape := makeWall(s, wall)
			g.space.AddBody(shape.Body())
			g.space.AddShape(shape)
		}
	}

	packageTable := NewRngTable(
		NewRngItem(20, "package-0.png"),
		NewRngItem(20, "package-1.png"),
		NewRngItem(20, "package-2.png"),
		NewRngItem(20, "package-3.png"),
		NewRngItem(20, "package-4.png"),
		NewRngItem(20, "package-5.png"),
		NewRngItem(20, "package-6.png"),
		NewRngItem(10, "package-7.png"),
		NewRngItem(1, "package-8.png"),
		NewRngItem(1, "package-9.png"),
		NewRngItem(1, "package-10.png"),
		NewRngItem(1, "package-11.png"),
	)

	// numSprites := 11
	g.packages = make([]string, 10 + g.difficulty)
	for i := range g.packages {
		pkgName := packageTable.Roll()
		g.packages[i] = pkgName
	}

	g.activeBounds = g.levelBounds.Unpad(glitch.R(100, 0, 100, 0))
	g.pegBounds = g.activeBounds.Unpad(glitch.R(0, g.levelBounds.H()/2, 0, 100))
	g.acceptBounds = g.levelBounds.Unpad(glitch.R(0, 0, 0, 100 + g.levelBounds.H()/2))

	g.heldShape = g.GetNextPackage()

	numPegs := 10 + g.difficulty
	g.allPegs = make([]phy2.Pos, 0)
	for i := 0; i < numPegs; i++ {
		g.AddPeg()
	}

	g.dropHeight = g.levelBounds.Max[1] + 200
	g.idleCounter = 0
}

func (g *Game) AddPeg() {
	minDistance := 8 * 16.0

	attempts := 0

	var x, y float64
	for {
		attempts++
		if attempts > 10 {
			return
		}

		tooClose := false
		x = (rand.Float64() * g.pegBounds.W()) + g.pegBounds.Min[0]
		y = (rand.Float64() * g.pegBounds.H()) + g.pegBounds.Min[1]

		for i := range g.allPegs {
			if g.allPegs[i].Sub(phy2.Pos{x, y}).Len() < minDistance {
				tooClose = true
			}
		}

		if tooClose {
			continue
		} else {
			break
		}
	}

	g.allPegs = append(g.allPegs, phy2.Pos{x, y})

	sprite, err := g.spritesheet.Get("peg-0.png")
	if err != nil { panic(err) }
	s := NewSprite(sprite)

	shape := makePeg(s, x, y)
	g.space.AddBody(shape.Body())
	g.space.AddShape(shape)
}

func (g *Game) GetNextPackage() *cp.Shape {
	if len(g.packages) <= 0 {
		return nil
	}

	pkg := g.packages[0]
	g.packages = g.packages[1:]
	// fmt.Println("NextPackage: ", pkg)
	sprite, err := g.spritesheet.Get(pkg)
	if err != nil { panic(err) }
	s := NewSprite(sprite)

	shape := makePackage(s, 0, 250)
	shape.Body().SetPosition(cp.Vector{g.mousePos[0], g.dropHeight})

	g.space.AddBody(shape.Body())
	g.space.AddShape(shape)

	return shape
}

func (g *Game) DrawNextPackages(pass *glitch.RenderPass, num int) {
	screenHeight := g.win.Bounds().H()

	packageOffset :=  (3.0/4.0) * screenHeight / float64(num)

	startY := g.dropHeight - 100
	startX := g.levelBounds.Min[0] - 300

	for i := 0; i < num; i++ {
		if i >= len(g.packages) { break }

		sprite, err := g.spritesheet.Get(g.packages[i])
		if err != nil { panic(err) }


		mat := glitch.Mat4Ident
		mat.Translate(startX, startY, 0)
		sprite.Draw(pass, mat)

		startY -= packageOffset
	}
}

func DrawBody(pass *glitch.RenderPass, body *cp.Body) {
	geom := glitch.NewGeomDraw()
	geom.SetColor(glitch.White)

	sprite := body.UserData.(Sprite)

	pos := body.Position()
	angle := body.Angle()


	if sprite.sprite != nil {
		mat := glitch.Mat4Ident
		mat.Scale(sprite.scale[0], sprite.scale[1], 1.0)
		mat.Rotate(angle, glitch.Vec3{0, 0, 1})
		mat.Translate(pos.X, pos.Y, 0)
		sprite.sprite.Draw(pass, mat)
	} else if sprite.ninePanel != nil {
		sprite.ninePanel.RectDraw(pass, sprite.rect.Moved(glitch.Vec2{pos.X, pos.Y}))
	}
}

func makePackage(sprite Sprite, x, y float64) *cp.Shape {
	body := cp.NewBody(10, 1.0)
	body.SetPosition(cp.Vector{X: x, Y: y})

	width := sprite.sprite.Bounds().W()
	height := sprite.sprite.Bounds().H()

	sprite.isPackage = true
	body.UserData = sprite

	// shape := cp.NewCircle(body, 8, cp.Vector{})
	shape := cp.NewBox(body, width, height, 0)
	shape.SetCollisionType(cp.WILDCARD_COLLISION_TYPE)
	shape.SetElasticity(0.5)
	shape.SetDensity(1)
	shape.SetFriction(0.5)

	return shape
}

func makePeg(sprite Sprite, x, y float64) *cp.Shape {
	// body = space.AddBody(cp.NewBody(1e9, cp.INFINITY))
	body := cp.NewStaticBody()
	body.SetPosition(cp.Vector{x, y})

	radius := sprite.sprite.Bounds().W()/2

	body.UserData = sprite

	shape := cp.NewCircle(body, radius, cp.Vector{})
	shape.SetCollisionType(cp.WILDCARD_COLLISION_TYPE)
	shape.SetElasticity(0.5)
	shape.SetDensity(1)
	shape.SetFriction(0.2)


	return shape
}

func makeWall(sprite Sprite, rect glitch.Rect) *cp.Shape {
	body := cp.NewStaticBody()
	center := rect.Center()
	body.SetPosition(cp.Vector{center[0], center[1]})

	width := rect.W()
	height := rect.H()

	body.UserData = sprite

	shape := cp.NewBox(body, width, height, 0)
	shape.SetElasticity(0)
	shape.SetDensity(1)
	shape.SetFriction(0.5)

	return shape
}

type Sprite struct {
	// mesh *glitch.Mesh
	sprite *glitch.Sprite
	ninePanel *glitch.NinePanelSprite
	rect glitch.Rect
	scale glitch.Vec2
	isPackage bool
}
func NewSprite(s *glitch.Sprite) Sprite {
	return Sprite{
		sprite: s,
		scale: glitch.Vec2{1, 1},
	}
}
