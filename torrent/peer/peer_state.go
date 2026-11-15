package peer

import "sync"

type PeerState struct {
	mu       sync.Mutex
	unchoked bool
	cond     *sync.Cond
}

func New() *PeerState {
	p := &PeerState{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *PeerState) IsChoked() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return !p.unchoked
}

func (p *PeerState) IsUnchoked() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.unchoked
}

func (p *PeerState) SetChoked() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.unchoked = false
}

func (p *PeerState) SetUnchoked() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.unchoked = true
	p.cond.Broadcast()
}

func (p *PeerState) WaitUnchoked() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for !p.unchoked {
		p.cond.Wait()
	}
}
