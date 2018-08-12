package main

import (
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"time"

	"github.com/disintegration/gift"
)

var aliensPerRow = 8
var aliensStartCol = 100
var alienSize = 30
var bombProbability = 0.005
var bombSpeed = 10

// sprites
var sprites image.Image
var background image.Image
var cannonSprite = image.Rect(20, 47, 38, 59)
var cannonExplode = image.Rect(0, 47, 16, 57)
var alien1Sprite = image.Rect(0, 0, 20, 14)
var alien1aSprite = image.Rect(20, 0, 40, 14)
var alien2Sprite = image.Rect(0, 14, 20, 26)
var alien2aSprite = image.Rect(20, 14, 40, 26)
var alien3Sprite = image.Rect(0, 27, 20, 40)
var alien3aSprite = image.Rect(20, 27, 40, 40)
var alienExplode = image.Rect(0, 60, 16, 68)
var beamSprite = image.Rect(20, 60, 22, 65)
var bombSprite = image.Rect(0, 70, 10, 79)

// Sprite represents a sprite in the game
type Sprite struct {
	size     image.Rectangle // the sprite size
	Filter   *gift.GIFT      // normal filter used to draw the sprite
	FilterA  *gift.GIFT      // alternate filter used to draw the sprite
	FilterE  *gift.GIFT      // exploded filter used to draw the sprite
	Position image.Point     // top left position of the sprite
	Status   bool            // alive or dead
	Points   int             // number of points if destroyed
}

// sprite for laser cannon
var laserCannon = Sprite{
	size:     cannonSprite,
	Filter:   gift.New(gift.Crop(cannonSprite)),
	FilterE:  gift.New(gift.Crop(cannonExplode)),
	Position: image.Pt(50, 250),
	Status:   true,
}

// sprite for the laser beam
var beam = Sprite{
	size:     beamSprite,
	Filter:   gift.New(gift.Crop(beamSprite)),
	Position: image.Pt(laserCannon.Position.X+7, 250),
	Status:   false,
}

// used for creating alien sprites
func createAlien(x, y int, sprite, alt image.Rectangle, points int) (s Sprite) {
	s = Sprite{
		size:     sprite,
		Filter:   gift.New(gift.Crop(sprite)),
		FilterA:  gift.New(gift.Crop(alt)),
		FilterE:  gift.New(gift.Crop(alienExplode)),
		Position: image.Pt(x, y),
		Status:   true,
		Points:   points,
	}
	return
}

// generate frames for the game
func generateFrames() {
	rand.Seed(time.Now().UTC().UnixNano())
	var aliens = []Sprite{}
	var bombs = []Sprite{}
	// game variables
	loop := 0         // game loop
	beamShot := false // the instance where the beam is shot

	alienDirection := 1 // direction where alien is heading
	score := 0          // number of points scored in the game so far

	// populate the aliens
	for i := aliensStartCol; i < aliensStartCol+(alienSize*aliensPerRow); i += alienSize {
		aliens = append(aliens, createAlien(i, 30, alien1Sprite, alien1aSprite, 30))
	}
	for i := aliensStartCol; i < aliensStartCol+(30*aliensPerRow); i += alienSize {
		aliens = append(aliens, createAlien(i, 55, alien2Sprite, alien2aSprite, 20))
	}
	for i := aliensStartCol; i < aliensStartCol+(30*aliensPerRow); i += alienSize {
		aliens = append(aliens, createAlien(i, 80, alien3Sprite, alien3aSprite, 10))
	}

	// main game loop
	for !gameOver {
		// to slow up or speed up the game
		time.Sleep(time.Millisecond * time.Duration(gameDelay))
		// if any of the keyboard events are captured
		select {
		case ev := <-events:
			// exit the game
			if ev == "81" { // q
				gameOver = true
			}
			if ev == "32" { // space bar
				if beam.Status == false {
					beamShot = true
				}
				playSound("shoot")
			}
			if ev == "39" { // right arrow key
				laserCannon.Position.X += 10
			}
			if ev == "37" { // left arrow key
				laserCannon.Position.X -= 10
			}
		default:
		}

		// create background
		dst := image.NewRGBA(image.Rect(0, 0, windowWidth, windowHeight))
		gift.New().Draw(dst, background)

		// process aliens
		for i := 0; i < len(aliens); i++ {
			aliens[i].Position.X = aliens[i].Position.X + 3*alienDirection
			if aliens[i].Status {
				// if alien is hit by a laser beam
				if collide(aliens[i], beam) {
					// draw the explosion
					aliens[i].FilterE.DrawAt(dst, sprites, image.Pt(aliens[i].Position.X, aliens[i].Position.Y), gift.OverOperator)
					// alien dies, player scores points
					aliens[i].Status = false
					score += aliens[i].Points
					playSound("invaderkilled")
					// reset the laser beam
					resetBeam()
				} else {
					// show alternating aliens
					if loop%2 == 0 {
						aliens[i].Filter.DrawAt(dst, sprites, image.Pt(aliens[i].Position.X, aliens[i].Position.Y), gift.OverOperator)
					} else {
						aliens[i].FilterA.DrawAt(dst, sprites, image.Pt(aliens[i].Position.X, aliens[i].Position.Y), gift.OverOperator)
					}
					// drop torpedoes
					if rand.Float64() < bombProbability {
						bombs = append(bombs, dropBomb(aliens[i]))
					}
				}
			}
		}

		// draw bombs, if laser cannon is hit, game over
		for i := 0; i < len(bombs); i++ {
			bombs[i].Position.Y = bombs[i].Position.Y + bombSpeed
			bombs[i].Filter.DrawAt(dst, sprites, image.Pt(bombs[i].Position.X, bombs[i].Position.Y), gift.OverOperator)
			if collide(bombs[i], laserCannon) {
				gameOver = true
				laserCannon.FilterE.DrawAt(dst, sprites, image.Pt(laserCannon.Position.X, laserCannon.Position.Y), gift.OverOperator)
			}
		}
		// draw the laser cannon unless it's been destroyed
		if !gameOver {
			laserCannon.Filter.DrawAt(dst, sprites, image.Pt(laserCannon.Position.X, laserCannon.Position.Y), gift.OverOperator)
		}

		// move the aliens back and forth
		if aliens[0].Position.X < alienSize || aliens[aliensPerRow-1].Position.X > windowWidth-(2*alienSize) {
			alienDirection = alienDirection * -1
			for i := 0; i < len(aliens); i++ {
				aliens[i].Position.Y = aliens[i].Position.Y + 10
			}
		}

		// if the beam is shot, place the beam at start of the cannon
		if beamShot {
			beam.Position.X = laserCannon.Position.X + 7
			beam.Status = true
			beamShot = false
		}

		// keep drawing the beam as it moves every loop
		if beam.Status {
			beam.Filter.DrawAt(dst, sprites, image.Pt(beam.Position.X, beam.Position.Y), gift.OverOperator)
			beam.Position.Y -= 10
		}

		// if the beam leaves the window reset it
		if beam.Position.Y < 0 {
			resetBeam()
		}

		// if the aliens reach the position of the cannon, it's game over!
		if aliens[0].Position.Y > 180 {
			gameOver = true
		}
		createFrame(dst)
		// pause a bit before ending the game
		if gameOver {
			playSound("explosion")
			time.Sleep(time.Second)
		}
		loop++
	}

	// show end screen and score
	endScreen := getImage(dir + "/public/images/gameover.png").(*image.RGBA)
	printLine(endScreen, 137, 220, fmt.Sprintf("Your score is %d", score), color.RGBA{255, 0, 0, 255})
	printLine(endScreen, 104, 240, "Press 's' to play again", color.RGBA{255, 0, 0, 255})
	printLine(endScreen, 137, 260, "Press 'q' to quit", color.RGBA{255, 0, 0, 255})

	createFrame(endScreen)
}

// alien drops the bomb
func dropBomb(alien Sprite) (torpedo Sprite) {
	torpedo = Sprite{
		size:     bombSprite,
		Filter:   gift.New(gift.Crop(bombSprite)),
		Position: image.Pt(alien.Position.X+7, alien.Position.Y),
		Status:   true,
	}
	return
}

// resets the beam once it goes out of the frame
func resetBeam() {
	beam.Status = false
	beam.Position.Y = 250
}

// checks if two sprites collide
func collide(s1, s2 Sprite) bool {
	spriteA := image.Rect(s1.Position.X, s1.Position.Y, s1.Position.X+s1.size.Dx(), s1.Position.Y+s1.size.Dy())
	spriteB := image.Rect(s2.Position.X, s2.Position.Y, s2.Position.X+s1.size.Dx(), s2.Position.Y+s1.size.Dy())
	if spriteA.Min.X < spriteB.Max.X && spriteA.Max.X > spriteB.Min.X &&
		spriteA.Min.Y < spriteB.Max.Y && spriteA.Max.Y > spriteB.Min.Y {
		return true
	}
	return false
}
