package breaker

import (
	"sync"
	"time"
)

type State uint

var (
	StateClosed   State = 0
	StateOpen     State = 1
	StateHalfOpen State = 2
)

type settings struct {
	capacity             int           // 窗口容量，单位为请求数
	halfOpenAllowed      int           // 半开状态允许的请求数，超过后根据失败率决定是否切换到开状态
	minRequestCount      int           // 最小请求数，未达到该数值时不考虑切换到开状态
	failureRateThreshold float64       // 失败率阈值，超过该值时切换到开状态
	interval             time.Duration // 统计窗口的时间间隔，单位为秒
	halfOpenMaxDuration  time.Duration // 半开状态的最大持续时间，超过该时间后切换到开状态
	openDuration         time.Duration // 开状态的持续时间，超过该时间后切换到半开状态
}

func newSettings(opts *Options) *settings {
	return &settings{
		halfOpenAllowed:      opts.halfOpenAllowed,
		minRequestCount:      opts.minRequestCount,
		failureRateThreshold: opts.failureRateThreshold,
		interval:             time.Duration(opts.windowCapacity) * time.Second,
		halfOpenMaxDuration:  time.Duration(opts.halfOpenMaxDuration) * time.Second,
		openDuration:         time.Duration(opts.openDuration) * time.Second,
	}
}

type subBreaker struct {
	mtx           *sync.Mutex
	state         State     // 当前状态
	openUntil     time.Time // open状态持续到的时间点，超过该时间点后切换到半开状态
	halfOpenSince time.Time // 半开状态开始的时间点，用于计算半开状态持续的时间
	probeUsed     int       // 半开状态已探测的请求数
	probeSuccess  int       // 半开状态已探测的成功请求数
	lastTick      time.Time // 上次状态更新的时间点
	settings      *settings

	windowCounter *counter
}

func newSubBreaker(opts *Options) *subBreaker {
	return &subBreaker{
		mtx:           &sync.Mutex{},
		state:         StateClosed,
		openUntil:     time.Time{},
		halfOpenSince: time.Time{},
		probeUsed:     0,
		probeSuccess:  0,
		lastTick:      time.Now(),
		settings:      newSettings(opts),
		windowCounter: NewCounter(opts.windowCapacity),
	}
}

// 时间推进：根据 now 与 lastTick 的差值补齐缺失时间片，对应创建空桶并淘汰过期桶。
// 统计对齐：保证后续读取到的窗口统计始终表示“最近 windowDuration 的真实时间”。
// 过期清理：当时间跨度大于等于 bucketCount 时，直接清空整窗，避免历史样本污染当前判断。
// 单一时间基准：为 Allowed 与 Record 提供同一时间基准，避免决策与记账使用不同窗口视图。
func (s *subBreaker) advance(now time.Time) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	sub := now.Sub(s.lastTick)
	// 如果距离上次更新时间不足一个统计窗口周期，则无需推进
	if sub < s.settings.interval {
		return
	}
	// 计算需要推进的时间片数量
	bucketCount := int(sub / s.settings.interval)
	for i := 0; i < bucketCount-1; i++ {
		s.windowCounter.PushEmpty()
	}
	s.windowCounter.Flush()
	s.lastTick = now
}

func (s *subBreaker) allowed(now time.Time) bool {
	s.advance(now)

	s.mtx.Lock()
	defer s.mtx.Unlock()

	switch s.state {
	case StateClosed:
		summary := s.windowCounter.Summary()
		if summary.Total < s.settings.minRequestCount {
			return true
		}
		failureRate := float64(summary.NetworkFailure+summary.Timeout) / float64(summary.Total)
		if failureRate > s.settings.failureRateThreshold {
			s.state = StateOpen
			s.openUntil = now.Add(s.settings.openDuration)
			return false
		}
	case StateOpen:
		if !now.After(s.openUntil) {
			return false
		}
		s.state = StateHalfOpen
		s.halfOpenSince = now
		s.probeUsed = 0
		s.probeSuccess = 0
		fallthrough
	case StateHalfOpen:
		// 半开状态超时后开路
		if now.Sub(s.halfOpenSince) > s.settings.halfOpenMaxDuration {
			s.state = StateOpen
			s.openUntil = now.Add(s.settings.openDuration)
			return false
		}
		// 请求数量超过半开状态允许的请求数后，根据失败率决定是否切换到开状态
		if s.probeUsed >= s.settings.halfOpenAllowed {
			return false
		}
		// record 请求返回后才调用，不能在 record 里做累加
		s.probeUsed++
	}
	return true
}

func (s *subBreaker) record(now time.Time, result Metric) {
	s.advance(now)

	s.mtx.Lock()
	defer s.mtx.Unlock()

	switch result {
	case SUCCESS:
		s.windowCounter.RecordSuccess()
	case BUSINESS_FAILURE:
		s.windowCounter.RecordBusinessFailure()
	case NETWORK_FAILURE:
		s.windowCounter.RecordNetworkFailure()
	case TIMEOUT:
		s.windowCounter.RecordTimeout()
	}

	if s.state == StateHalfOpen {
		switch result {
		case NETWORK_FAILURE, TIMEOUT:
			s.state = StateOpen
			s.openUntil = now.Add(s.settings.openDuration)
		case SUCCESS:
			s.probeSuccess++
		}
		if s.probeSuccess >= s.settings.halfOpenAllowed {
			s.state = StateClosed
			s.windowCounter.Reset()
		}
	}
}
