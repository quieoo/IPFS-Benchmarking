package metrics

import (
	"fmt"
	"github.com/ipfs/go-cid"
	"sync"
	"time"
)

/*
	TotalBlocks: LeafBlocks+InternalBlocks
	  MaxLinks=BlockSize/LinkSize
	  Levels= (Lg(LeafBlocks) / Lg(MaxLinks)) + 1
	LoadBatchSize=Min{10, MaxLinks}
	Batchs

	BlockServiceTime=TotalBlocks * AvgBlockServiceTime
	FindProviderTime=RootNeighbourAsking + RootFindProvider
	WaitToWantTime=RootWaitToWant + AvgLeafWaitToWant * (Batchs)
	BitswapTime=TotalBlocks * AvgBitswap
	PutStoreTime=TotalBlocks * AvgPutToStore
	VisitTime = TotalBlocks * AvgVisit
*/
type Monitor struct {
	EventList     sync.Map
	Root          cid.Cid
	GetStartTime  time.Time
	GetFinishTime time.Time

	TotalBlocks  int
	TotalFetches int
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
	FirstPutStore     time.Time
	ReceiveFrom       string
	BeginVisit        time.Time
	FinishVisit       time.Time

	NumOfRedundantReqs int
	NumOfRedundantBlks int
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
	be.FirstPutStore = ZeroTime
	be.lock = new(sync.RWMutex)
	be.NumOfRedundantReqs = 0
	be.NumOfRedundantBlks = 0
	m.EventList.Store(c, be)
	m.TotalBlocks++
	m.TotalFetches++
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
		be.FirstPutStore = ZeroTime
		be.NumOfRedundantReqs = 0
		be.NumOfRedundantBlks = 0
		be.lock = new(sync.RWMutex)
		m.EventList.Store(c, be)
	}
	m.TotalBlocks += len(ks)
	m.TotalFetches++
}

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
		} else {
			// Nonzero FirstReceive means that we have already received a block with cid c;
			// so this block is a redundant block.
			be.NumOfRedundantBlks++
			m.EventList.Store(c, be)
		}
	}
}

func (m *Monitor) PutStore(blk cid.Cid) {
	if v, ok := m.EventList.Load(blk); ok {
		be := v.(BlockEvent)
		if be.FirstPutStore == ZeroTime {
			be.FirstPutStore = time.Now()
			m.EventList.Store(blk, be)
		}
	}
}
func (m *Monitor) SendWant(c cid.Cid, p string) {
	//fmt.Printf("SendWantTo %s %s %s\n", c, p, time.Now())
	v, ok := m.EventList.Load(c)
	if ok {
		be := v.(BlockEvent)

		// FirstReceive is nonzero means that we have received the
		// wanted block with the cid c at FirstReceive. So this want
		// is a redundant request
		if be.FirstReceive != ZeroTime {
			be.NumOfRedundantReqs++
		}

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

func (m *Monitor) AvgBlockServiceTime() time.Duration {
	var sumTime int64
	num := 0
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if blockServ := be.BlockServiceGet; blockServ != ZeroTime {
			if request := be.GetBlocksRequest; request != ZeroTime {
				sumTime += request.Sub(blockServ).Nanoseconds()
				num++
			}
		}
		return true
	})
	if num != 0 {
		return time.Duration(sumTime / int64(num))
	}
	return time.Duration(0)
}

func (m *Monitor) RootNeighbourAskingTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				if gotpt.(time.Time).Before(wantt.(time.Time)) && be.FirstFindProvider != ZeroTime && be.GetBlocksRequest != ZeroTime {
					return be.FirstFindProvider.Sub(be.GetBlocksRequest)
				}
			}
		}
	}
	return time.Duration(0)
}
func (m *Monitor) RootFindProviderTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				if gotpt.(time.Time).Before(wantt.(time.Time)) && be.FirstFindProvider != ZeroTime {
					return gotpt.(time.Time).Sub(be.FirstFindProvider)
				}
			}
		}
	}
	return time.Duration(0)
}

func (m *Monitor) RootWaitToWantTime() time.Duration {
	if c, ok := m.EventList.Load(m.Root); ok {
		be := c.(BlockEvent)
		if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				if gotpt.(time.Time).Before(wantt.(time.Time)) {
					return wantt.(time.Time).Sub(gotpt.(time.Time))
				}
			}
		}
	}
	return time.Duration(0)
}
func (m *Monitor) AvgLeafWaitToWantTime() time.Duration {
	sum := time.Duration(0)
	var num int64
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.Level > 0 {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				if be.FirstFindProvider != ZeroTime {
					sum += wantt.(time.Time).Sub(be.GetBlocksRequest)
					num++
				}
			}
		}
		return true
	})
	if num == 0 {
		return time.Duration(0)
	}
	return time.Duration(sum.Nanoseconds() / num)
}
func (m *Monitor) AvgBitswapTime() time.Duration {
	sum := time.Duration(0)
	var num int64
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)

		if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
			if be.FirstReceive != ZeroTime {
				sum += be.FirstReceive.Sub(wantt.(time.Time))
				num++

			}
		} else {
			fmt.Println("AvgLeafBitswapTime: dose not receive from peer who just send WANT to")
		}

		return true
	})
	if num == 0 {
		return sum
	}
	return time.Duration(sum.Nanoseconds() / num)
}
func (m *Monitor) AvgPutToStore() time.Duration {
	sum := time.Duration(0)
	num := 0
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.FirstReceive != ZeroTime && be.FirstPutStore != ZeroTime {
			sum += be.FirstPutStore.Sub(be.FirstReceive)
			num++
		}
		return true
	})
	if num == 0 {
		return time.Duration(0)
	}
	return time.Duration(sum.Nanoseconds() / int64(num))
}
func (m *Monitor) AvgVisitTime() time.Duration {
	sum := time.Duration(0)
	var num int64
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		if be.FinishVisit != ZeroTime && be.BeginVisit != ZeroTime {
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

func (m *Monitor) RealTime() time.Duration {
	return m.GetFinishTime.Sub(m.GetStartTime)
}
func (m *Monitor) ModeledTime() time.Duration {
	var result int64
	totalFetch := int64(m.TotalFetches)
	result = totalFetch*m.AvgBlockServiceTime().Nanoseconds() +
		m.RootNeighbourAskingTime().Nanoseconds() + m.RootFindProviderTime().Nanoseconds() +
		m.RootWaitToWantTime().Nanoseconds() + (totalFetch-1)*m.AvgLeafWaitToWantTime().Nanoseconds() +
		totalFetch*m.AvgBitswapTime().Nanoseconds() +
		totalFetch*m.AvgPutToStore().Nanoseconds() +
		totalFetch*m.AvgVisitTime().Nanoseconds()

	return time.Duration(result)
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
	fmt.Printf("BlockserviceGet-GetBlocksRequest-FirstFindProvider-FirstGotProvider-FirstWantTo-FirstReceive-PutStore-BeginVisit-FinishVisit\n")
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
		if value := be.FirstPutStore; value != ZeroTime {
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

func (m *Monitor)SumBlksRedundant() int {
	sum := 0
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		sum += be.NumOfRedundantBlks
		return true
	})
	return sum
}

func (m *Monitor)SumReqsRedundant() int {
	sum := 0
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		sum += be.NumOfRedundantReqs
		return true
	})
	return sum
}

func (be *BlockEvent) SetLevel(l int) {
	be.Level = l
}
