package stealth

import (
	"math/rand"
	"time"
)

type Scheduler struct {
	timezone  *time.Location
	startHour int
	endHour   int
	workDays  []time.Weekday
	breaks    []TimeRange
	rand      *rand.Rand
}

type TimeRange struct {
	StartHour int
	EndHour   int
}

func NewScheduler(tz string, start, end int, workDays []time.Weekday) (*Scheduler, error) {
	location, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		timezone:  location,
		startHour: start,
		endHour:   end,
		workDays:  workDays,
		breaks: []TimeRange{
			{StartHour: 12, EndHour: 13},
			{StartHour: 10, EndHour: 10},
			{StartHour: 15, EndHour: 15},
		},
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (s *Scheduler) ShouldOperate() bool {
	now := time.Now().In(s.timezone)

	isWorkday := false
	for _, day := range s.workDays {
		if now.Weekday() == day {
			isWorkday = true
			break
		}
	}

	if !isWorkday {
		return false
	}

	hour := now.Hour()
	if hour < s.startHour || hour >= s.endHour {
		return false
	}

	for _, breakTime := range s.breaks {
		if hour == breakTime.StartHour {
			minute := now.Minute()
			if breakTime.StartHour == breakTime.EndHour {
				if minute < 15 {
					return false
				}
			} else {
				return false
			}
		}
	}

	return true
}

func (s *Scheduler) WaitUntilOperatingHours() time.Duration {
	for !s.ShouldOperate() {
		now := time.Now().In(s.timezone)

		if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
			daysUntilMonday := (8 - int(now.Weekday())) % 7
			if daysUntilMonday == 0 {
				daysUntilMonday = 1
			}

			nextStart := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday,
				s.startHour, 0, 0, 0, s.timezone)
			waitDuration := time.Until(nextStart)

			if waitDuration > 15*time.Minute {
				time.Sleep(15 * time.Minute)
			} else {
				time.Sleep(waitDuration)
			}
			continue
		}

		if now.Hour() < s.startHour {
			nextStart := time.Date(now.Year(), now.Month(), now.Day(),
				s.startHour, 0, 0, 0, s.timezone)
			waitDuration := time.Until(nextStart)

			if waitDuration > 15*time.Minute {
				time.Sleep(15 * time.Minute)
			} else {
				time.Sleep(waitDuration)
			}
			continue
		}

		if now.Hour() >= s.endHour {
			tomorrow := now.Add(24 * time.Hour)
			nextStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
				s.startHour, 0, 0, 0, s.timezone)

			if time.Until(nextStart) > 15*time.Minute {
				time.Sleep(15 * time.Minute)
			} else {
				time.Sleep(time.Until(nextStart))
			}
			continue
		}

		time.Sleep(5 * time.Minute)
	}

	return 0
}

func (s *Scheduler) GetRandomWorkDuration() time.Duration {
	minutes := 30 + s.rand.Intn(60)
	return time.Duration(minutes) * time.Minute
}

func (s *Scheduler) GetBreakDuration() time.Duration {
	minutes := 5 + s.rand.Intn(5)
	return time.Duration(minutes) * time.Minute
}
