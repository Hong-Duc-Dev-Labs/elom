package metrics

import (
	"regexp"
	"strings"
	"time"
)

// Battery is the machine's battery, for machines that have one.
type Battery struct {
	Percent int
	// Plugged is true while the charger is connected.
	Plugged bool
	// Remaining is Windows' estimate of the time left on battery, zero when
	// it has none (on the charger, or still estimating).
	Remaining time.Duration
}

// System is what does not change while the app runs: read once, shown as the
// chips on the health card.
type System struct {
	CPUModel string // short, as "i7-1360P" or "Ryzen 7 5800X"
	Cores    int    // logical processors
	OS       string // as "Windows 11 25H2"
}

var (
	intelCore = regexp.MustCompile(`\b(i[3579]-\w+|Ultra \d+ \w+)`)
	ryzen     = regexp.MustCompile(`\bRyzen( (Threadripper|AI))? \d+( \w+)?`)
)

// shortCPU turns a processor's marketing name into the few characters a chip
// has room for. A name it does not recognize is returned cleaned of trademark
// marks and clock speed.
func shortCPU(name string) string {
	clean := name
	for _, mark := range []string{"(R)", "(TM)", "(tm)", "®", "™"} {
		clean = strings.ReplaceAll(clean, mark, "")
	}
	if at := strings.Index(clean, "@"); at >= 0 {
		clean = clean[:at]
	}
	clean = strings.Join(strings.Fields(clean), " ")
	if m := intelCore.FindString(clean); m != "" {
		return m
	}
	if m := ryzen.FindString(clean); m != "" {
		return m
	}
	clean = strings.TrimSuffix(clean, " CPU")
	clean = strings.TrimSuffix(clean, " Processor")
	return strings.TrimSpace(clean)
}
