package guard

import (
	"errors"
	"sync"
	"time"
)

type spend struct {
	amount float64
	reset  time.Time
}
type Budget struct {
	mu     sync.Mutex
	daily  float64
	hourly float64
	values map[string]spend
}

func NewBudget(daily, hourly float64) *Budget {
	return &Budget{daily: daily, hourly: hourly, values: map[string]spend{}}
}
func (b *Budget) Charge(scope string, amount float64, now time.Time) error {
	if amount < 0 {
		return errors.New("花费不能为负数")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	day := b.values["day"]
	if now.After(day.reset) {
		day = spend{reset: now.Add(24 * time.Hour)}
	}
	hour := b.values[scope]
	if now.After(hour.reset) {
		hour = spend{reset: now.Add(time.Hour)}
	}
	if b.daily > 0 && day.amount+amount > b.daily || b.hourly > 0 && hour.amount+amount > b.hourly {
		return errors.New("问答花费已达到限制")
	}
	day.amount += amount
	hour.amount += amount
	b.values["day"] = day
	b.values[scope] = hour
	return nil
}
