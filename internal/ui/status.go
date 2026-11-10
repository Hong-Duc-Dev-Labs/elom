package ui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/metrics"
	"elom/locales"
)

// StartStatus starts the 1 Hz sampler and keeps the view fed while the app
// runs, including while only the tray panel shows.
func (a *App) StartStatus() {
	if a.statusStarted || a.Win == nil {
		return
	}
	a.statusStarted = true
	ctx, cancel := context.WithCancel(context.Background())
	a.statusCancel = cancel
	go a.Status.Start(ctx, func(snap metrics.Snapshot) {
		a.Win.Update(func() { a.status = snap })
	})
}

// Stop ends the sampler and releases the screen lock.
func (a *App) Stop() {
	if a.statusCancel != nil {
		a.statusCancel()
		a.statusCancel = nil
	}
	a.setKeepScreenOn(false)
}

// setKeepScreenOn turns the power assertion on or off. MyGo keeps the display
// awake for the reason given, and the release function ends it.
func (a *App) setKeepScreenOn(on bool) {
	if on == a.keepOn && (on == (a.release != nil)) {
		return
	}
	a.keepOn = on
	if !on {
		if a.release != nil {
			a.release()
			a.release = nil
		}
		return
	}
	if a.release == nil {
		a.release = mygo.Power.KeepAwake(locales.StatusKeepOn(), true)
	}
}

// statusPage is Mole's grid of live tiles, four across, over the process
// table: health and the machine's facts first, then each resource with its
// number, its little chart and one line of detail.
func (a *App) statusPage(c *Context, pal palette) {
	snap := a.status

	if snap.At.IsZero() {
		Row(c).Gap(10).AlignItems(Center).Padding(16, 4).Children(func() {
			Spinner(c)
			Text(c, locales.StatusStarting()).FontSize(12.5).TextColor(appTheme.TextMuted)
		})
		return
	}

	mem := percentOf(float64(snap.MemUsed), float64(snap.MemTotal))
	tiles := []func(){
		func() { a.healthTile(c, pal, snap) },
		func() {
			cores := len(snap.Cores)
			if cores == 0 {
				cores = snap.System.Cores
			}
			meta := ""
			if cores > 0 {
				meta = locales.StatusCores(cores)
			}
			tile(c, iconCPU, accentMint, locales.StatusCpu(), meta, func() {
				bigNumber(c, fmt.Sprintf("%.0f", snap.CPU), "%")
				if len(snap.Cores) > 0 {
					perCoreBars(c, snap.Cores, 34)
				} else {
					barsChart(c, snap.History.CPU, 100, accentMint, 34)
				}
				if cores > 0 {
					hintText(c, locales.StatusCores(cores))
				}
			})
		},
		func() {
			chip := locales.StatusPressureNormal()
			if mem > 85 {
				chip = locales.StatusPressureHigh()
			}
			tile(c, iconMemory, Hex("#e3c27a"), locales.StatusMemory(), chip, func() {
				bigNumber(c, fmt.Sprintf("%.0f", mem), "%")
				bar(c, float32(mem/100), Hex("#d8c48f"), 14)
				hintText(c, fmt.Sprintf("%s / %s", humanBytes(int64(snap.MemUsed)), humanBytes(int64(snap.MemTotal))))
			})
		},
		func() {
			chip := ""
			if snap.SysTotal > 0 {
				chip = humanBytes(int64(snap.SysTotal))
			}
			used := snap.SysTotal - snap.SysFree
			tile(c, iconDrive, Hex("#7aa7ff"), locales.StatusFreeSpace(), chip, func() {
				value, unit := splitBytes(int64(snap.SysFree))
				bigNumber(c, value, unit)
				bar(c, float32(percentOf(float64(used), float64(snap.SysTotal))/100), Hex("#5b8fe8"), 6)
				hintText(c, locales.StatusUsed(humanBytes(int64(used)), int(percentOf(float64(used), float64(snap.SysTotal)))))
			})
		},
		func() {
			down, up := snap.History.NetDown, snap.History.NetUp
			peak := seriesMax(append(append([]float64(nil), down...), up...))
			tile(c, iconNetwork, Hex("#7aa7ff"), locales.StatusNetwork(), "", func() {
				value, unit := splitBytes(int64(snap.NetRecv + snap.NetSent))
				bigNumber(c, value, unit+"/s")
				netChart(c, down, up, peak, 34)
				hintText(c, locales.StatusDown(humanBytes(int64(snap.NetRecv))+"/s")+" · "+
					locales.StatusUp(humanBytes(int64(snap.NetSent))+"/s"))
			})
		},
		func() {
			tile(c, iconActivity, Hex("#8fa8ff"), locales.StatusDiskActivity(), "", func() {
				value, unit := splitBytes(int64(snap.DiskRead + snap.DiskWrite))
				bigNumber(c, value, unit+"/s")
				lineChart(c, snap.History.Disk, seriesMax(snap.History.Disk), Hex("#8fa8ff"), 34)
				hintText(c, locales.StatusRead(humanBytes(int64(snap.DiskRead))+"/s")+" · "+
					locales.StatusWrite(humanBytes(int64(snap.DiskWrite))+"/s"))
			})
		},
	}
	if b := snap.Battery; b != nil {
		tiles = append(tiles, func() {
			tile(c, iconBattery, Hex("#8ad9a6"), locales.StatusBattery(), "", func() {
				bigNumber(c, fmt.Sprint(b.Percent), "%")
				bar(c, float32(b.Percent)/100, accentMint, 8)
				hintText(c, batteryLine(b))
			})
		})
	}
	tiles = append(tiles, func() {
		tile(c, iconSun, Hex("#f2c078"), locales.StatusKeepOn(), "", func() {
			on := a.keepOn
			if toggle(c, &on, locales.StatusKeepOn()).Changed() {
				a.setKeepScreenOn(on)
			}
			Spacer(c)
			hintText(c, keepLabel(on))
		})
	})

	// Four tiles a row; the last row's tiles grow to fill it. A plain
	// Columns(4) grid left a 276x164 hole when the count was seven.
	for start := 0; start < len(tiles); start += 4 {
		end := min(start+4, len(tiles))
		Row(c).FillWidth().Gap(12).Children(func() {
			for _, build := range tiles[start:end] {
				Box(c).Grow(1).Children(build)
			}
		})
	}

	a.processTable(c, pal)
}

// tile is one of the Status grid's cards: a colored glyph and label in
// capitals, an optional chip on the right, then the reading.
func tile(c *Context, icon *SVG, accent Color, label, chip string, body func()) {
	card(c).Height(164).Children(func() {
		Column(c).FillWidth().Gap(8).Children(func() {
			tileHead(c, icon, accent, label, chip)
			body()
		})
	})
}

func tileHead(c *Context, icon *SVG, accent Color, label, chip string) {
	Row(c).FillWidth().Gap(7).AlignItems(Center).Children(func() {
		Icon(c, icon).FontSize(12).TextColor(accent)
		capsLabel(c, label, fgSecondary())
		Spacer(c)
		if chip != "" {
			chipTag(c, chip)
		}
	})
}

// chipTag is the small boxed fact in a tile's corner, as "2.64 GHz".
func chipTag(c *Context, label string) {
	Box(c).Padding(2, 7).Radius(5).Border(1, RGBA(255, 255, 255, 0.14)).Children(func() {
		Text(c, label).Font(fontMono).FontSize(sizeFootnote).TextColor(fgSecondary()).NoWrap()
	})
}

// healthTile is the first tile: the score and its word, the machine's facts
// as chips, and the planet.
func (a *App) healthTile(c *Context, pal palette, snap metrics.Snapshot) {
	card(c).Height(164).Children(func() {
		Column(c).FillWidth().Gap(8).Children(func() {
			Row(c).FillWidth().Gap(5).AlignItems(Center).Children(func() {
				Icon(c, iconHeart).FontSize(12).TextColor(appTheme.Success)
				Text(c, strings.ToUpper(locales.StatusHealth())).FontSize(sizeFootnote).FontWeight(700).
					LetterSpacing(0.8).TextColor(fgSecondary()).NoWrap()
				Spacer(c)
				for _, fact := range []string{snap.System.CPUModel, totalRAM(snap.MemTotal), snap.System.OS} {
					if fact != "" {
						chipTag(c, fact)
					}
				}
			})
			Row(c).FillWidth().AlignItems(Center).Children(func() {
				Column(c).Grow(1).Gap(6).Children(func() {
					Row(c).Gap(8).AlignItems(End).Children(func() {
						Text(c, fmt.Sprint(snap.Health)).Font(fontMono).FontSize(26).FontWeight(500).TextColor(fgPrimary())
						Text(c, healthWord(snap.Health)).FontSize(13).TextColor(fgSecondary()).Margin(0, 0, 5, 0)
					})
					line := locales.StatusNormal()
					if snap.Health < 85 {
						line = locales.StatusBusy()
					}
					hintText(c, line)
					hintText(c, locales.StatusUptime()+" "+formatUptime(snap.Uptime))
				})
				planet(c, 84, pal)
			})
		})
	})
}

// healthWord names a score as Mole does beside it.
func healthWord(score int) string {
	switch {
	case score >= 90:
		return locales.StatusExcellent()
	case score >= 70:
		return locales.StatusGood()
	case score >= 50:
		return locales.StatusFair()
	default:
		return locales.StatusPoor()
	}
}

// totalRAM rounds the memory size to whole gigabytes, as a spec sheet does.
func totalRAM(bytes uint64) string {
	if bytes == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f GB", math.Round(float64(bytes)/(1<<30)))
}

// splitBytes is humanBytes as a number and its unit, for a big reading.
func splitBytes(n int64) (string, string) {
	text := humanBytes(n)
	if at := strings.LastIndex(text, " "); at > 0 {
		return text[:at], text[at+1:]
	}
	return text, ""
}

func batteryLine(b *metrics.Battery) string {
	if b.Plugged || b.Remaining <= 0 {
		return locales.StatusCharging()
	}
	return locales.StatusLeft(formatUptime(b.Remaining))
}

func keepLabel(on bool) string {
	if on {
		return locales.StatusKeepOn()
	}
	return locales.StatusKeepOff()
}

func seriesMax(series []float64) float64 {
	peak := 0.0
	for _, v := range series {
		if v > peak {
			peak = v
		}
	}
	if peak < 1024 {
		peak = 1024
	}
	return peak
}

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func (a *App) processTable(c *Context, pal palette) {
	rows := a.status.Processes
	if len(rows) == 0 {
		return
	}
	a.procTable.Selected = &a.procPicked
	peak := 1.0
	for _, proc := range rows {
		peak = math.Max(peak, proc.CPU)
	}
	card(c).Padding(10, 14).Children(func() {
		Column(c).FillWidth().Gap(6).Children(func() {
			Row(c).FillWidth().AlignItems(Center).Gap(8).Padding(4, 2, 0, 2).Children(func() {
				capsLabel(c, locales.StatusProcesses(), fgMuted())
				Spacer(c)
			})
			Table(c, &a.procTable, []TableColumn{
				{Title: "NAME", Width: 0, Fixed: true},
				{Title: "CPU", ID: "cpu", Width: 150, Align: End, Fixed: true},
				{Title: "MEM", Width: 96, Align: End, Fixed: true},
				{Title: "PID", Width: 72, Align: End, Fixed: true},
				{Title: "", ID: "end", Width: 34, Fixed: true},
			}, len(rows), func(row, col int) {
				proc := rows[row]
				switch col {
				case 0:
					Row(c).Gap(8).AlignItems(Center).Children(func() {
						Box(c).Size(16, 16).Radius(4).Background(palaceAccent(proc.Name, pal)).AlignItems(Center).Justify(Center).Children(func() {
							initial := "?"
							if r := []rune(proc.Name); len(r) > 0 {
								initial = strings.ToUpper(string(r[0]))
							}
							Text(c, initial).Font(fontMono).FontSize(9).FontWeight(600).
								TextColor(readableOn(palaceAccent(proc.Name, pal)))
						})
						Text(c, trim(proc.Name, 44)).FontSize(12.5).NoWrap()
					})
				case 1:
					Row(c).FillWidth().Gap(8).AlignItems(Center).Justify(End).Children(func() {
						bar(c, float32(proc.CPU/peak), accentMint, 3).Width(72)
						Text(c, fmt.Sprintf("%.1f", proc.CPU)).Font(fontMono).FontSize(11).NoWrap()
					})
				case 2:
					Text(c, humanBytes(int64(proc.Memory))).Font(fontMono).FontSize(11.5).NoWrap()
				case 3:
					Text(c, fmt.Sprint(proc.PID)).Font(fontMono).FontSize(11).TextColor(fgMuted()).NoWrap()
				case 4:
					if iconButton(c, iconMore, locales.StatusEndTask()).Clicked() {
						a.endTask(proc)
					}
				}
			}).Height(float32(min(260, 72+34*len(rows))))
		})
	})
}

// palaceAccent gives a process row's glyph tile a stable tone inside the
// screen's planet ink: two neighbouring steps, not an unrelated hue.
func palaceAccent(name string, pal palette) Color {
	k := 0.0
	if len(name)%2 == 0 {
		k = 0.35
	}
	return pal.ink[0].Mix(pal.ink[1], float32(k))
}

func (a *App) endTask(proc metrics.Process) {
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             a.Win,
			Type:               mygo.MessageWarning,
			Message:            locales.StatusEndTaskTitle(proc.Name),
			Detail:             locales.StatusEndTaskDetail(),
			Buttons:            []string{locales.CleanCancel(), locales.StatusEndTask()},
			DefaultButton:      1,
			CancelButton:       0,
			DestructiveButtons: []int{1},
		})
		if err != nil || res.Button != 1 {
			return
		}
		_ = metrics.KillProcess(proc.PID)
	}()
}
