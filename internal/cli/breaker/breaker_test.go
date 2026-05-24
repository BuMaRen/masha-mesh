package breaker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestBreakerWithKey 直接向 breakers 注入指定 key，绕过 Allowed 内首次创建时的空指针路径。
func newTestBreakerWithKey(key string) (*Breaker, *subBreaker) {
	sb := newSubBreaker(newTestOptions())
	b := &Breaker{
		mtx:  &sync.RWMutex{},
		opts: newTestOptions(),
		breakers: map[string]*subBreaker{
			key: sb,
		},
	}
	return b, sb
}

func TestBreaker_FromClosed(t *testing.T) {
	t.Run("达到失败阈值后开路", func(t *testing.T) {
		const key = "svc"
		b, sb := newTestBreakerWithKey(key)
		base := time.Now()
		sb.lastTick = base

		b.RecordTimeout(key)
		if allowed := b.Allowed(key); allowed {
			t.Fatalf("expected request to be rejected when breaker opens")
		}
		if sb.state != StateOpen {
			t.Fatalf("expected state open, got %v", sb.state)
		}
	})
}

func TestBreaker_FromOpen(t *testing.T) {
	t.Run("开路状态结束前拒绝请求", func(t *testing.T) {
		const key = "svc"
		b, sb := newTestBreakerWithKey(key)
		base := time.Now()
		sb.lastTick = base
		sb.state = StateOpen
		sb.openUntil = base.Add(sb.settings.openDuration)

		// Allowed 内部使用 time.Now()，用临时 sb.allowed 验证同一逻辑
		if allowed := sb.allowed(base.Add(1 * time.Second)); allowed {
			t.Fatalf("expected request to be rejected before open duration expires")
		}
		if sb.state != StateOpen {
			t.Fatalf("expected state open, got %v", sb.state)
		}
		_ = b
	})

	t.Run("开路状态结束后转为半开状态", func(t *testing.T) {
		const key = "svc"
		b, sb := newTestBreakerWithKey(key)
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
		_ = b
	})
}

func TestBreaker_FromHalfOpen(t *testing.T) {
	t.Run("半开状态在探测成功后转为闭合状态", func(t *testing.T) {
		const key = "svc"
		b, sb := newTestBreakerWithKey(key)
		base := time.Now()
		sb.lastTick = base
		sb.state = StateHalfOpen
		sb.halfOpenSince = base

		if !b.Allowed(key) {
			t.Fatalf("expected first probe request to be allowed")
		}
		b.RecordSuccess(key)

		if !b.Allowed(key) {
			t.Fatalf("expected second probe request to be allowed")
		}
		b.RecordSuccess(key)

		if sb.state != StateClosed {
			t.Fatalf("expected state closed after successful probes, got %v", sb.state)
		}
	})

	t.Run("半开状态在探测失败后转为开路状态", func(t *testing.T) {
		const key = "svc"
		b, sb := newTestBreakerWithKey(key)
		base := time.Now()
		sb.lastTick = base
		sb.state = StateHalfOpen
		sb.halfOpenSince = base

		if !b.Allowed(key) {
			t.Fatalf("expected probe request to be allowed")
		}
		b.RecordTimeout(key)

		if sb.state != StateOpen {
			t.Fatalf("expected state open after half-open probe failure, got %v", sb.state)
		}
	})
}

func TestBreaker_Concurrency(t *testing.T) {
	t.Run("半开状态并发放行不超过探测配额", func(t *testing.T) {
		const key = "svc"
		b, sb := newTestBreakerWithKey(key)
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
				if b.Allowed(key) {
					atomic.AddInt32(&allowedCount, 1)
				}
			}()
		}
		wg.Wait()

		if int(allowedCount) > sb.settings.halfOpenAllowed {
			t.Fatalf("allowed probes exceeded quota: got=%d quota=%d", allowedCount, sb.settings.halfOpenAllowed)
		}
	})

	t.Run("并发Allowed与Record可完成且不死锁", func(t *testing.T) {
		const key = "svc"
		b, sb := newTestBreakerWithKey(key)
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
						if b.Allowed(key) {
							if i%5 == 0 {
								b.RecordTimeout(key)
							} else {
								b.RecordSuccess(key)
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
		case <-time.After(3 * time.Second):
			t.Fatalf("concurrent Allowed/Record timed out, possible deadlock")
		}
	})
}
