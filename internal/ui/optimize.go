package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	. "github.com/egoist/mygo/ui"

	"elom/internal/core/maint"
	"elom/internal/core/oplog"
	"elom/internal/svc"
	"elom/locales"
)

type optimizeState struct {
	running string
	results map[string]maint.Result
	err     string
	note    string
}

func newOptimizeState() optimizeState {
	return optimizeState{results: map[string]maint.Result{}}
}

// userTasks are the maintenance tasks the screen offers. The machine-wide
// startup write is a task of the job protocol, run from the Startup tab with
// an entry's name, not something to run on its own.
func userTasks() []maint.Task {
	var out []maint.Task
	for _, task := range maint.Tasks {
		if task.Op == "startup.set-hklm" {
			continue
		}
		out = append(out, task)
	}
	return out
}

// routineTasks are what the Optimize button runs: every offered task that is
// not marked careful.
func routineTasks() []maint.Task {
	var out []maint.Task
	for _, task := range userTasks() {
		if !task.Danger {
			out = append(out, task)
		}
	}
	return out
}

// optimizePage is Mole's: the planet, two quiet lines and one button, with
// the tasks it stands for listed below it.
func (a *App) optimizePage(c *Context, pal palette) {
	st := &a.optimize

	Column(c).FillWidth().AlignItems(Center).Padding(28, 0, 10, 0).Gap(28).Children(func() {
		planet(c, 196, pal)
		Column(c).FillWidth().AlignItems(Center).Gap(14).Children(func() {
			Text(c, locales.OptimizePoem()).Font(fontMono).FontSize(12.5).LineHeight(1.7).
				TextAlign(Center).TextColor(softText())
			if st.running != "" {
				Row(c).Gap(10).AlignItems(Center).Children(func() {
					Spinner(c)
					Text(c, st.note).FontSize(12.5).TextColor(appTheme.TextMuted)
				})
			} else if primaryPill(c, locales.OptimizeAll()).Width(220).Height(44).Clicked() {
				a.runRoutine()
			}
			if st.note != "" && st.running == "" {
				Text(c, st.note).FontSize(12).TextColor(softText())
			}
			hintText(c, locales.OptimizeAllHint())
		})
	})

	cardWith(c, 4, func() {
		cardHead(c, pal, strings.ToUpper(locales.OptimizeTasks()), "")
		Box(c).Height(6)
		tasks := userTasks()
		for i, task := range tasks {
			task := task
			row := Row(c).FillWidth().Gap(12).AlignItems(Center).Padding(9, 8)
			row.Children(func() {
				Column(c).Grow(1).Gap(3).Children(func() {
					Row(c).Gap(8).AlignItems(Center).Children(func() {
						Text(c, task.Title).FontSize(13).FontWeight(600).TextColor(appTheme.Text)
						if task.Admin {
							pillTag(c, locales.OptimizeAdmin(), appTheme.Warning)
						}
						if task.Danger {
							pillTag(c, locales.OptimizeCareful(), appTheme.Danger)
						}
					})
					Text(c, task.Detail).FontSize(11).TextColor(appTheme.TextMuted)
					if result, ok := st.results[task.Op]; ok {
						line := locales.OptimizeDone(result.Detail)
						color := appTheme.Success
						if !result.OK {
							line = locales.OptimizeFailed(result.Err)
							color = appTheme.Danger
						}
						Text(c, trim(line, 160)).Font(fontMono).FontSize(sizeMicro).TextColor(color)
					}
				})
				switch {
				case st.running == task.Op:
					Spinner(c)
				case st.running == "":
					if ghostPill(c, locales.OptimizeRun()).Clicked() {
						a.runTask(task)
					}
				}
			})
			if i < len(tasks)-1 {
				rowDivider(c)
			}
		}
		Box(c).Height(4)
		hintText(c, locales.OptimizeNote())
	})
}

// runRoutine runs the routine tasks: the ones a user may run directly first,
// then those that need an administrator in one elevated job, so one prompt
// covers them all.
func (a *App) runRoutine() {
	st := &a.optimize
	if st.running != "" {
		return
	}
	var plain, admin []maint.Task
	for _, task := range routineTasks() {
		if task.Admin {
			admin = append(admin, task)
		} else {
			plain = append(plain, task)
		}
	}
	st.running = "routine"
	st.note = locales.OptimizeRunning(locales.OptimizeAll())
	go func() {
		results := map[string]maint.Result{}
		for _, task := range plain {
			result := maint.Run(context.Background(), maint.Op{Name: task.Op})
			a.logMaintenance(task, result)
			results[task.Op] = result
		}
		cancelled := false
		if len(admin) > 0 {
			ops := make([]maint.Op, len(admin))
			for i, task := range admin {
				ops[i] = maint.Op{Name: task.Op}
			}
			got, err := a.Elevate(context.Background(), ops)
			switch {
			case errors.Is(err, svc.ErrCancelled):
				cancelled = true
			case err != nil:
				for _, task := range admin {
					results[task.Op] = maint.Result{Op: task.Op, Err: err.Error()}
				}
			default:
				for _, result := range got {
					results[result.Op] = result
				}
			}
			for _, task := range admin {
				if result, ok := results[task.Op]; ok {
					a.logMaintenance(task, result)
				}
			}
		}
		a.update(func() {
			ok, failed := 0, 0
			for op, result := range results {
				st.results[op] = result
				if result.OK {
					ok++
				} else {
					failed++
				}
			}
			st.running = ""
			st.note = locales.OptimizeSummaryLine(ok, failed)
			if cancelled {
				st.note += " · " + locales.OptimizeCancelled()
			}
		})
	}()
}

func (a *App) runTask(task maint.Task) {
	st := &a.optimize
	op := maint.Op{Name: task.Op}
	go func() {
		if task.Danger {
			res, err := mygo.Dialog.Message(mygo.MessageOptions{
				Parent:        a.Win,
				Type:          mygo.MessageWarning,
				Message:       locales.OptimizeConfirmTitle(task.Title),
				Detail:        task.Detail,
				Buttons:       []string{locales.CleanCancel(), locales.OptimizeRun()},
				DefaultButton: 1,
				CancelButton:  0,
			})
			if err != nil || res.Button != 1 {
				return
			}
		}
		a.Win.Update(func() {
			st.running = task.Op
			st.note = locales.OptimizeRunning(task.Title)
		})

		var result maint.Result
		if task.Admin {
			results, err := a.Elevate(context.Background(), []maint.Op{op})
			switch {
			case err == svc.ErrCancelled:
				a.Win.Update(func() {
					st.running = ""
					st.note = locales.OptimizeCancelled()
				})
				return
			case err != nil:
				result = maint.Result{Op: task.Op, Err: err.Error()}
			case len(results) > 0:
				result = results[0]
			}
		} else {
			result = maint.Run(context.Background(), op)
		}
		a.logMaintenance(task, result)
		a.Win.Update(func() {
			st.running = ""
			st.note = ""
			st.results[task.Op] = result
		})
	}()
}

// logMaintenance records the operation and its outcome; an operation is not a
// file deletion, so it writes one record with the "done" outcome.
func (a *App) logMaintenance(task maint.Task, result maint.Result) {
	if a.Log == nil {
		return
	}
	outcome := oplog.ResultDone
	errText := ""
	if !result.OK {
		outcome = oplog.ResultFailed
		errText = result.Err
	}
	_ = a.Log.Append(oplog.Record{
		RunID:  svc.NewRunID(),
		Op:     task.Op,
		Path:   fmt.Sprintf("maintenance: %s", task.Title),
		Result: outcome,
		Err:    errText,
	})
}
