package metrics

import (
	"fmt"
	"github.com/ipfs/go-cid"
	"sync"
	"time"
)

type FindProviderMonitor struct {
	EventList sync.Map
}

/*
	event happens during find providers for a cid
*/

type ProviderEvent struct {
	//lock *sync.RWMutex
	c    cid.Cid
	mh   string
	self string
	//start time point
	FindProviderAsync time.Time
	//time finish searching cache and datastore for provider to see if we already know the provider
	FinishLocalSearch time.Time

	//structure of dht search tree
	FirstGotProviderFrom sync.Map // provider,peer
	FirstGotCloserFrom   sync.Map // closer, peer

	//time point in dht search tree
	FirstRequestTime  sync.Map // peerID,time
	FirstResponseTime sync.Map //peerID, time
}

func NewFPMonitor() *FindProviderMonitor {
	var fpm FindProviderMonitor
	return &fpm
}

func (m *FindProviderMonitor) NewProviderEvent(cid cid.Cid, multihash string, selfID string) {
	fmt.Printf("%s NewProviderEvent %s %s\n", time.Now(), cid, multihash)
	var pe ProviderEvent
	pe.c = cid
	pe.mh = multihash
	pe.FindProviderAsync = time.Now()
	pe.FinishLocalSearch = ZeroTime
	pe.self = selfID
	//pe.lock = new(sync.RWMutex)

	m.EventList.Store(pe.mh, &pe)
}
func (m *FindProviderMonitor) PMGotProviders(mh string) {
	fmt.Printf("%s PMGotProviders %s\n", time.Now(), mh)
	if v, ok := m.EventList.Load(mh); ok {
		pe := v.(*ProviderEvent)
		pe.FinishLocalSearch = time.Now()
		m.EventList.Store(pe.mh, pe)
	}
}

func (m *FindProviderMonitor) GotProviderFrom(target string, provider string, from string) {
	fmt.Printf("%s GotProviderFrom %s %s %s\n", time.Now(), target, provider, from)

	if v, ok := m.EventList.Load(target); ok {
		pe := v.(*ProviderEvent)
		if _, ok := pe.FirstGotProviderFrom.Load(provider); !ok {
			pe.FirstGotProviderFrom.Store(provider, from)
			m.EventList.Store(target, pe)
		}
	}
}
func (m *FindProviderMonitor) GotCloserFrom(target string, closers []string, from string) {
	for _, c := range closers {
		fmt.Printf("%s GotCloserFrom %s  %s\n", time.Now(), c, from)

		if v, ok := m.EventList.Load(target); ok {
			pe := v.(*ProviderEvent)
			//pe.lock.RLock()
			p, ok := pe.FirstGotCloserFrom.Load(c)
			//pe.lock.RUnlock()
			if !ok {
				//pe.lock.Lock()
				pe.FirstGotCloserFrom.Store(c, from)
				//pe.lock.Unlock()
			} else {
				fmt.Printf("closer already have parent %s\n", p.(string))
			}

			m.EventList.Store(target, pe)
		}
	}

}

func (m *FindProviderMonitor) SendNodeWant(target string, peer string) {
	fmt.Printf("%s SendNodeWant %s %s\n", time.Now(), target, peer)
	if v, ok := m.EventList.Load(target); ok {
		pe := v.(*ProviderEvent)
		needWB := false
		if _, ok := pe.FirstRequestTime.Load(peer); !ok {
			needWB = true
			pe.FirstRequestTime.Store(peer, time.Now())
		}
		if needWB {
			m.EventList.Store(target, pe)
		}
	}
}

func (m *FindProviderMonitor) ReceiveResult(target string, peer string) {
	fmt.Printf("%s ReceiveResult %s %s\n", time.Now(), target, peer)

	if v, ok := m.EventList.Load(target); ok {
		pe := v.(*ProviderEvent)
		if _, ok := pe.FirstResponseTime.Load(peer); !ok {
			pe.FirstResponseTime.Store(peer, time.Now())
			m.EventList.Store(target, pe)
		}
	}
}

func (m *FindProviderMonitor) CriticalPath() {
	m.EventList.Range(func(key, value interface{}) bool {
		pe := value.(*ProviderEvent)
		fmt.Printf("CID: %s\n", pe.c)
		pe.FirstGotProviderFrom.Range(func(key, value interface{}) bool {
			fmt.Printf("Providers: %s\n", key.(string))
			father := value.(string)
			for true {
				fmt.Printf("%s\n", father)
				if newf, ok := pe.FirstGotCloserFrom.Load(father); ok {
					father = newf.(string)
				} else {
					return false
				}
			}
			return true
		})
		return true
	})
}
