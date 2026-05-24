package breaker

import (
	"sync"

	"github.com/BuMaRen/mesh/pkg/datastruct/queue"
)

type Metric uint

var (
	SUCCESS          Metric = 0
	BUSINESS_FAILURE Metric = 1
	NETWORK_FAILURE  Metric = 2
	TIMEOUT          Metric = 3
)

type StatisticalUnit struct {
	Total           int // 总请求数
	Success         int // 成功请求数
	Timeout         int // 超时请求数
	BusinessFailure int // 业务失败请求数
	NetworkFailure  int // 网络失败请求数
}

type counter struct {
	mtx     *sync.Mutex
	current *StatisticalUnit
	units   *queue.CircularQueue[*StatisticalUnit]
}

func NewCounter(capacity int) *counter {
	return &counter{
		units:   queue.NewCircularQueue[*StatisticalUnit](capacity),
		current: &StatisticalUnit{},
		mtx:     &sync.Mutex{},
	}
}

func (c *counter) Reset() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.current = &StatisticalUnit{}
	c.units.Reset()
}

func (c *counter) Flush() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.units.Enqueue(c.current)
	c.current = &StatisticalUnit{}
}

func (c *counter) PushEmpty() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.units.Enqueue(&StatisticalUnit{})
}

func (c *counter) Summary() *StatisticalUnit {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	summary := &StatisticalUnit{}
	units := c.units.List()
	for _, unit := range units {
		summary.Total += unit.Total
		summary.Success += unit.Success
		summary.BusinessFailure += unit.BusinessFailure
		summary.NetworkFailure += unit.NetworkFailure
		summary.Timeout += unit.Timeout
	}
	summary.Total += c.current.Total
	summary.Success += c.current.Success
	summary.BusinessFailure += c.current.BusinessFailure
	summary.NetworkFailure += c.current.NetworkFailure
	summary.Timeout += c.current.Timeout
	return summary
}

func (c *counter) RecordSuccess() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.current.Success++
	c.current.Total++
}

func (c *counter) RecordBusinessFailure() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.current.BusinessFailure++
	c.current.Total++
}

func (c *counter) RecordNetworkFailure() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.current.NetworkFailure++
	c.current.Total++
}

func (c *counter) RecordTimeout() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.current.Timeout++
	c.current.Total++
}
