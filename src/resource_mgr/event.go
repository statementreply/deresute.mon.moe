package resource_mgr

import (
	"time"
)

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

func (e *EventDetail) Id() int {
	return e.id
}

// not exported: NoticeStart
func (e *EventDetail) EventStart() time.Time {
	return e.event_start
}

func (e *EventDetail) SecondHalfStart() time.Time {
	return e.second_half_start
}

func (e *EventDetail) EventEnd() time.Time {
	return e.event_end
}

func (e *EventDetail) CalcStart() time.Time {
	return e.calc_start
}

func (e *EventDetail) ResultStart() time.Time {
	return e.result_start
}

func (e *EventDetail) ResultEnd() time.Time {
	return e.result_end
}

func (e *EventDetail) LoginBonusType() int {
	return e.login_bonus_type
}

func (e *EventDetail) LoginBonusCount() int {
	return e.login_bonus_count
}

func (e *EventDetail) HasRanking() bool {
	if (e.typ == 1) || (e.typ == 3) {
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

func FindCurrentEvent(eventList []*EventDetail) *EventDetail {
	now := time.Now()
	for _, e := range eventList {
		if !now.Before(e.event_start) && !now.After(e.result_end) {
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
