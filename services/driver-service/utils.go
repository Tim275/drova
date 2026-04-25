package main

import (
	math "math/rand/v2"
	"fmt"
)

var PredefinedRoutes = [][][2]float64{
	{{52.5200, 13.4050}, {52.5300, 13.4150}},
	{{52.5170, 13.3880}, {52.5080, 13.3760}},
	{{52.5250, 13.4200}, {52.5350, 13.4300}},
	{{52.5100, 13.3950}, {52.5000, 13.3850}},
	{{52.5320, 13.3800}, {52.5420, 13.3900}},
	{{52.5060, 13.4100}, {52.4960, 13.4000}},
	{{52.5280, 13.4350}, {52.5380, 13.4450}},
	{{52.5130, 13.3700}, {52.5230, 13.3800}},
}

var letters = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

func GenerateRandomPlate() string {
	city := string([]rune{letters[math.IntN(len(letters))], letters[math.IntN(len(letters))]})
	letters2 := string([]rune{letters[math.IntN(len(letters))], letters[math.IntN(len(letters))]})
	numbers := math.IntN(9000) + 1000
	return fmt.Sprintf("%s-%s %d", city, letters2, numbers)
}
