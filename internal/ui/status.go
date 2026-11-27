package ui

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/metrics"
	"elom/locales"
)

// StartStatus synchronizes the metrics sampler with the active screen and
// window visibility. The sampler runs only while the Status screen is visible
// and the window is neither minimized nor hidden.
func (a *App) StartStatus() {
	a.syncStatusSampler()
}

// syncStatusSampler starts the 1 Hz sampler when the Status screen is actively
// shown and stops it whenever the user switches to another screen or minimizes
// the window, keeping idle background CPU and syscall usage at zero.
func (a *App) syncStatusSampler() {
	shouldRun := a.Win != nil && a.Screen == "status" && !a.winHidden && !a.Win.IsMinimized()
	if !shouldRun {
		if a.statusStarted {
			if a.statusCancel != nil {
				a.statusCancel()
				a.statusCancel = nil
			}
			a.statusStarted = false
		}
		return
	}
	if a.statusStarted {
		return
	}
	if a.status.At.IsZero() && a.Status != nil {
		if snap := a.Status.Snapshot(); !snap.At.IsZero() {
			a.status = snap
		}
	}
	a.statusStarted = true
	ctx, cancel := context.WithCancel(context.Background())
	a.statusCancel = cancel
	go a.Status.Start(ctx, func(snap metrics.Snapshot) {
		a.statusMu.Lock()
		a.statusLive = snap
		paused := a.statusPaused
		a.statusMu.Unlock()
		if paused {
			return
		}
		a.Win.Update(func() {
			if a.Screen != "status" || a.statusPaused {
				return
			}
			a.status = snap
		})
	})
}

// Stop ends the sampler, icon worker and releases the screen lock.
func (a *App) Stop() {
	if a.statusCancel != nil {
		a.statusCancel()
		a.statusCancel = nil
	}
	a.statusStarted = false
	if a.software.iconQueue != nil {
		close(a.software.iconQueue)
		a.software.iconQueue = nil
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

// displayedStatus returns the frozen snapshot while paused, or the latest
// live reading otherwise. The background sampler keeps running while paused so
// CPU delta counters and sparkline history never break.
func (a *App) displayedStatus() metrics.Snapshot {
	if a.statusPaused && !a.statusFrozen.At.IsZero() {
		return a.statusFrozen
	}
	return a.status
}

// toggleStatusPause freezes or resumes the Status view without stopping the
// background sampler.
func (a *App) toggleStatusPause() {
	a.statusMu.Lock()
	if a.statusPaused {
		a.statusPaused = false
		live := a.statusLive
		a.statusMu.Unlock()
		if !live.At.IsZero() && live.At.After(a.status.At) {
			a.status = live
		}
		return
	}
	a.statusFrozen = a.status
	a.statusPaused = true
	a.statusMu.Unlock()
}

// cachedSortedProcesses memoizes the sorted process table so hover/scroll
// repaints do not clone and re-sort the slice on every frame.
func (a *App) cachedSortedProcesses(snap metrics.Snapshot) []metrics.Process {
	if a.procSorted != nil &&
		a.procSortedAt.Equal(snap.At) &&
		a.procSortedByMem == a.procSortByMem &&
		a.procSortedLen == len(snap.Processes) {
		return a.procSorted
	}
	a.procSorted = sortedProcesses(snap.Processes, a.procSortByMem)
	a.procSortedAt = snap.At
	a.procSortedByMem = a.procSortByMem
	a.procSortedLen = len(snap.Processes)
	return a.procSorted
}

// sortedProcesses returns a sorted copy of procs ordered by CPU or Memory.
func sortedProcesses(procs []metrics.Process, byMem bool) []metrics.Process {
	out := append([]metrics.Process(nil), procs...)
	sort.SliceStable(out, func(i, j int) bool {
		if byMem {
			if out[i].Memory != out[j].Memory {
				return out[i].Memory > out[j].Memory
			}
			return out[i].CPU > out[j].CPU
		}
		if out[i].CPU != out[j].CPU {
			return out[i].CPU > out[j].CPU
		}
		return out[i].Memory > out[j].Memory
	})
	return out
}

// statusPage is Mole's grid of live tiles, four across, over the process
// table: health and the machine's facts first, then each resource with its
// number, its little chart and one line of detail.
func (a *App) statusPage(c *Context, pal palette) {
	snap := a.displayedStatus()

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
				detail := fmt.Sprintf("%s / %s", humanBytes(int64(snap.MemUsed)), humanBytes(int64(snap.MemTotal)))
				if snap.SwapTotal > 0 {
					detail += " · " + locales.StatusSwap(humanBytes(int64(snap.SwapUsed)), humanBytes(int64(snap.SwapTotal)))
				}
				hintText(c, detail)
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
				detail := locales.StatusUsed(humanBytes(int64(used)), int(percentOf(float64(used), float64(snap.SysTotal))))
				if extra := extraDrivesLine(snap.Drives); extra != "" {
					detail = extra
				}
				hintText(c, detail)
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

	a.processTable(c, pal, snap)
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

// chipTag is the small boxed fact in a tile's corner, as "2.64 GHz". It is
// the same chip as a task's tags, so one concept keeps one look.
func chipTag(c *Context, label string) {
	pillTag(c, label, fgSecondary())
}

// healthTile is the first tile: the score and its word, the machine's facts
// as chips, and the planet.
func (a *App) healthTile(c *Context, pal palette, snap metrics.Snapshot) {
	card(c).Height(164).Children(func() {
		Column(c).FillWidth().Gap(8).Children(func() {
			Row(c).FillWidth().Gap(5).AlignItems(Center).Children(func() {
				Icon(c, iconHeart).FontSize(12).TextColor(appTheme.Success)
				capsLabel(c, locales.StatusHealth(), fgSecondary())
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
						bigNumber(c, fmt.Sprint(snap.Health), "")
						Text(c, healthWord(snap.Health)).FontSize(13).TextColor(fgSecondary()).Margin(0, 0, 5, 0)
					})
					hintText(c, diagnosisLine(snap.Diagnosis, snap.Health))
					hintText(c, locales.StatusUptime()+" "+formatUptime(snap.Uptime))
				})
				planet(c, 84, pal)
			})
		})
	})
}

func diagnosisLine(d metrics.Diagnosis, health int) string {
	switch d.Kind {
	case "cpu":
		if d.Target != "" {
			return locales.StatusDiagCpuProc(trim(d.Target, 20))
		}
		return locales.StatusDiagCpu()
	case "memory":
		if d.Target != "" {
			return locales.StatusDiagMemProc(trim(d.Target, 20))
		}
		return locales.StatusDiagMem()
	case "disk":
		return locales.StatusDiagDisk(humanBytes(int64(d.FreeBytes)))
	case "disk_io":
		return locales.StatusDiagDiskIO()
	case "uptime":
		return locales.StatusDiagUptime()
	case "ok":
		return locales.StatusDiagOk()
	default:
		if health < 85 {
			return locales.StatusBusy()
		}
		return locales.StatusNormal()
	}
}

func extraDrivesLine(drives []metrics.Drive) string {
	if len(drives) <= 1 {
		return ""
	}
	parts := make([]string, 0, min(len(drives), 3))
	for _, d := range drives {
		if len(parts) >= 3 {
			break
		}
		parts = append(parts, locales.StatusDriveFree(d.Mount, humanBytes(int64(d.Free))))
	}
	return strings.Join(parts, " · ")
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

func (a *App) processTable(c *Context, pal palette, snap metrics.Snapshot) {
	rows := a.cachedSortedProcesses(snap)
	a.procTable.Selected = &a.procPicked
	peak := 1.0
	for _, proc := range rows {
		peak = math.Max(peak, proc.CPU)
	}
	card(c).Children(func() {
		Column(c).FillWidth().Gap(6).Children(func() {
			Row(c).FillWidth().Gap(8).AlignItems(Center).Children(func() {
				Box(c).Size(6, 6).Radius(3).Background(pal.glow)
				capsLabel(c, locales.StatusProcesses(), fgSecondary())
				if a.statusPaused {
					pillTag(c, locales.StatusPaused(), appTheme.Warning)
				}
				Spacer(c)
				sortLink(c, locales.StatusSortCpu(), !a.procSortByMem, func() { a.procSortByMem = false })
				sortLink(c, locales.StatusSortMem(), a.procSortByMem, func() { a.procSortByMem = true })
				pauseLabel := locales.StatusPause()
				if a.statusPaused {
					pauseLabel = locales.StatusResume()
				}
				if ghostPill(c, pauseLabel).Clicked() {
					a.toggleStatusPause()
				}
			})
			if a.statusKillErr != "" {
				Text(c, a.statusKillErr).FontSize(sizeMeta).TextColor(appTheme.Danger)
			}
			// A sampler that has not answered yet, or a machine that reports
			// nothing, still leaves the card in place with a reason; the old
			// view dropped the whole section and looked broken.
			if len(rows) == 0 {
				hintText(c, locales.StatusNoProcesses())
				return
			}
			rule(c)
			Table(c, &a.procTable, []TableColumn{
				{Title: locales.StatusColName(), Width: 0, Fixed: true},
				{Title: locales.StatusColCpu(), ID: "cpu", Width: 150, Align: End, Fixed: true},
				{Title: locales.StatusColMem(), Width: 96, Align: End, Fixed: true},
				{Title: locales.StatusColPid(), Width: 72, Align: End, Fixed: true},
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
		if err := metrics.KillProcess(proc.PID); err != nil {
			a.update(func() {
				a.statusKillErr = locales.StatusKillFailed(proc.Name, err.Error())
			})
			return
		}
		a.update(func() { a.statusKillErr = "" })
	}()
}
