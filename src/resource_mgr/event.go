package resource_mgr

import (
	"regexp"
	"time"
)

var grooveFilter = regexp.MustCompile("LIVE Groove")

type EventDetail struct {
	id, typ                                   int
	name                                      string
	notice_start                              time.Time
	event_start, second_half_start, event_end time.Time
	calc_start, result_start, result_end      time.Time
	limit_flag, bg_type, bg_id                int
	login_bonus_type, login_bonus_count       int
}

func (e *EventDetail) Type() int {
	return e.typ
}

func (e *EventDetail) Name() string {
	return e.name
}

func (e *EventDetail) ShortName() string {
	if grooveFilter.MatchString(e.name) {
		return "LIVE Groove"
	} else {
		return e.name
	}
}

func (e *EventDetail) Id() int {
	return e.id
}

// time-related
func demask(t time.Time) time.Time {
	if t.Year() < 2099 {
		return t
	} else {
		ynow := time.Now().Year()
		return t.AddDate(ynow - t.Year(), 0, 0)
	}
}

func (e *EventDetail) NoticeStart() time.Time {
	return demask(e.notice_start)
}

func (e *EventDetail) EventStart() time.Time {
	return demask(e.event_start)
}

func (e *EventDetail) SecondHalfStart() time.Time {
	return demask(e.second_half_start)
}

func (e *EventDetail) EventEnd() time.Time {
	return demask(e.event_end)
}

func (e *EventDetail) CalcStart() time.Time {
	return demask(e.calc_start)
}

func (e *EventDetail) ResultStart() time.Time {
	return demask(e.result_start)
}

func (e *EventDetail) ResultEnd() time.Time {
	return demask(e.result_end)
}

func (e *EventDetail) LoginBonusType() int {
	return e.login_bonus_type
}

func (e *EventDetail) LoginBonusCount() int {
	return e.login_bonus_count
}

func (e *EventDetail) HasRanking() bool {
	if (e.typ == 1) || (e.typ == 3) || (e.typ == 5) {
		return true
	} else {
		return false
	}
}

func (e *EventDetail) RankingAvailable() bool {
	now := time.Now()
	// now is in [EventStart, EventEnd]
	//        or [ResultStart, ResultEnd]
	// allow 20min at end?
	if !now.Before(e.event_start) && !now.After(e.event_end) {
		return true
	} else if !now.Before(e.result_start) && !now.After(e.result_end) {
		return true
	} else {
		return false
	}
}

func (e *EventDetail) IsCalc(t time.Time) bool {
	if t.After(e.calc_start) && t.Before(e.result_start) {
		return true
	} else {
		return false
	}
}

func (e *EventDetail) IsFinal(t time.Time) bool {
	if !t.Before(e.result_start) && !t.After(e.result_end) {
		return true
	} else {
		return false
	}
}

func FindCurrentEvent(eventList []*EventDetail) *EventDetail {
	now := time.Now()
	for _, e := range eventList {
		if !now.Before(e.event_start) && !now.After(e.result_end) {
			return e
		}
	}
	return nil
}

func FindLatestEvent(eventList []*EventDetail) *EventDetail {
	now := time.Now()
	for i := len(eventList) - 1; i >= 0; i-- {
		e := eventList[i]
		if !now.Before(e.event_start) && e.HasRanking() {
			return e
		}
	}
	return nil
}

type EventDetailList []*EventDetail

func (l EventDetailList) Len() int {
	return len(l)
}

func (l EventDetailList) Less(i, j int) bool {
	return l[i].EventStart().Before(l[j].EventStart())
}

func (l EventDetailList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (eventList *EventDetailList) FindEventById(id int) *EventDetail {
	for _, e := range *eventList {
		if e.Id() == id {
			return e
		}
	}
	return nil
}

func (eventList *EventDetailList) Overwrite(e_new *EventDetail) {
	for index, e := range *eventList {
		if e.Id() == e_new.Id() {
			(*eventList)[index] = e_new
		}
	}
}
