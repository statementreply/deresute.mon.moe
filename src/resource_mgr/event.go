package resource_mgr

import (
	"time"
)

type EventDetail struct {
	id, typ                                                                                       int
	name                                                                                          string
	notice_start, event_start, second_half_start, event_end, calc_start, result_start, result_end time.Time
	limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count                               int
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

func FindCurrentEvent(eventList []*EventDetail) *EventDetail {
	now := time.Now()
	for _, e := range eventList {
		if !now.Before(e.EventStart()) && !now.After(e.ResultEnd()) {
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

func FindEventById(eventList EventDetailList, id int) *EventDetail {
	for _, e := range eventList {
		if e.Id() == id {
			return e
		}
	}
	return nil
}
