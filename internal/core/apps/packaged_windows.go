//go:build windows

package apps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// packagedApp mirrors the JSON PowerShell writes for Get-AppxPackage.
type packagedApp struct {
	Name             string `json:"Name"`
	Version          string `json:"Version"`
	PublisherDisplay string `json:"PublisherDisplayName"`
	InstallLocation  string `json:"InstallLocation"`
	PackageFullName  string `json:"PackageFullName"`
}

// EnumeratePackaged lists Microsoft Store and other packaged apps. It reads
// them through PowerShell and returns JSON rather than a table, which survives
// localization.
func EnumeratePackaged(ctx context.Context) ([]App, error) {
	script := "Get-AppxPackage | Select-Object Name,Version,PublisherDisplayName,InstallLocation,PackageFullName | ConvertTo-Json -Compress"
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("apps: Get-AppxPackage: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return ParsePackaged(out)
}

// ParsePackaged decodes ConvertTo-Json output: an array, a single object, or
// nothing at all.
func ParsePackaged(data []byte) ([]App, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var raw []packagedApp
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return nil, fmt.Errorf("apps: packaged apps: %w", err)
		}
	} else {
		var one packagedApp
		if err := json.Unmarshal(trimmed, &one); err != nil {
			return nil, fmt.Errorf("apps: packaged app: %w", err)
		}
		raw = []packagedApp{one}
	}
	out := make([]App, 0, len(raw))
	for _, p := range raw {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.PackageFullName) == "" {
			continue
		}
		out = append(out, App{
			ID:              p.PackageFullName,
			Name:            p.Name,
			Version:         p.Version,
			Publisher:       p.PublisherDisplay,
			InstallLocation: p.InstallLocation,
			Kind:            KindAppx,
		})
	}
	SortApps(out)
	return out, nil
}
