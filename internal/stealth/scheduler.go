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
			{StartHour: 12, EndHour: 13}, // Lunch: 12-1 PM
			{StartHour: 10, EndHour: 10}, // Morning break: 10-10:15 AM
			{StartHour: 15, EndHour: 15}, // Afternoon break: 3-3:15 PM
		},
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (s *Scheduler) ShouldOperate() bool {
	now := time.Now().In(s.timezone)

	// Check if it's a workday
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

	// Check if within business hours
	hour := now.Hour()
	if hour < s.startHour || hour >= s.endHour {
		return false
	}

	// Check if during a break
	for _, breakTime := range s.breaks {
		if hour == breakTime.StartHour {
			minute := now.Minute()
			if breakTime.StartHour == breakTime.EndHour {
				// Short break (e.g., 10:00-10:15)
				if minute < 15 {
					return false
				}
			} else {
				// Long break (e.g., 12:00-13:00)
				return false
			}
		}
	}

	return true
}

func (s *Scheduler) WaitUntilOperatingHours() time.Duration {
	for !s.ShouldOperate() {
		now := time.Now().In(s.timezone)

		// Calculate time until next operating period
		if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
			// Wait until Monday
			daysUntilMonday := (8 - int(now.Weekday())) % 7
			if daysUntilMonday == 0 {
				daysUntilMonday = 1
			}

			nextStart := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday,
				s.startHour, 0, 0, 0, s.timezone)
			waitDuration := time.Until(nextStart)

			if waitDuration > 15*time.Minute {
				time.Sleep(15 * time.Minute) // Check every 15 minutes
			} else {
				time.Sleep(waitDuration)
			}
			continue
		}

		// Wait until start of business hours
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

		// After business hours, wait until next day
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

		// During a break
		time.Sleep(5 * time.Minute)
	}

	return 0
}

func (s *Scheduler) GetRandomWorkDuration() time.Duration {
	// Random work session: 30-90 minutes
	minutes := 30 + s.rand.Intn(60)
	return time.Duration(minutes) * time.Minute
}

func (s *Scheduler) GetBreakDuration() time.Duration {
	// Short break: 5-10 minutes
	minutes := 5 + s.rand.Intn(5)
	return time.Duration(minutes) * time.Minute
}
