package commandrunner

import "strings"

// OSFamily represents a broad operating system family for VMs.
type OSFamily string

const (
	OSFamilyUnknown OSFamily = "unknown"
	OSFamilyLinux   OSFamily = "linux"
	OSFamilyWindows OSFamily = "windows"
)

// detectOSFamily maps a Proxmox ostype string to a coarse OS family.
func detectOSFamily(ostype string) OSFamily {
	if ostype == "" {
		return OSFamilyUnknown
	}

	lower := strings.ToLower(ostype)

	switch {
	case strings.HasPrefix(lower, "win"):
		return OSFamilyWindows
	case strings.HasPrefix(lower, "l"):
		return OSFamilyLinux
	default:
		return OSFamilyUnknown
	}
}
