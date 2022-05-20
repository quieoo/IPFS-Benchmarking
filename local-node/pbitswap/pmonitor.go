package pbitswap

import (
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
)

type DispatchMonitor struct {
	redundants int
	effects    map[peer.ID]int
}

func NewMonitor() *DispatchMonitor {
	return &DispatchMonitor{
		redundants: 0,
		effects:    make(map[peer.ID]int),
	}
}

func (m *DispatchMonitor) updateRedundant() {
	m.redundants++
}

func (m *DispatchMonitor) updateEffects(p peer.ID, e int) {
	m.effects[p] = e
}

func (m *DispatchMonitor) GetRedundants() int {
	return m.redundants
}
func (m *DispatchMonitor) GetEffectsVariance() float64 {
	total := 0.0
	n := 0.0
	for _, e := range m.effects {
		total += float64(e)
		n++
	}
	average := total / n

	Vai := 0.0
	for _, e := range m.effects {
		Vai += (float64(e) - average) * (float64(e) - average)
	}
	return Vai
}

func (m *DispatchMonitor) collect() {
	fmt.Printf("redundant count %d\n", m.redundants)
	fmt.Printf("Variance :%f\n", m.GetEffectsVariance())
}
