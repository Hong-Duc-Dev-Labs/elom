//go:build windows

package apps

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// uninstallRoots are the registry locations Settings › Apps reads, with their
// 64- and 32-bit views.
type uninstallRoot struct {
	label string
	root  registry.Key
	path  string
	flags uint32
}

var uninstallRoots = []uninstallRoot{
	{"HKLM 64", registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_64KEY},
	{"HKLM 32", registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_32KEY},
	{"HKCU 64", registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_64KEY},
	{"HKCU 32", registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_32KEY},
}

// Enumerate lists installed programs from the registry. Packaged apps are added
// by the caller with EnumeratePackaged, which needs PowerShell.
func Enumerate() ([]App, error) {
	seen := map[string]bool{}
	var out []App
	opened := 0
	for _, root := range uninstallRoots {
		key, err := registry.OpenKey(root.root, root.path, registry.READ|root.flags)
		if err != nil {
			continue // a hive or view that does not exist on this account
		}
		opened++
		names, err := key.ReadSubKeyNames(-1)
		if err != nil {
			key.Close()
			continue
		}
		for _, name := range names {
			sub, err := registry.OpenKey(key, name, registry.READ|root.flags)
			if err != nil {
				continue
			}
			vals, ints := readValues(sub)
			sub.Close()
			app, ok := ParseUninstall(root.label+`\`+name, vals, ints)
			if !ok {
				continue
			}
			keyID := app.Name + "\x00" + app.Version
			if seen[keyID] {
				continue
			}
			seen[keyID] = true
			out = append(out, app)
		}
		key.Close()
	}
	if opened == 0 {
		// Every location failed: an empty list would read as "no apps
		// installed", which is a different claim.
		return nil, fmt.Errorf("apps: none of the %d uninstall registry locations could be read", len(uninstallRoots))
	}
	SortApps(out)
	return out, nil
}

// readValues reads the string and integer values of one uninstall key.
func readValues(key registry.Key) (map[string]string, map[string]uint64) {
	vals := map[string]string{}
	ints := map[string]uint64{}
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return vals, ints
	}
	for _, name := range names {
		if text, _, err := key.GetStringValue(name); err == nil {
			vals[name] = text
			continue
		}
		if n, _, err := key.GetIntegerValue(name); err == nil {
			ints[name] = n
		}
	}
	return vals, ints
}
