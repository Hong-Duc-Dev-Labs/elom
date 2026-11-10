//go:build windows

package startup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// ErrNeedsAdmin is returned when a machine-wide entry would be changed.
var ErrNeedsAdmin = errors.New("startup: a machine-wide entry needs administrator rights")

// runKeys are the registry locations Explorer starts programs from.
var runKeys = []struct {
	source string
	scope  string
	root   registry.Key
	path   string
	flags  uint32
}{
	{"HKCU Run", "user", registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_64KEY},
	{"HKCU RunOnce", "user", registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\RunOnce`, registry.WOW64_64KEY},
	{"HKLM Run", "machine", registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_64KEY},
	{"HKLM Run (32-bit)", "machine", registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_64KEY},
	{"HKLM RunOnce", "machine", registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce`, registry.WOW64_64KEY},
}

// approvedPath is where Explorer keeps the enabled flag of each item.
const approvedPath = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`

// Enumerate lists startup entries from Run keys and the Startup folders.
func Enumerate() ([]Entry, error) {
	var out []Entry
	for _, key := range runKeys {
		k, err := registry.OpenKey(key.root, key.path, registry.READ)
		if err != nil {
			continue
		}
		names, err := k.ReadValueNames(-1)
		if err != nil {
			k.Close()
			continue
		}
		approved := readApproved(key.root, key.scope)
		for _, name := range names {
			command, _, err := k.GetStringValue(name)
			if err != nil || strings.TrimSpace(command) == "" {
				continue
			}
			entry := Entry{
				Name:     name,
				Command:  strings.TrimSpace(command),
				Source:   key.source,
				Scope:    key.scope,
				Enabled:  true,
				Writable: true,
			}
			if blob, ok := approved[strings.ToLower(name)]; ok {
				entry.Enabled = ApprovedEnabled(blob)
			}
			out = append(out, entry)
		}
		k.Close()
	}
	out = append(out, folderEntries()...)
	return out, nil
}

// readApproved returns the StartupApproved blobs by lower-cased value name,
// for the user's and the machine's Run key.
func readApproved(root registry.Key, scope string) map[string][]byte {
	blobs := map[string][]byte{}
	key, err := registry.OpenKey(root, approvedPath, registry.READ)
	if err != nil {
		return blobs
	}
	defer key.Close()
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return blobs
	}
	for _, name := range names {
		if data, _, err := key.GetBinaryValue(name); err == nil {
			blobs[strings.ToLower(name)] = data
		} else if n, _, err := key.GetIntegerValue(name); err == nil {
			blobs[strings.ToLower(name)] = []byte{byte(n)}
		}
	}
	return blobs
}

// folderEntries lists shortcuts in the two Startup folders. Windows records no
// enabled flag for these, so Elom shows them without a switch.
func folderEntries() []Entry {
	var out []Entry
	appData := os.Getenv("APPDATA")
	programData := os.Getenv("ProgramData")
	if appData != "" {
		out = append(out, entriesIn(filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup`), "Startup folder (user)", "user")...)
	}
	if programData != "" {
		out = append(out, entriesIn(filepath.Join(programData, `Microsoft\Windows\Start Menu\Programs\Startup`), "Startup folder (all users)", "machine")...)
	}
	return out
}

func entriesIn(dir, source, scope string) []Entry {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Entry
	for _, item := range items {
		if item.IsDir() || strings.EqualFold(item.Name(), "desktop.ini") {
			continue
		}
		out = append(out, Entry{
			Name:     item.Name(),
			Command:  filepath.Join(dir, item.Name()),
			Source:   source,
			Scope:    scope,
			Enabled:  true,
			Writable: false,
			Note:     "a shortcut in the Startup folder: enable or disable it there",
		})
	}
	return out
}

// SetEnabled writes the StartupApproved flag for one entry. User entries are
// changed directly; machine-wide ones need the elevated job.
func SetEnabled(entry Entry, enabled bool) error {
	if entry.Scope != "user" {
		return ErrNeedsAdmin
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, approvedPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetBinaryValue(entry.Name, ApprovedBlob(enabled))
}
