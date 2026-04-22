package util

import "fmt"

func GetRandomAvatar(seed int) string {
	return fmt.Sprintf("https://randomuser.me/api/portraits/lego/%d.jpg", seed)
}
