package guard

import (
	"errors"
	"math"
	"sync"
	"time"
)

var ErrBudgetExceeded = errors.New("问答花费已达到限制")

type spend struct {
	amount float64
	reset  time.Time
}
type Budget struct {
	mu            sync.Mutex
	daily, hourly float64
	values        map[string]spend
	// Limit 每次预留读取当前设置，修改上限无需重启。
	Limit       func() float64
	HourlyLimit func() float64
	Load        func(time.Time) (float64, error)
}

func NewBudget(daily, hourly float64) *Budget {
	return &Budget{daily: daily, hourly: hourly, values: map[string]spend{}}
}

// Reserve 在调用前原子预留费用；完成时按真实价格结算，未知用量保留预估费用。
func (b *Budget) Reserve(scope string, amount float64, now time.Time) (func(float64), error) {
	if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, errors.New("花费无效")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	daily := b.daily
	hourly := b.hourly
	if b.Limit != nil {
		daily = b.Limit()
	}
	if b.HourlyLimit != nil {
		hourly = b.HourlyLimit()
	}
	dayKey := "day:" + now.UTC().Format("2006-01-02")
	hourKey := "hour:" + scope + ":" + now.UTC().Format("2006-01-02T15")
	for key, v := range b.values {
		if !now.Before(v.reset) {
			delete(b.values, key)
		}
	}
	day, exists := b.values[dayKey]
	if !exists && b.Load != nil {
		amount, err := b.Load(now)
		if err != nil {
			return nil, err
		}
		day = spend{amount: amount, reset: now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)}
		b.values[dayKey] = day
	}
	hour := b.values[hourKey]
	if daily > 0 && day.amount+amount > daily || hourly > 0 && hour.amount+amount > hourly {
		return nil, ErrBudgetExceeded
	}
	day.amount += amount
	day.reset = now.Add(24 * time.Hour)
	hour.amount += amount
	hour.reset = now.Add(time.Hour)
	b.values[dayKey], b.values[hourKey] = day, hour
	var once sync.Once
	return func(actual float64) {
		once.Do(func() {
			if actual < 0 || math.IsNaN(actual) || math.IsInf(actual, 0) {
				return
			}
			b.mu.Lock()
			defer b.mu.Unlock()
			for _, key := range []string{dayKey, hourKey} {
				if v, ok := b.values[key]; ok {
					v.amount += actual - amount
					b.values[key] = v
				}
			}
		})
	}, nil
}
func (b *Budget) Charge(scope string, amount float64, now time.Time) error {
	_, err := b.Reserve(scope, amount, now)
	return err
}
