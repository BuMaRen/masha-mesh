package controller

import "sync"

type Publisher struct {
	mtx *sync.Mutex
}

func NewPublisher() *Publisher {
	return &Publisher{
		mtx: &sync.Mutex{},
	}
}

func (p *Publisher) Update() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
}
