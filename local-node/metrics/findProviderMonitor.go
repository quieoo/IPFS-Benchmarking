package metrics

import (
	"github.com/ipfs/go-cid"
	"sync"
	"time"
)

/*
	FindProviderMonitor keeps track of a list of ProviderEvent, which record events happen during finding providers of a target block.
	One ProviderEvent is added when calls FindProvidersAsync(key). Since bitswap will call this function at certain time intervals until the provider is found, we only add it at the first time.
	(But events from multiple process for one block will also update the same ProvideEvent, since they share same target multi-hash)

	In some cases, a FindProvidersAsync is called, but before it return useful providers the bitswap has finished block request. So we only collect the ProviderEvent which at least got one provider.

	ProviderEvent records:
		FindProviderAsync: time of beginning find providers
		FinishLocalSearch: time of finishing searching cache and datastore for provider, see if we already know the provider
		FirstOutputProviderTime: time of firstly got one provider and return it to bitswap
			//below 3 metrics record the time of event firstly happens, because they all may happen more than one times for a peer
		FirstFindPeerTime: time of firstly find a peer, which actually is the time we begin to add the peer to local routing table
		FirstRequestTime: time of firstly send request to a peer, the request will ask if peer know any providers, otherwise peer will return 20 closer peers it knows
		FirstResponseTime: time of firstly got response from a peer
			//below 2 metrics record the path of the node encountered during the find provider process
		FirstGotProviderFrom: record all providers have been found, and which peer first lead us to it
		FirstGotCloserFrom: record all peers have been found (may not include providers), and which peer first lead us to it

		CPL: record every peer and its common prefix length of target block cid

	collect metrics(for one provider, from view of latency):
		Through FirstGotProviderFrom and FirstGotCloserFrom, we can get a critical path lead us to a provider, which is also the fastest way we can find this provider.

		We divide the procedure from one intermediate peer(a) to next peer(b) into 3 parts:
			DHTChooseTime: FirstRequestTime[a]-FirstFindPeerTime[a]
			DHTResponseTime: FirstResponseTime[a]-FirstRequestTime[a]
			DHTReplaceTime: FirstFindPeerTime[b]-FirstResponseTime[a]

		Let's say we query x intermediate peers until the xth peer return a provider. The total time can be calculated by:
			FinishLocalSearch-FindProviderAsync + x*(avg{DHTChooseTime}+avg{ResponseTime}) + (x-1)*avg{DHTReplaceTime}
		We compare this modeled time with real time: FirstOutputProviderTime-FindProviderAsync
*/
type FindProviderMonitor struct {
	EventList         sync.Map
	FirstFindPeerTime sync.Map //Find peer means adding it to routing table
}

/*
	event happens during find providers for a cid
*/

type ProviderEvent struct {
	//lock *sync.RWMutex
	c    cid.Cid
	mh   string
	self string

	FindProviderAsync       time.Time
	FinishLocalSearch       time.Time
	FirstOutputProviderTime sync.Map //peerID, time

	//structure of dht search tree
	FirstGotProviderFrom sync.Map // provider,peer
	FirstGotCloserFrom   sync.Map // closer, peer

	//time point in dht search tree
	FirstRequestTime  sync.Map // peerID,time
	FirstResponseTime sync.Map //peerID, time

	CPL sync.Map //peerID.string, cpl
}

func NewFPMonitor() *FindProviderMonitor {
	var fpm FindProviderMonitor
	return &fpm
}

func (m *FindProviderMonitor) NewProviderEvent(cid cid.Cid, multihash string, selfID string, selfcpl int) {
	if !CMD_EnableMetrics {
		return
	}
	//fmt.Printf("%s NewProviderEvent %s %s\n", time.Now(), cid, multihash)
	if _, ok := m.EventList.Load(multihash); !ok {
		var pe ProviderEvent
		pe.c = cid
		pe.mh = multihash
		pe.FindProviderAsync = time.Now()
		pe.FinishLocalSearch = ZeroTime
		pe.self = selfID
		//pe.lock = new(sync.RWMutex)
		pe.CPL.Store(selfID, selfcpl)

		m.EventList.Store(multihash, &pe)
	}
}
func (m *FindProviderMonitor) PMGotProviders(mh string) {
	//fmt.Printf("%s PMGotProviders %s\n", time.Now(), mh)
	if v, ok := m.EventList.Load(mh); ok {
		pe := v.(*ProviderEvent)
		pe.FinishLocalSearch = time.Now()
		m.EventList.Store(pe.mh, pe)
	}
}

func (m *FindProviderMonitor) GotProviderFrom(mh string, provider string, from string) {
	//fmt.Printf("%s GotProviderFrom %s %s %s\n", time.Now(), target, provider, from)

	if v, ok := m.EventList.Load(mh); ok {
		pe := v.(*ProviderEvent)
		if _, ok := pe.FirstGotProviderFrom.Load(provider); !ok {
			pe.FirstGotProviderFrom.Store(provider, from)
			m.EventList.Store(mh, pe)
		}

		if _, ok := pe.FirstOutputProviderTime.Load(provider); !ok {
			pe.FirstOutputProviderTime.Store(provider, time.Now())
			m.EventList.Store(mh, pe)
		}
	}
}
func (m *FindProviderMonitor) GotCloserFrom(mh string, closers []string, from string, cpls []int) {
	for i, c := range closers {
		//fmt.Printf("%s GotCloserFrom %s  %s\n", time.Now(), c, from)

		if v, ok := m.EventList.Load(mh); ok {
			pe := v.(*ProviderEvent)
			//pe.lock.RLock()
			_, ok := pe.FirstGotCloserFrom.Load(c)
			//_,ok2:=m.FirstFindPeerTime.Load(c)
			_, ok3 := pe.FirstRequestTime.Load(c)
			//pe.lock.RUnlock()
			if !ok && !ok3 {
				//pe.lock.Lock()
				pe.FirstGotCloserFrom.Store(c, from)
				pe.CPL.Store(c, cpls[i])
				//pe.lock.Unlock()
			} else {
				//fmt.Printf("closer already have parent %s\n", p.(string))
			}

			m.EventList.Store(mh, pe)
		}
	}

}

func (m *FindProviderMonitor) SendNodeWant(mh string, peer string, peercpl int) {
	//fmt.Printf("%s SendNodeWant %s %s\n", time.Now(), target, peer)
	if v, ok := m.EventList.Load(mh); ok {
		pe := v.(*ProviderEvent)
		needWB := false
		if _, ok := pe.FirstRequestTime.Load(peer); !ok {
			needWB = true
			pe.FirstRequestTime.Store(peer, time.Now())
			pe.CPL.Store(peer, peercpl)
		}
		if needWB {
			m.EventList.Store(mh, pe)
		}
	}
}

func (m *FindProviderMonitor) ReceiveResult(mh string, peer string) {
	//fmt.Printf("%s ReceiveResult %s %s\n", time.Now(), target, peer)

	if v, ok := m.EventList.Load(mh); ok {
		pe := v.(*ProviderEvent)
		if _, ok := pe.FirstResponseTime.Load(peer); !ok {
			pe.FirstResponseTime.Store(peer, time.Now())
			m.EventList.Store(mh, pe)
		}
	}
}

func (m *FindProviderMonitor) FindPeer(peer string) {
	if !CMD_EnableMetrics {
		return
	}
	if _, ok := m.FirstFindPeerTime.Load(peer); !ok {
		m.FirstFindPeerTime.Store(peer, time.Now())
	}

}

func (m *FindProviderMonitor) InheritFindPeer(oldMonitor *FindProviderMonitor) {
	if !CMD_EnableMetrics {
		return
	}
	oldMonitor.FirstFindPeerTime.Range(func(key, value interface{}) bool {
		m.FirstFindPeerTime.Store(key.(string), value.(time.Time))

		return true
	})
}

func (m *FindProviderMonitor) CriticalPath() map[cid.Cid][]string {
	CPforBlk := make(map[cid.Cid][]string)
	m.EventList.Range(func(key, value interface{}) bool {
		var cp []string
		var blk cid.Cid

		pe := value.(*ProviderEvent)
		//fmt.Printf("CID: %s\n", pe.c)
		blk = pe.c
		pe.FirstGotProviderFrom.Range(func(key, value interface{}) bool {
			//fmt.Printf("Providers: %s\n", key.(string))
			cp = append(cp, key.(string))
			father := value.(string)
			for true {
				//fmt.Printf("%s\n", father)
				cp = append(cp, father)
				if newf, ok := pe.FirstGotCloserFrom.Load(father); ok {
					father = newf.(string)
				} else {
					break
				}
			}
			return true
		})
		CPforBlk[blk] = cp
		return true
	})
	return CPforBlk
}
