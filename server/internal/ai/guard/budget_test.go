package guard

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBudgetReservation(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := NewBudget(1, 0)
	settle, err := b.Reserve("a", .8, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Reserve("b", .3, now); err == nil {
		t.Fatal("并发预留未受限制")
	}
	settle(.2)
	settle(0)
	if err := b.Charge("b", .7, now); err != nil {
		t.Fatal(err)
	}
	b.Limit = func() float64 { return .5 }
	if err := b.Charge("c", .01, now); err == nil {
		t.Fatal("修改上限未生效")
	}
	if err := b.Charge("c", .4, now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
}
func TestBudgetConcurrent(t *testing.T) {
	b := NewBudget(1, 0)
	var accepted atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := b.Reserve("s", .2, time.Now()); err == nil {
				accepted.Add(1)
			}
		}()
	}
	group.Wait()
	if accepted.Load() != 5 {
		t.Fatalf("预留次数：%d", accepted.Load())
	}
}

func TestBudgetLoadsPersistedSpend(t *testing.T) {
	b := NewBudget(1, 0)
	loads := 0
	b.Load = func(time.Time) (float64, error) { loads++; return .9, nil }
	if _, err := b.Reserve("session", .2, time.Now()); err == nil {
		t.Fatal("重启后已用预算被清空")
	}
	if _, err := b.Reserve("session", .05, time.Now()); err != nil {
		t.Fatal(err)
	}
	if loads != 1 {
		t.Fatal("重复加载导致重复计费")
	}
}
