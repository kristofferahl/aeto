package sustainability

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	sustainabilityv1alpha1 "github.com/kristofferahl/aeto/apis/sustainability/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/util"
)

const (
	AnnotationSuspendFor   string = "sustainability.aeto.net/suspend-for"
	AnnotationSuspendUntil string = "sustainability.aeto.net/suspend-until"
)

var weekdays = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}

func (r SavingsPolicyReconciler) reconcileSuspendFor(rctx reconcile.Context, savingspolicy sustainabilityv1alpha1.SavingsPolicy) (changed bool, err error) {
	if val, ok := savingspolicy.Annotations[AnnotationSuspendFor]; ok {
		duration, err := time.ParseDuration(val)
		if err != nil {
			return false, err
		}

		until := time.Now().UTC().Add(duration).Format(time.RFC3339)
		rctx.Log.Info("suspending the SavingsPolicy", "for", duration, "until", until)

		delete(savingspolicy.Annotations, AnnotationSuspendFor)
		savingspolicy.Annotations[AnnotationSuspendUntil] = until

		if err = r.Client.Update(rctx, &savingspolicy); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (r SavingsPolicyReconciler) reconcileSuspendUntil(rctx reconcile.Context, savingspolicy sustainabilityv1alpha1.SavingsPolicy) (changed bool, err error) {
	if val, ok := savingspolicy.Annotations[AnnotationSuspendUntil]; ok {
		until, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return false, err
		}

		if until.Before(time.Now()) {
			rctx.Log.Info("enabling SavingsPolicy ", "reason", fmt.Sprintf("annotation %s has a value that is in the past", AnnotationSuspendUntil))
			delete(savingspolicy.Annotations, AnnotationSuspendUntil)

			if err = r.Client.Update(rctx, &savingspolicy); err != nil {
				return false, err
			}

			return true, nil
		}
	}
	return false, nil
}

func checkSchedule(rctx reconcile.Context, savingspolicy sustainabilityv1alpha1.SavingsPolicy) (suspended bool, reason string) {
	if val, ok := savingspolicy.Annotations[AnnotationSuspendUntil]; ok {
		until, err := time.Parse(time.RFC3339, val)
		if err != nil {
			reason = fmt.Sprintf("annotation %s is present but has an invalid RFC339 time value", AnnotationSuspendUntil)
			rctx.Log.Error(err, reason)
			return true, reason
		} else {
			if until.After(time.Now()) {
				reason = fmt.Sprintf("SavingsPolicy is suspended until %s as annotation %s is set", until.Format(time.RFC3339), AnnotationSuspendUntil)
				rctx.Log.Info("SavingsPolicy is suspended", "until", until.Format(time.RFC3339), "reason", reason)
				return true, reason
			}
		}
	}

	expr, _ := regexp.Compile(`^([a-zA-Z]{3})-([a-zA-Z]{3}) (\d\d):(\d\d)-(\d\d):(\d\d) (?P<tz>[a-zA-Z/_]+)$`)
	timestamp := time.Now().UTC()

	for _, pattern := range savingspolicy.Spec.Suspended {
		match := expr.FindStringSubmatch(pattern)
		if len(match) == 8 {
			now := NewWeekdayTimeUTC(timestamp.Weekday().String(), fmt.Sprint(timestamp.Hour()), fmt.Sprint(timestamp.Minute()))
			wdr, err := NewWeekdayRange(match[1], match[2])
			if err != nil {
				rctx.Log.Error(err, "checking range patterns failed, skipping", "pattern", pattern)
				continue
			}

			failed := false
			for wdr.HasNext() && !suspended && !failed {
				weekday := wdr.Next()
				from, err := NewWeekdayTimeLocal(weekday, match[3], match[4], match[7])
				if err != nil {
					rctx.Log.Error(err, "checking range patterns failed, skipping", "pattern", pattern)
					failed = true
					continue
				}
				to, err := NewWeekdayTimeLocal(weekday, match[5], match[6], match[7])
				if err != nil {
					rctx.Log.Error(err, "checking range patterns failed, skipping", "pattern", pattern)
					failed = true
					continue
				}
				inRange := now.InRange(from, to)

				rctx.Log.V(2).Info("checking", "pattern", pattern, "in-range", inRange, "now", now, "from", from, "to", to)
				if inRange {
					suspended = true
					reason = fmt.Sprintf("SavingsPolicy is suspended as it matches the time range pattern %s", pattern)
					rctx.Log.Info(reason)
					break
				}
			}
		} else {
			rctx.Log.Info(fmt.Sprintf("invalid time range %s", pattern))
		}
	}

	if !suspended {
		reason = "SavingsPolicy is active (matches no time range patterns)"
	}

	return suspended, reason
}

type WeekdayRange struct {
	From         string
	To           string
	fromIndex    int
	toIndex      int
	numberOfDays int
	index        int
	ubound       int
}

func NewWeekdayRange(from, to string) (*WeekdayRange, error) {
	wr := WeekdayRange{}
	wr.From = strings.ToUpper(from)[:3]
	wr.To = strings.ToUpper(to)[:3]

	wr.fromIndex = util.IndexOfString(wr.From, weekdays)
	if wr.fromIndex == -1 {
		return nil, fmt.Errorf("WeekdayRange error, weekday not found in weekdays (from=%s)", wr.From)
	}
	wr.toIndex = util.IndexOfString(wr.To, weekdays)
	if wr.toIndex == -1 {
		return nil, fmt.Errorf("WeekdayRange error, weekday not found in weekdays (to=%s)", wr.To)
	}

	wr.numberOfDays = 0
	if wr.toIndex >= wr.fromIndex {
		wr.numberOfDays = wr.toIndex - wr.fromIndex + 1
	} else {
		wr.numberOfDays = 8 - (wr.fromIndex - wr.toIndex)
	}

	wr.ubound = wr.fromIndex + wr.numberOfDays

	wr.Reset()

	return &wr, nil
}

func (wr *WeekdayRange) HasNext() bool {
	return wr.index < wr.ubound
}

func (wr *WeekdayRange) Next() string {
	if wr.HasNext() {
		weekdayIndex := (wr.index) % 7
		weekday := weekdays[weekdayIndex]
		wr.index++
		return weekday
	}
	return ""
}

func (wr *WeekdayRange) Reset() {
	wr.index = wr.fromIndex
}

func (wr *WeekdayRange) String() string {
	days := make([]string, 0)
	for wr.HasNext() {
		days = append(days, wr.Next())
	}
	wr.Reset()
	return strings.Join(days, ",")
}

type WeekdayTime struct {
	ts *time.Time
}

func NewWeekdayTimeUTC(weekday, hour, minute string) WeekdayTime {
	wdt, _ := NewWeekdayTimeLocal(weekday, hour, minute, "UTC")
	return wdt
}

func NewWeekdayTimeLocal(weekday, hour, minute, tz string) (WeekdayTime, error) {
	wd := util.IndexOfString(strings.ToUpper(weekday[:3]), weekdays)
	if wd == -1 {
		return WeekdayTime{}, fmt.Errorf("WeekdayTimeLocal error, weekday not found in weekdays (weekday=%s hour=%s minute=%s, tz=%s)", weekday, hour, minute, tz)
	}

	h, err := strconv.Atoi(hour)
	if err != nil {
		return WeekdayTime{}, fmt.Errorf("WeekdayTimeLocal error, %w (weekday=%s hour=%s minute=%s, tz=%s)", err, weekday, hour, minute, tz)
	}

	m, err := strconv.Atoi(minute)
	if err != nil {
		return WeekdayTime{}, fmt.Errorf("WeekdayTimeLocal error, %w (weekday=%s hour=%s minute=%s, tz=%s)", err, weekday, hour, minute, tz)
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return WeekdayTime{}, fmt.Errorf("WeekdayTimeLocal error, %w (weekday=%s hour=%s minute=%s, tz=%s)", err, weekday, hour, minute, tz)
	}

	now := time.Now().UTC()
	nowWd := util.IndexOfString(strings.ToUpper(now.Weekday().String()[:3]), weekdays)

	if wd > nowWd {
		diff := (wd - nowWd)
		days := (time.Duration(diff) * 24) * time.Hour
		now = now.Add(days)
	}

	if wd < nowWd {
		diff := (wd - nowWd)
		days := (time.Duration(diff) * 24) * time.Hour
		now = now.Add(days)
	}

	t := time.Date(1980, 01, now.Day(), h, m, 0, 0, loc)

	return WeekdayTime{
		ts: &t,
	}, nil
}

func (wt WeekdayTime) InRange(from, to WeekdayTime) bool {
	if wt.OnOrAfter(from) && wt.OnOrBefore(to) {
		return true
	}

	return false
}

func (wt WeekdayTime) OnOrAfter(val WeekdayTime) bool {
	after := wt.ts.After(*val.ts)
	exact := wt.ts.Equal(*val.ts)
	return exact || after
}

func (wt WeekdayTime) OnOrBefore(val WeekdayTime) bool {
	before := wt.ts.Before(*val.ts)
	exact := wt.ts.Equal(*val.ts)
	return exact || before
}

func (wt WeekdayTime) String() string {
	return fmt.Sprintf("%s %02d:%02d %s", wt.ts.Weekday(), wt.ts.Hour(), wt.ts.Minute(), wt.ts.Location())
}
