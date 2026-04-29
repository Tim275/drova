package util

import "fmt"

var driverSeeds = []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}

func GetRandomAvatar(seed int) string {
	name := driverSeeds[seed%len(driverSeeds)]
	return fmt.Sprintf("https://api.dicebear.com/9.x/bottts/svg?seed=%s&backgroundColor=b6e3f4", name)
}
