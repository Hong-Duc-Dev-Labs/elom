//go:build windows

package metrics

import (
	"strconv"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var procGetSystemPowerStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemPowerStatus")

// systemPowerStatus is SYSTEM_POWER_STATUS.
type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

// readBattery asks Windows for the battery's charge. It returns nil on a
// machine without a battery, or when Windows does not know.
func readBattery() *Battery {
	var st systemPowerStatus
	if r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&st))); r == 0 {
		return nil
	}
	const noBattery, unknown = 128, 255
	if st.BatteryFlag&noBattery != 0 || st.BatteryFlag == unknown || st.BatteryLifePercent == unknown {
		return nil
	}
	b := &Battery{Percent: int(st.BatteryLifePercent), Plugged: st.ACLineStatus == 1}
	if st.BatteryLifeTime != 0xFFFFFFFF && !b.Plugged {
		b.Remaining = time.Duration(st.BatteryLifeTime) * time.Second
	}
	return b
}

// osLabel names Windows as Settings › About does: "Windows 11 25H2". The
// registry's ProductName still says Windows 10 on 11, so the build decides.
func osLabel() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Windows"
	}
	defer key.Close()
	name := "Windows 10"
	if build, _, err := key.GetStringValue("CurrentBuild"); err == nil {
		if n, err := strconv.Atoi(build); err == nil && n >= 22000 {
			name = "Windows 11"
		}
	}
	if display, _, err := key.GetStringValue("DisplayVersion"); err == nil && display != "" {
		return name + " " + display
	}
	return name
}
