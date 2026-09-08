package services

import (
	"fmt"
	"time"

	"mikuchrono/internal/applog"
	"mikuchrono/internal/models"
	"mikuchrono/internal/store"
)

// goalNotifyCheckInterval is the ticker period of the notifier. The check is
// two small SQL reads plus per-activity arithmetic, so half-minute
// granularity keeps the idle cost negligible while still catching a goal
// reached mid-session promptly (the whole point of the live add-on below).
const goalNotifyCheckInterval = 30 * time.Second

// GoalNotifier watches the daily totals of every active activity and shows a
// system notification the first time one reaches its daily-goal minutes.
// It is a silent no-op when the goal_notify_enabled setting is "0" (default:
// on). The running session counts toward today's total via the live timer's
// session seconds, so a goal reached while timing is caught without waiting
// for the stop.
type GoalNotifier struct {
	Store *store.Store
	// Notify shows a system notification. When nil it falls back to the
	// package-default showNotification, backed by a tray balloon via the
	// //go:build windows implementation; tests inject a capture func here.
	Notify func(title, body string)
	// Emit, when set (wired in main.go), broadcasts goal:achieved app-wide
	// with the activity name so the desktop pet can play its celebration
	// easter egg in the same tick the notification fires. Nil in tests.
	Emit func(event string, data ...any)
	stop chan struct{}

	// lastCleanupDate remembers the day of the last stale-key sweep so the
	// delete runs once per day instead of on every tick. Only the
	// loop goroutine touches it.
	lastCleanupDate string
}

// NewGoalNotifier builds a notifier over the store.
func NewGoalNotifier(st *store.Store) *GoalNotifier {
	return &GoalNotifier{Store: st}
}

// Start launches the background check loop. Safe to call before Run.
func (g *GoalNotifier) Start() {
	g.stop = make(chan struct{})
	go g.loop()
}

// Stop terminates the check loop; idempotent.
func (g *GoalNotifier) Stop() {
	if g.stop == nil {
		return
	}
	select {
	case <-g.stop:
	default:
		close(g.stop)
	}
}

func (g *GoalNotifier) loop() {
	ticker := time.NewTicker(goalNotifyCheckInterval)
	defer ticker.Stop()
	// Check immediately on start too: a goal reached before this session's
	// launch (e.g. after a reboot) should still be announced promptly.
	g.check()
	for {
		select {
		case <-ticker.C:
			g.check()
		case <-g.stop:
			return
		}
	}
}

// check scans active activities once and notifies any that crossed their
// daily goal today and have not been notified yet. All failures are treated
// as transient: they are logged via applog (they used to vanish silently,
// which made "reminders stopped working" undiagnosable) and the next tick
// retries. A panic is recovered so one bad tick cannot kill the loop — or
// the app.
func (g *GoalNotifier) check() {
	defer func() {
		if r := recover(); r != nil {
			applog.Printf("goal notify: check panic: %v", r)
		}
	}()
	v, found, err := g.Store.GetSetting(keyGoalNotify)
	if err != nil {
		applog.Printf("goal notify: read setting: %v", err)
		return
	}
	if found && v == "0" {
		return
	}
	acts, err := g.Store.ListActivities(false)
	if err != nil {
		applog.Printf("goal notify: list activities: %v", err)
		return
	}
	today := store.Today(time.Now())
	// Retire dedupe keys from earlier days once per day so the meta table
	// does not grow without bound; a failure is transient and retried by
	// the next tick.
	if g.lastCleanupDate != today {
		if err := g.Store.DeleteStaleGoalNotifyKeys(today); err != nil {
			applog.Printf("goal notify: sweep stale keys: %v", err)
		} else {
			g.lastCleanupDate = today
		}
	}
	todaySecs, err := g.Store.TodaySecondsByActivity(today)
	if err != nil {
		applog.Printf("goal notify: today totals: %v", err)
		return
	}
	// The running timer's session seconds are not in the recorded totals yet.
	// A read error here just means the add-on is skipped this tick.
	var running *models.TimerState
	if st, err := g.Store.GetTimerState(time.Now()); err != nil {
		applog.Printf("goal notify: timer state: %v", err)
	} else if st.Running {
		running = &st
	}
	for _, a := range acts {
		if a.DailyGoalMinutes <= 0 {
			continue
		}
		total := todaySecs[a.ID]
		if running != nil && running.ActivityID == a.ID {
			total += sessionSecondsToday(running.StartedAt, time.Now())
		}
		if total < int64(a.DailyGoalMinutes)*60 {
			continue
		}
		// Per-day per-activity dedupe key ("<date>_<activityID>" after the
		// store.GoalNotifiedKeyPrefix prefix), written once the notification
		// has been shown.
		key := fmt.Sprintf("%s%s_%d", store.GoalNotifiedKeyPrefix, today, a.ID)
		if _, dup, err := g.Store.GetSetting(key); err != nil || dup {
			// A read error also skips: the next tick retries, so a transient
			// failure can never slip past the dedupe and double-notify.
			continue
		}
		// Mark before notifying so a notification failure still dedupes with
		// the next tick rather than spamming.
		if err := g.Store.SetSetting(key, "1"); err != nil {
			applog.Printf("goal notify: write dedupe key: %v", err)
			continue
		}
		notify := g.Notify
		if notify == nil {
			notify = showNotification
		}
		notify(
			"Miku Chrono · 目标达成",
			fmt.Sprintf("「%s」今日累计已达 %d 分钟目标 🎉", a.Name, a.DailyGoalMinutes),
		)
		if g.Emit != nil {
			g.Emit(EventGoalAchieved, a.Name)
		}
	}
}

// sessionSecondsToday returns the portion of the running session that belongs
// to the local day of now. A session started before midnight must not inflate
// today's total with seconds the recorded data attributes to yesterday — the
// day-splitting (store.loadDayPieces) clips every entry to each day's own
// slice, so the live add-on has to match that convention or a goal can fire
// on the wrong day's bucket. startedAt is a stored RFC3339 timestamp; an
// unparseable value contributes nothing (the next tick retries).
func sessionSecondsToday(startedAt string, now time.Time) int64 {
	start, err := store.ParseTime(startedAt)
	if err != nil {
		return 0
	}
	start = start.Local()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if now.Before(dayStart) {
		// Clock rolled back past midnight: nothing safely belongs to "today".
		return 0
	}
	if start.Before(dayStart) {
		start = dayStart
	}
	return elapsedSince(start, now)
}
