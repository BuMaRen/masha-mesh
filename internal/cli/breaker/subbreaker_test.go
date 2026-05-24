package breaker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestOptions() *Options {
	return &Options{
		windowCapacity:       1,
		halfOpenAllowed:      2,
		minRequestCount:      1,
		failureRateThreshold: 0.5,
		halfOpenMaxDuration:  3,
		openDuration:         2,
	}
}

func TestSubBreaker_FromClosed(t *testing.T) {
	t.Run("达到失败阈值后开路", func(t *testing.T) {
		sb := newSubBreaker(newTestOptions())
		base := time.Now()
		sb.lastTick = base

		sb.record(base, TIMEOUT)
		if allowed := sb.allowed(base.Add(1 * time.Second)); allowed {
			t.Fatalf("expected request to be rejected when breaker opens")
		}
		if sb.state != StateOpen {
			t.Fatalf("expected state open, got %v", sb.state)
		}
	})
}

func TestSubBreaker_FromOpen(t *testing.T) {
	t.Run("开路状态结束前拒绝其他请求", func(t *testing.T) {
		sb := newSubBreaker(newTestOptions())
		base := time.Now()
		sb.lastTick = base
		sb.state = StateOpen
		sb.openUntil = base.Add(sb.settings.openDuration)

		if allowed := sb.allowed(base.Add(1 * time.Second)); allowed {
			t.Fatalf("expected request to be rejected before open duration expires")
		}
		if sb.state != StateOpen {
			t.Fatalf("expected state open, got %v", sb.state)
		}
	})

	t.Run("开路状态结束后转为半开状态", func(t *testing.T) {
		sb := newSubBreaker(newTestOptions())
		base := time.Now()
		sb.lastTick = base
		sb.state = StateOpen
		sb.openUntil = base.Add(sb.settings.openDuration)

		if allowed := sb.allowed(base.Add(3 * time.Second)); !allowed {
			t.Fatalf("expected request to be allowed when transitioning to half-open")
		}
		if sb.state != StateHalfOpen {
			t.Fatalf("expected state half-open, got %v", sb.state)
		}
		if sb.probeUsed != 1 {
			t.Fatalf("expected probeUsed=1 after first half-open allow, got %d", sb.probeUsed)
		}
	})
}

func TestSubBreaker_FromHalfOpen(t *testing.T) {
	t.Run("半开状态在探测成功后转为闭合状态", func(t *testing.T) {
		sb := newSubBreaker(newTestOptions())
		base := time.Now()
		sb.lastTick = base
		sb.state = StateHalfOpen
		sb.halfOpenSince = base

		if allowed := sb.allowed(base); !allowed {
			t.Fatalf("expected first probe request to be allowed")
		}
		sb.record(base, SUCCESS)

		if allowed := sb.allowed(base.Add(200 * time.Millisecond)); !allowed {
			t.Fatalf("expected second probe request to be allowed")
		}
		sb.record(base.Add(200*time.Millisecond), SUCCESS)

		if sb.state != StateClosed {
			t.Fatalf("expected state closed after successful probes, got %v", sb.state)
		}
	})

	t.Run("半开状态在探测失败后转为开路状态", func(t *testing.T) {
		sb := newSubBreaker(newTestOptions())
		base := time.Now()
		sb.lastTick = base
		sb.state = StateHalfOpen
		sb.halfOpenSince = base

		if allowed := sb.allowed(base); !allowed {
			t.Fatalf("expected probe request to be allowed")
		}
		sb.record(base, TIMEOUT)

		if sb.state != StateOpen {
			t.Fatalf("expected state open after half-open probe failure, got %v", sb.state)
		}
		if !sb.openUntil.Equal(base.Add(sb.settings.openDuration)) {
			t.Fatalf("expected openUntil to be updated after failure")
		}
	})
}

func TestSubBreaker_Concurrency(t *testing.T) {
	t.Run("半开状态并发放行不超过探测配额", func(t *testing.T) {
		sb := newSubBreaker(newTestOptions())
		base := time.Now()
		sb.lastTick = base
		sb.state = StateHalfOpen
		sb.halfOpenSince = base

		var allowedCount int32
		var wg sync.WaitGroup
		const goroutines = 64

		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				if sb.allowed(base) {
					atomic.AddInt32(&allowedCount, 1)
				}
			}()
		}
		wg.Wait()

		if int(allowedCount) > sb.settings.halfOpenAllowed {
			t.Fatalf("allowed probes exceeded quota: got=%d quota=%d", allowedCount, sb.settings.halfOpenAllowed)
		}
		if sb.probeUsed != int(allowedCount) {
			t.Fatalf("probeUsed should equal allowed probe count: probeUsed=%d allowed=%d", sb.probeUsed, allowedCount)
		}
	})

	t.Run("并发allowed与record可完成且不死锁", func(t *testing.T) {
		sb := newSubBreaker(newTestOptions())
		base := time.Now()
		sb.lastTick = base

		const workers = 16
		const loops = 100

		done := make(chan struct{})
		go func() {
			var wg sync.WaitGroup
			wg.Add(workers)
			for w := 0; w < workers; w++ {
				go func(offset int) {
					defer wg.Done()
					for i := 0; i < loops; i++ {
						now := base.Add(time.Duration((offset+i)%3) * time.Second)
						if sb.allowed(now) {
							if i%5 == 0 {
								sb.record(now, TIMEOUT)
							} else {
								sb.record(now, SUCCESS)
							}
						}
					}
				}(w)
			}
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// pass
		case <-time.After(3 * time.Second):
			t.Fatalf("concurrent allowed/record timed out, possible deadlock")
		}

		sb.mtx.Lock()
		defer sb.mtx.Unlock()
		if sb.state != StateClosed && sb.state != StateOpen && sb.state != StateHalfOpen {
			t.Fatalf("invalid breaker state after concurrency test: %v", sb.state)
		}
	})
}
