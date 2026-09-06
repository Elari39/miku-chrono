package services

import (
	"fmt"
	"time"

	"mikuchrono/internal/models"
	"mikuchrono/internal/store"
)

// goalNotifyCheckInterval is the ticker period of the notifier. The check is
// two small SQL reads plus per-activity arithmetic, so one minute granularity
// keeps the idle cost negligible; a goal reached mid-session is announced
// within one minute instead of exactly at the second.
const goalNotifyCheckInterval = time.Minute

// GoalNotifier watches the daily totals of every active activity and shows a
// system notification the first time one reaches its daily-goal minutes.
// It is a silent no-op when the goal_notify_enabled setting is "0" (default:
// on). The running session counts toward today's total via the live timer's
// session seconds, so a goal reached while timing is caught without waiting
// for the stop.
type GoalNotifier struct {
	Store *store.Store
	stop  chan struct{}

	// lastCleanupDate remembers the day of the last stale-key sweep so the
	// delete runs once per day instead of on every minute tick. Only the
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
// as transient and silent: the next tick retries.
func (g *GoalNotifier) check() {
	v, found, err := g.Store.GetSetting(keyGoalNotify)
	if err != nil {
		return
	}
	if found && v == "0" {
		return
	}
	acts, err := g.Store.ListActivities(false)
	if err != nil {
		return
	}
	today := store.Today(time.Now())
	// Retire dedupe keys from earlier days once per day so the meta table
	// does not grow without bound; a failure is transient and retried by
	// the next tick.
	if g.lastCleanupDate != today {
		if err := g.Store.DeleteStaleGoalNotifyKeys(today); err == nil {
			g.lastCleanupDate = today
		}
	}
	todaySecs, err := g.Store.TodaySecondsByActivity(today)
	if err != nil {
		return
	}
	// The running timer's session seconds are not in the recorded totals yet.
	var running *models.TimerState
	if st, err := g.Store.GetTimerState(time.Now()); err == nil && st.Running {
		running = &st
	}
	for _, a := range acts {
		if a.DailyGoalMinutes <= 0 {
			continue
		}
		total := todaySecs[a.ID]
		if running != nil && running.ActivityID == a.ID {
			total += running.SessionElapsedSeconds
		}
		if total < int64(a.DailyGoalMinutes)*60 {
			continue
		}
		// Per-day per-activity dedupe key ("<date>_<activityID>" after the
		// store.GoalNotifiedKeyPrefix prefix), written once the notification
		// has been shown.
		key := fmt.Sprintf("%s%s_%d", store.GoalNotifiedKeyPrefix, today, a.ID)
		if _, dup, err := g.Store.GetSetting(key); err == nil && dup {
			continue
		}
		// Mark before notifying so a notification failure still dedupes with
		// the next tick rather than spamming.
		if err := g.Store.SetSetting(key, "1"); err != nil {
			continue
		}
		showNotification(
			"Miku Chrono · 目标达成",
			fmt.Sprintf("「%s」今日累计已达 %d 分钟目标 🎉", a.Name, a.DailyGoalMinutes),
		)
	}
}
