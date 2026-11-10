//go:build !windows

package metrics

import "runtime"

func readBattery() *Battery { return nil }

func osLabel() string { return runtime.GOOS }
