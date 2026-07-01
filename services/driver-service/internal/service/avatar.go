package service

import "fmt"

// GetRandomAvatar was shared/util — only driver-service used it, so it now lives
// here in the driver service package.
var avatarSeeds = []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}

func GetRandomAvatar(seed int) string {
	name := avatarSeeds[seed%len(avatarSeeds)]
	return fmt.Sprintf("https://api.dicebear.com/9.x/bottts/svg?seed=%s&backgroundColor=b6e3f4", name)
}
