// Copyright 2016 GUO Yixuan <culy.gyx@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resource_mgr

import (
	"regexp"
	"strings"
	"time"
)

var grooveFilter = regexp.MustCompile("LIVE Groove")

// event_type
const (
	EventAtapon  = 1
	EventCaravan = 2
	// The internal name for groove is medley
	EventGroove  = 3
	EventParty   = 4
	EventTour    = 5
	EventRail    = 6
)

type EventDetail struct {
	// Columns of table event_data (incomplete)
	id, typ                                   int
	name                                      string
	notice_start                              time.Time
	event_start, second_half_start, event_end time.Time
	calc_start, result_start, result_end      time.Time
	limit_flag, bg_type, bg_id                int
	login_bonus_type, login_bonus_count       int
	master_plus_support                       int
	// extra, for medley event
	music_name string
}

func (e *EventDetail) Type() int {
	return e.typ
}

func (e *EventDetail) Name() string {
	return e.name
}

func (e *EventDetail) ShortName() string {
	// for groove/parade
	if e.typ == EventGroove || e.typ == EventTour {
		return e.MusicName()
	}
	// Usually names like "LIVE Groove Vocal Burst" is too long to fit in a tweet,
	// so a shortened form is preferred.
	// Update: now the name of the event song is used instead in tweets, so this
	// branch should be unreachable.
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
		return t.AddDate(ynow-t.Year(), 0, 0)
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
	if (e.typ == EventAtapon) || (e.typ == EventGroove) || (e.typ == EventTour) {
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

// in competition period
func (e *EventDetail) IsActive(t time.Time) bool {
	if !t.Before(e.event_start) && !t.After(e.event_end) {
		return true
	} else {
		return false
	}
}

// in full event period
func (e *EventDetail) InPeriod(t time.Time) bool {
	if !t.Before(e.event_start) && !t.After(e.result_end) {
		return true
	} else {
		return false
	}
}

// in calculating period
func (e *EventDetail) IsCalc(t time.Time) bool {
	if t.After(e.calc_start) && t.Before(e.result_start) {
		return true
	} else {
		return false
	}
}

// in result publishing period
func (e *EventDetail) IsFinal(t time.Time) bool {
	if !t.Before(e.result_start) && !t.After(e.result_end) {
		return true
	} else {
		return false
	}
}

// only current event in period
func FindCurrentEvent(eventList []*EventDetail) *EventDetail {
	now := time.Now()
	for _, e := range eventList {
		if e.InPeriod(now) {
			return e
		}
	}
	return nil
}

// the last event that has started and has ranking
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

// An interface implementation for sort.Sort to work on []*EventDetail
type EventDetailList []*EventDetail

func (l EventDetailList) Len() int {
	return len(l)
}

// Sort according to EventStart(), assuming no events share the same start date.
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

// from groove event id to music title
// event id 3012 => あいくるしい
// story category id 272
// music id 5029
// live data 317, 527

// event id 3011 =>
// music id 9009
// live id 313 523

// live_data table
// id, music_data_id, sort = event_id

// new workflow
// event id
// => medley_story_detail  *tour_story_detail   "event_id -> id"
// story id
// => story detail   "id -> category_id"
// story category id
// => story_category "id -> title"
// title

// Use a single SQL query with JOIN on 3 tables, see resource_mgr.go

// For live groove and live parade: title of the song.
func (e *EventDetail) MusicName() string {
	if e.typ != EventGroove && e.typ != EventTour {
		return e.name
	}

	// Special cases for early live groove events without story commu.
	if e.id == 3001 {
		return "夢色ハーモニー"
	}
	if e.id == 3002 {
		return "流れ星キセキ"
	}
	if e.id == 3005 {
		return "Absolute NIne"
	}
	if e.music_name != "" {
		return strings.Replace(e.music_name, "\\n", "", -1)
	} else {
		return e.name
	}
}

// For live groove and live parade events, return a name containing both
// event type and song name.
func (e *EventDetail) LongName() string {
	long := e.name
	if e.typ == EventGroove || e.typ == EventTour {
		long += " = " + e.MusicName()
	}
	return long
}
