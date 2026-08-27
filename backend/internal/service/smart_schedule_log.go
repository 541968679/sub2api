package service

import "log/slog"

const (
	SmartScheduleLogCooldownStart = "cooldown_start"
	SmartScheduleLogSoftEnd       = "soft_end"
	SmartScheduleLogProbeEnter    = "probe_enter"
)

var logSmartScheduleEventFn = logSmartScheduleEventDefault

func logSmartScheduleEventDefault(event string, userID, accountID int64, platform, phase, reason string) {
	slog.Info(event,
		"event", event,
		"user_id", userID,
		"account_id", accountID,
		"platform", platform,
		"phase", phase,
		"reason", reason,
	)
}

// LogSmartScheduleEvent writes a searchable pair-state transition.
func LogSmartScheduleEvent(event string, userID, accountID int64, platform, phase, reason string) {
	if logSmartScheduleEventFn == nil {
		return
	}
	logSmartScheduleEventFn(event, userID, accountID, platform, phase, reason)
}
