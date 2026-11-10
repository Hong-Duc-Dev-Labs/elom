package svc

import (
	"os"

	"elom/internal/core/startup"
)

// Startup lists and toggles what starts with Windows.
type Startup struct{}

// List returns the startup entries of this machine.
func (Startup) List() ([]startup.Entry, error) { return startup.Enumerate() }

// SetEnabled writes the enabled flag for one entry.
func (Startup) SetEnabled(entry startup.Entry, on bool) error {
	return startup.SetEnabled(entry, on)
}

// userHome is the profile directory, or the working directory when the
// variable is missing.
func userHome() string {
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}
