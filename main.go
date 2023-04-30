package main

import (
	"fmt"
	"time"
	"math/rand"
	"embed"

	"github.com/jakecoffman/cp"

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
	fmt.Println("Running")
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

	atlas, err := glitch.DefaultAtlas()
	if err != nil { panic(err) }

	healthText := atlas.Text("Health: 10")

	shader, err := glitch.NewShader(shaders.SpriteShader)
	if err != nil { panic(err) }
	pass := glitch.NewRenderPass(shader)

	camera := glitch.NewCameraOrtho()

	levelBounds := glitch.R(0, 0, 900, 700).CenterAt(glitch.Vec2{}).Moved(glitch.Vec2{0, -100})

	game := NewGame(win, levelBounds, spritesheet)

	game.Reset()

	packingLine, err := spritesheet.Get("packing-line-0.png")
	if err != nil { panic(err) }

	for !win.Closed() {
		camera.SetOrtho2D(win.Bounds())
		camPos := win.Bounds().Center()
		camera.SetView2D(-camPos[0], -camPos[1], 1.0, 1.0)

		if win.Pressed(glitch.KeyEscape) {
			win.Close()
		}

		mouseX, mouseY := win.MousePosition()
		game.mousePos = camera.Unproject(glitch.Vec3{mouseX, mouseY, 0})

		// Limit mouse pos within the level bounds
		// heldPkgWidth := 100.0
		// if game.heldShape != nil {
		// 	heldSprite := game.heldShape.Body().UserData.(Sprite)
		// 	heldPkgWidth = heldSprite.sprite.Bounds().W()
		// }
		// if game.mousePos[0] < game.levelBounds.Min[0] + (heldPkgWidth/2) {
		// 	game.mousePos[0] = game.levelBounds.Min[0] + (heldPkgWidth/2)
		// } else if game.mousePos[0] > game.levelBounds.Max[0] - (heldPkgWidth/2) {
		// 	game.mousePos[0] = game.levelBounds.Max[0] - (heldPkgWidth/2)
		// }
		if game.mousePos[0] < game.activeBounds.Min[0] {
			game.mousePos[0] = game.activeBounds.Min[0]
		} else if game.mousePos[0] > game.activeBounds.Max[0] {
			game.mousePos[0] = game.activeBounds.Max[0]
		}

		if game.heldShape != nil {
			game.heldShape.Body().SetPosition(cp.Vector{game.mousePos[0], game.dropHeight})

			if win.Pressed(glitch.MouseButtonLeft) {
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
				fmt.Println("Idle", body.IdleTime())
				// Don't search if something is still active
				if body.IdleTime() < 0.1 {
					stillActive = true
				}
			})
			if !stillActive {
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
				game.Reset()
			}
		}

		pass.Clear()

		packingLine.RectDraw(pass, game.acceptBounds)

		game.space.EachBody(func(body *cp.Body) {
			DrawBody(pass, body)
		})

		game.DrawNextPackages(pass, 8)

		{
			healthText.Set(fmt.Sprintf("Health: %d", game.health))
			mat := glitch.Mat4Ident
			mat.Translate(-win.Bounds().W()/2, -win.Bounds().H()/2, 0)
			healthText.Draw(pass, mat)
		}

		glitch.Clear(win, glitch.Black)

		pass.SetUniform("projection", camera.Projection)
		pass.SetUniform("view", camera.View)
		pass.Draw(win)

		win.Update()
	}
}

type Game struct {
	win *glitch.Window
	spritesheet *asset.Spritesheet
	space *cp.Space

	mousePos glitch.Vec3

	dropHeight float64
	levelBounds, activeBounds, pegBounds, acceptBounds glitch.Rect

	health int

	heldShape *cp.Shape
	packages []string

	lastDropTime time.Time
	allPegs []phy2.Pos
}

func NewGame(win *glitch.Window, levelBounds glitch.Rect, spritesheet *asset.Spritesheet) *Game {
	game := &Game{
		win: win,
		spritesheet: spritesheet,
		health: 10,

		levelBounds: levelBounds,
	}

	return game
}

func (g *Game) Reset() {
	g.space = cp.NewSpace()
	g.space.Iterations = 4
	// g.space.IdleSpeedThreshold = 0.1
	g.space.SleepTimeThreshold = 1

	g.space.UseSpatialHash(2.0, 10000)
	g.space.SetGravity(cp.Vector{0, Gravity})

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
			sprite, err := g.spritesheet.Get("wall-0.png")
			if err != nil { panic(err) }

			s := NewSprite(sprite)
			s.scale = glitch.Vec2{wall.W() / sprite.Bounds().W(), wall.H() / sprite.Bounds().H()}

			shape := makeWall(s, wall)
			g.space.AddBody(shape.Body())
			g.space.AddShape(shape)
		}
	}

	g.packages = make([]string, 10) // TODO: Number of packages
	for i := range g.packages {
		packageNum := rand.Intn(6) // TODO: number of package sprites
		g.packages[i] = fmt.Sprintf("package-%d.png", packageNum)
	}

	g.activeBounds = g.levelBounds.Unpad(glitch.R(100, 0, 100, 0))
	g.pegBounds = g.activeBounds.Unpad(glitch.R(0, g.levelBounds.H()/2, 0, 100))
	g.acceptBounds = g.levelBounds.Unpad(glitch.R(0, 0, 0, 100 + g.levelBounds.H()/2))

	g.heldShape = g.GetNextPackage()

	// TODO: NumPegs
	g.allPegs = make([]phy2.Pos, 0)
	for i := 0; i < 10; i++ {
		g.AddPeg()
	}

	g.dropHeight = g.levelBounds.Max[1] + 200
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
		fmt.Println("You Win")
		return nil
	}
	pkg := g.packages[0]
	g.packages = g.packages[1:]
	fmt.Println("NextPackage: ", pkg)
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

	mat := glitch.Mat4Ident
	mat.Scale(sprite.scale[0], sprite.scale[1], 1.0)
	mat.Rotate(angle, glitch.Vec3{0, 0, 1})
	mat.Translate(pos.X, pos.Y, 0)
	sprite.sprite.Draw(pass, mat)
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
	scale glitch.Vec2
	isPackage bool
}
func NewSprite(s *glitch.Sprite) Sprite {
	return Sprite{
		sprite: s,
		scale: glitch.Vec2{1, 1},
	}
}
