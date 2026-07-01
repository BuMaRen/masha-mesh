package breaker

import (
	"sync"
	"time"
)

type Breaker struct {
	mtx      *sync.RWMutex
	opts     *Options
	breakers map[string]*subBreaker
}

func NewBreaker(opts *Options) *Breaker {
	return &Breaker{
		mtx:      &sync.RWMutex{},
		opts:     opts,
		breakers: make(map[string]*subBreaker),
	}
}

func (b *Breaker) Allowed(key string) bool {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	sb, ok := b.breakers[key]
	if !ok {
		sb = newSubBreaker(b.opts)
		b.breakers[key] = sb
	}
	return sb.allowed(time.Now())
}

func (b *Breaker) RecordSuccess(key string) {
	b.record(key, SUCCESS)
}

func (b *Breaker) RecordNetworkFailure(key string) {
	b.record(key, NETWORK_FAILURE)
}

func (b *Breaker) RecordTimeout(key string) {
	b.record(key, TIMEOUT)
}

func (b *Breaker) RecordBusinessFailure(key string) {
	b.record(key, BUSINESS_FAILURE)
}

func (b *Breaker) record(key string, result Metric) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	sb, ok := b.breakers[key]
	if !ok {
		sb = newSubBreaker(b.opts)
		b.breakers[key] = sb
	}
	sb.record(time.Now(), result)
}
