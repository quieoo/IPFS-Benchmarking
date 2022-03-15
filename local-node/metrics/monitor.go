package metrics

import (
	"fmt"
	"github.com/ipfs/go-cid"
	"sync"
	"time"
)

/*
	BlockServiceGetTime=GetBlocksRequest-BlockserviceGet
	NeighbourAskingTime:
		if FirstGotProvider < FirstWantTo:
			NeighbourAskingTime=FirstFindProvider-GetBlocksRequest
	FindProviderTime:
		if FirstGotProvider < FirstWantTo:
			FindProviderTim=FirstGotProvider-FirstFindProvider
	WaitToWantTime:
		if FirstGotProvider < FirstWantTo:
			WaitToWantTime=FirstWantTo-FirstGotProvider
		else:
			WaitToWantTime=FirstWantTo-GetBlocksRequest
	BitswapTime=FirstReceive-FirstWantTo
	BeforeVisitTime=BeginVisit-FirstReceive
	VisitTime=FinishVisit-BeginVisit

Total time of "Get" a file includes 2 part:
	root node: BlockServiceGetTime + NeighbourAskingTime + FindProviderTime + WaitToWantTime + BitswapTime + BeforeVisitTime + VisitTime
	leaf nodes:
		(walker fetch 10 child nodes at a time)
		Average(BlockServiceGetTime + WaitToWantTime + BitswapTime + BeforeVisitTime + VisitTime) * (leafNodeNumber/10+1)
*/
type Monitor struct {
	EventList     sync.Map
	Root          cid.Cid
	GetStartTime  time.Time
	GetFinishTime time.Time
}

type BlockEvent struct {
	Level             int
	lock              *sync.RWMutex
	BlockServiceGet   time.Time
	GetBlocksRequest  time.Time
	FirstWantTo       sync.Map
	FirstFindProvider time.Time
	FirstGotProvider  sync.Map
	FirstReceive      time.Time
	ReceiveFrom       string
	BeginVisit        time.Time
	FinishVisit       time.Time
}

func Newmonitor() *Monitor {
	var monitor Monitor
	return &monitor
}

var ZeroTime = time.Unix(0, 0)

func (m *Monitor) NewBlockEvent(c cid.Cid, l int) {
	//fmt.Printf("NewBlockEvent %s %s\n", c, time.Now())
	if l == 0 {
		m.Root = c
	}
	var be BlockEvent
	be.Level = l
	be.GetBlocksRequest = ZeroTime
	be.FirstReceive = ZeroTime
	be.FirstFindProvider = ZeroTime
	be.BlockServiceGet = ZeroTime
	be.BeginVisit = ZeroTime
	be.FinishVisit = ZeroTime
	be.lock = new(sync.RWMutex)
	m.EventList.Store(c, be)
}
func (m *Monitor) NewBlockEnevts(ks []cid.Cid, l int) {
	//fmt.Printf("NewBlockEvent %s %s\n", c, time.Now())
	for _, c := range ks {
		var be BlockEvent
		be.Level = l
		be.GetBlocksRequest = ZeroTime
		be.FirstReceive = ZeroTime
		be.FirstFindProvider = ZeroTime
		be.BlockServiceGet = ZeroTime
		be.BeginVisit = ZeroTime
		be.FinishVisit = ZeroTime
		be.lock = new(sync.RWMutex)
		m.EventList.Store(c, be)
	}
}

/*
	remember all timestamps:
		BlockServiceGet
		BitswapGet
		FindProviders
		FoundProvide
		SendWant
		ReceiveBlock
		BeginVisit
		FinishVisit
*/

func (m *Monitor) BlockServiceGet(c cid.Cid) {
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		be.BlockServiceGet = time.Now()
		m.EventList.Store(c, be)
	}
}
func (m *Monitor) BlockServiceGets(ks []cid.Cid) {
	t := time.Now()
	for _, c := range ks {
		if v, ok := m.EventList.Load(c); ok {
			be := v.(BlockEvent)
			be.BlockServiceGet = t
			m.EventList.Store(c, be)
		}
	}
}

func (m *Monitor) BitswapGet(c cid.Cid) {
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		be.GetBlocksRequest = time.Now()
		m.EventList.Store(c, be)
	}
}
func (m *Monitor) ReceiveBlock(c cid.Cid, p string) {
	//fmt.Printf("ReceiveBlock %s %s %s\n", c, p, time.Now())
	value, ok := m.EventList.Load(c)
	if ok {
		be := value.(BlockEvent)
		if be.FirstReceive == ZeroTime {
			be.FirstReceive = time.Now()
			be.ReceiveFrom = p
			m.EventList.Store(c, be)
		}
	}
}
func (m *Monitor) SendWant(c cid.Cid, p string) {
	//fmt.Printf("SendWantTo %s %s %s\n", c, p, time.Now())
	v, ok := m.EventList.Load(c)
	if ok {
		be := v.(BlockEvent)
		be.lock.RLock()
		_, ok = be.FirstWantTo.Load(p)
		be.lock.RUnlock()
		if !ok {
			be.lock.Lock()
			be.FirstWantTo.Store(p, time.Now())
			be.lock.Unlock()
		}
		m.EventList.Store(c, be)
	}
}
func (m *Monitor) FindProviders(c cid.Cid) {
	//fmt.Printf("FindProviders %s %s\n", c, time.Now())
	v, ok := m.EventList.Load(c)
	if ok {
		be := v.(BlockEvent)
		if be.FirstFindProvider == ZeroTime {
			be.FirstFindProvider = time.Now()
			m.EventList.Store(c, be)
		}
	}

}
func (m *Monitor) FoundProvide(mh string, p string) {
	//fmt.Printf("FoundProvide %s %s %s\n", mh, p, time.Now())
	var be BlockEvent
	var c cid.Cid
	found := false
	m.EventList.Range(func(key, value interface{}) bool {
		hash := key.(cid.Cid).Hash()
		if hash.String() == mh {
			c = key.(cid.Cid)
			be = value.(BlockEvent)
			found = true
			return false
		}
		return true
	})
	if found {
		if _, ok := be.FirstGotProvider.Load(p); !ok {
			be.FirstGotProvider.Store(p, time.Now())
			m.EventList.Store(c, be)
		}
	}
}

func (m *Monitor) BeginVisit(c cid.Cid) {
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		be.BeginVisit = time.Now()
		m.EventList.Store(c, be)
	}
}
func (m *Monitor) FinishVisit(c cid.Cid) {
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		be.FinishVisit = time.Now()
		m.EventList.Store(c, be)
	}
}

//Statistics on all time stamps:
func (m *Monitor) RootBlockServiceTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		return be.GetBlocksRequest.Sub(be.BlockServiceGet)
	}
	fmt.Println("root not found")
	return time.Duration(0)
}
func (m *Monitor) AvgLeafBlockServiceTime() time.Duration {
	sum := time.Duration(0)
	var num int64
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.Level > 0 {
			sum += be.GetBlocksRequest.Sub(be.BlockServiceGet)
			num++
		}
		return true
	})
	if num == 0 {
		return sum
	}
	if num == 0 {
		return sum
	}
	return time.Duration(sum.Nanoseconds() / num)
}
func (m *Monitor) RootNeighbourAskingTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				if gotpt.(time.Time).Before(wantt.(time.Time)) {
					return be.FirstFindProvider.Sub(be.GetBlocksRequest)
				} else {
					fmt.Println("root sent WANT before find provider by 'FindProviders'")
					return time.Duration(0)
				}
			} else {
				fmt.Println("RootNeighbourAskingTime: root dose not receive from peer who just send WANT to")
				return time.Duration(0)
			}
		} else {
			//fmt.Println("root does not receive blocks from provider found by 'FindProviders'")
			return time.Duration(0)
		}
	}
	fmt.Println("root not found")
	return time.Duration(0)
}
func (m *Monitor) RootFindProviderTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				if gotpt.(time.Time).Before(wantt.(time.Time)) {
					return gotpt.(time.Time).Sub(be.FirstFindProvider)
				} else {
					fmt.Println("root sent WANT before find provider by 'FindProviders'")
					return time.Duration(0)
				}
			} else {
				fmt.Println("RootFindProviderTime: root dose not receive from peer who just send WANT to")
				return time.Duration(0)
			}
		} else {
			//fmt.Println("root does not receive blocks from provider found by 'FindProviders'")
			return time.Duration(0)
		}
	}
	fmt.Println("root not found")
	return time.Duration(0)
}

func (m *Monitor) RootWaitToWantTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				if gotpt.(time.Time).Before(wantt.(time.Time)) {
					return wantt.(time.Time).Sub(gotpt.(time.Time))
				} else {
					fmt.Println("root sent WANT before find provider by 'FindProviders'")
					return time.Duration(0)
				}
			} else {
				fmt.Println("RootWaitToWantTime: root dose not receive from peer who just send WANT to")
				return time.Duration(0)
			}
		} else {
			//fmt.Println("root does not receive blocks from provider found by 'FindProviders'")
			return time.Duration(0)
		}
	}
	fmt.Println("root not found")
	return time.Duration(0)
}
func (m *Monitor) AvgLeafWaitToWantTime() time.Duration {
	sum := time.Duration(0)
	var num int64
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.Level > 0 {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				sum += wantt.(time.Time).Sub(be.GetBlocksRequest)
				num++
			} else {
				fmt.Println("leaf node dose not receive from peer who just send WANT to")
			}
		}

		return true
	})
	if num == 0 {
		return sum
	}
	return time.Duration(sum.Nanoseconds() / num)
}

func (m *Monitor) RootBitswapTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
			return be.FirstReceive.Sub(wantt.(time.Time))
		} else {
			fmt.Println("RootBitswapTime: root dose not receive from peer who just send WANT to")
			return time.Duration(0)
		}
	}
	fmt.Println("root not found")
	return time.Duration(0)
}
func (m *Monitor) AvgLeafBitswapTime() time.Duration {
	sum := time.Duration(0)
	var num int64
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.Level > 0 {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				sum += be.FirstReceive.Sub(wantt.(time.Time))
				num++
			} else {
				fmt.Println("AvgLeafBitswapTime: dose not receive from peer who just send WANT to")
			}
		}
		return true
	})
	if num == 0 {
		return sum
	}
	return time.Duration(sum.Nanoseconds() / num)
}

func (m *Monitor) RootBeforeVisitTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		return be.BeginVisit.Sub(be.FirstReceive)
	}
	fmt.Println("root not found")
	return time.Duration(0)
}
func (m *Monitor) AvgLeafBeforeVisitTime() time.Duration {
	sum := time.Duration(0)
	var num int64
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.Level > 0 {
			sum += be.BeginVisit.Sub(be.FirstReceive)
			num++
		}
		return true
	})
	if num == 0 {
		return sum
	}
	return time.Duration(sum.Nanoseconds() / num)
}

func (m *Monitor) RootVisitTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		return be.FinishVisit.Sub(be.BeginVisit)
	}
	fmt.Println("root not found")
	return time.Duration(0)
}
func (m *Monitor) AvgLeafVisitTime() time.Duration {
	sum := time.Duration(0)
	var num int64
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.Level > 0 {
			sum += be.FinishVisit.Sub(be.BeginVisit)
			num++
		}
		return true
	})
	if num == 0 {
		return sum
	}
	return time.Duration(sum.Nanoseconds() / num)
}

func (m *Monitor) LeafNumber() int {
	result := 0
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.Level > 0 {
			result++
		}

		return true
	})
	return result
}
func (m *Monitor) RealTime() time.Duration {
	return m.GetFinishTime.Sub(m.GetStartTime)
}
func (m *Monitor) ModeledTime() time.Duration {
	root := m.RootBlockServiceTime().Nanoseconds() + m.RootNeighbourAskingTime().Nanoseconds() + m.RootFindProviderTime().Nanoseconds() +
		m.RootWaitToWantTime().Nanoseconds() + m.RootBitswapTime().Nanoseconds() + m.RootBeforeVisitTime().Nanoseconds() + m.RootVisitTime().Nanoseconds()
	leaf := int64(m.LeafNumber()/10+1) * (m.AvgLeafBlockServiceTime().Nanoseconds() + m.AvgLeafWaitToWantTime().Nanoseconds() + m.AvgLeafBitswapTime().Nanoseconds() + m.AvgLeafVisitTime().Nanoseconds() +
		m.AvgLeafBeforeVisitTime().Nanoseconds())
	return time.Duration(root + leaf)
}

func (m *Monitor) blkWantToTarget(c cid.Cid) time.Time {
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		if v, ok = be.FirstWantTo.Load(be.ReceiveFrom); ok {
			return v.(time.Time)
		}
	}
	return ZeroTime
}
func (m *Monitor) blkGotProviderTarget(c cid.Cid) time.Time {
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		if v, ok = be.FirstGotProvider.Load(be.ReceiveFrom); ok {
			return v.(time.Time)
		}
	}
	return ZeroTime
}

func (m *Monitor) SingleFileMetrics() {

}
func (m *Monitor) TimeStamps() {
	fmt.Printf("BeginFileGet-FinishFileGet\n")
	fmt.Printf("%.2f-%.2f\n", 0.0, m.GetFinishTime.Sub(m.GetStartTime).Seconds()*1000)
	fmt.Printf("BlockserviceGet-GetBlocksRequest-FirstFindProvider-FirstGotProvider-FirstWantTo-FirstReceive-BeginVisit-FinishVisit\n")
	m.EventList.Range(func(key, value interface{}) bool {
		cid := key.(cid.Cid)
		be := value.(BlockEvent)
		// fmt.Printf("%s ", cid)
		if value := be.BlockServiceGet; value != ZeroTime {
			fmt.Printf("%.2f ", value.Sub(m.GetStartTime).Seconds()*1000)
		} else {
			fmt.Printf("0 ")
		}
		if value := be.GetBlocksRequest; value != ZeroTime {
			fmt.Printf("%.2f ", value.Sub(m.GetStartTime).Seconds()*1000)
		} else {
			fmt.Printf("0 ")
		}
		if value := be.FirstFindProvider; value != ZeroTime {
			fmt.Printf("%.2f ", value.Sub(m.GetStartTime).Seconds()*1000)
		} else {
			fmt.Printf("0 ")
		}
		if value := m.blkGotProviderTarget(cid); value != ZeroTime {
			fmt.Printf("%.2f ", value.Sub(m.GetStartTime).Seconds()*1000)
		} else {
			fmt.Printf("0 ")
		}
		if value := m.blkWantToTarget(cid); value != ZeroTime {
			fmt.Printf("%.2f ", value.Sub(m.GetStartTime).Seconds()*1000)
		} else {
			fmt.Printf("0 ")
		}
		if value := be.FirstReceive; value != ZeroTime {
			fmt.Printf("%.2f ", value.Sub(m.GetStartTime).Seconds()*1000)
		} else {
			fmt.Printf("0 ")
		}
		if value := be.BeginVisit; value != ZeroTime {
			fmt.Printf("%.2f ", value.Sub(m.GetStartTime).Seconds()*1000)
		} else {
			fmt.Printf("0 ")
		}
		if value := be.FinishVisit; value != ZeroTime {
			fmt.Printf("%.2f ", value.Sub(m.GetStartTime).Seconds()*1000)
		} else {
			fmt.Printf("0 ")
		}
		fmt.Printf("\n")

		return true
	})
}

func (be *BlockEvent) SetLevel(l int) {
	be.Level = l
}
