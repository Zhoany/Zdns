package utils

import (
	"os/exec"
)

// CheckIPInSet returns true if the IP is contained in the given ipset set.
func CheckIPInSet(ip, set string) bool {
	if set == "" {
		return false
	}
	cmd := exec.Command("ipset", "test", set, ip)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
