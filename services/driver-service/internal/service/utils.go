package service

import (
	"fmt"
	math "math/rand/v2"
)

var PredefinedRoutes = [][][2]float64{
	// Berlin
	{{52.5200, 13.4050}, {52.5300, 13.4150}},
	{{52.5170, 13.3880}, {52.5080, 13.3760}},
	{{52.5250, 13.4200}, {52.5350, 13.4300}},
	{{52.5100, 13.3950}, {52.5000, 13.3850}},
	// Düsseldorf
	{{51.2217, 6.7762}, {51.2280, 6.7850}},
	{{51.2150, 6.7680}, {51.2100, 6.7600}},
	{{51.2350, 6.7900}, {51.2400, 6.8000}},
	{{51.2050, 6.7700}, {51.2000, 6.7620}},
	// Cologne
	{{50.9333, 6.9500}, {50.9400, 6.9600}},
	{{50.9280, 6.9420}, {50.9200, 6.9350}},
	// Frankfurt
	{{50.1109, 8.6821}, {50.1180, 8.6900}},
	{{50.1050, 8.6750}, {50.1000, 8.6680}},
	// Munich
	{{48.1351, 11.5820}, {48.1420, 11.5900}},
	{{48.1280, 11.5750}, {48.1200, 11.5680}},
	// Hamburg
	{{53.5753, 10.0153}, {53.5820, 10.0230}},
	{{53.5680, 10.0080}, {53.5610, 10.0010}},
}

var letters = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

func GenerateRandomPlate() string {
	city := string([]rune{letters[math.IntN(len(letters))], letters[math.IntN(len(letters))]})     // #nosec G404 -- display-only, not security-sensitive
	letters2 := string([]rune{letters[math.IntN(len(letters))], letters[math.IntN(len(letters))]}) // #nosec G404
	numbers := math.IntN(9000) + 1000                                                              // #nosec G404
	return fmt.Sprintf("%s-%s %d", city, letters2, numbers)
}
