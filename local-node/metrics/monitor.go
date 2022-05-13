package metrics

import (
	"fmt"
	"github.com/ipfs/go-cid"
	"sort"
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
	WaitToWantTime=RootWaitToWant + AvgLeafWaitToWafnt * (Batchs)
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
	if !CMD_EnableMetrics {
		return nil
	}
	var monitor Monitor
	return &monitor
}

var ZeroTime = time.Unix(0, 0)

func (m *Monitor) NewBlockEvent(c cid.Cid, l int) {
	if !CMD_EnableMetrics {
		return
	}
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
	if !CMD_EnableMetrics {
		return
	}
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
	if !CMD_EnableMetrics {
		return
	}
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		be.BlockServiceGet = time.Now()
		m.EventList.Store(c, be)
	}
}
func (m *Monitor) BlockServiceGets(ks []cid.Cid) {
	if !CMD_EnableMetrics {
		return
	}
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
	if !CMD_EnableMetrics {
		return
	}
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		be.GetBlocksRequest = time.Now()
		m.EventList.Store(c, be)
	}
}
func (m *Monitor) ReceiveBlock(c cid.Cid, p string) {
	if !CMD_EnableMetrics {
		return
	}
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
	if !CMD_EnableMetrics {
		return
	}
	if v, ok := m.EventList.Load(blk); ok {
		be := v.(BlockEvent)
		if be.FirstPutStore == ZeroTime {
			be.FirstPutStore = time.Now()
			m.EventList.Store(blk, be)
		}
	}
}
func (m *Monitor) SendWant(c cid.Cid, p string) {
	if !CMD_EnableMetrics {
		return
	}
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
	if !CMD_EnableMetrics {
		return
	}
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
	if !CMD_EnableMetrics {
		return
	}
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
	if !CMD_EnableMetrics {
		return
	}
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		be.BeginVisit = time.Now()
		m.EventList.Store(c, be)
	}
}
func (m *Monitor) FinishVisit(c cid.Cid) {
	if !CMD_EnableMetrics {
		return
	}
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		be.FinishVisit = time.Now()
		m.EventList.Store(c, be)
	}
}

// ----------------------------------------------------------------------

var TailRaio = 0.9
var ReduceRatio = 0.4

type BreakdownTimeForGet struct {
	BlockService   []time.Duration
	NeighbourAsk   time.Duration
	FindProvider   time.Duration
	RootWaitToWant time.Duration
	LeafWaitToWant []time.Duration
	Bitswap        []time.Duration
	PutStore       []time.Duration
	Visit          []time.Duration
}

// GetBreakdown get all sub-procedure time
func (m *Monitor) GetBreakdown() BreakdownTimeForGet {
	var result BreakdownTimeForGet
	if !CMD_EnableMetrics {
		return result
	}

	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		rootblk := false
		if key.(cid.Cid) == m.Root {
			rootblk = true
			if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
				if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
					if gotpt.(time.Time).Before(wantt.(time.Time)) && be.FirstFindProvider != ZeroTime && be.GetBlocksRequest != ZeroTime {
						result.NeighbourAsk = be.FirstFindProvider.Sub(be.GetBlocksRequest)
					}
				}
			}
			if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
				if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
					if gotpt.(time.Time).Before(wantt.(time.Time)) && be.FirstFindProvider != ZeroTime {
						result.FindProvider = gotpt.(time.Time).Sub(be.FirstFindProvider)
					}
				}
			}
			if gotpt, ok := be.FirstGotProvider.Load(be.ReceiveFrom); ok {
				if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
					if gotpt.(time.Time).Before(wantt.(time.Time)) {
						result.RootWaitToWant = wantt.(time.Time).Sub(gotpt.(time.Time))
					}
				}
			}
		}

		if blockServ := be.BlockServiceGet; blockServ != ZeroTime {
			if request := be.GetBlocksRequest; request != ZeroTime {
				result.BlockService = append(result.BlockService, request.Sub(blockServ))
			}
		}
		if !rootblk {
			if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
				if be.FirstFindProvider != ZeroTime {
					result.LeafWaitToWant = append(result.LeafWaitToWant, wantt.(time.Time).Sub(be.GetBlocksRequest))
				}
			}
		}
		if wantt, ok := be.FirstWantTo.Load(be.ReceiveFrom); ok {
			if be.FirstReceive != ZeroTime {
				result.Bitswap = append(result.Bitswap, be.FirstReceive.Sub(wantt.(time.Time)))
			}
		}
		if be.FirstReceive != ZeroTime && be.FirstPutStore != ZeroTime {
			result.PutStore = append(result.PutStore, be.FirstPutStore.Sub(be.FirstReceive))
		}
		if be.FinishVisit != ZeroTime && be.BeginVisit != ZeroTime {
			result.Visit = append(result.Visit, be.FinishVisit.Sub(be.BeginVisit))
		}

		return true
	})

	return result
}
func GetAvgAndTail(array []time.Duration) (float64, int64) {
	if len(array) == 0 {
		return 0, 0
	}

	sum := int64(0)
	//copy array
	var ca []int64
	for _, a := range array {
		sum += a.Nanoseconds()
		ca = append(ca, a.Nanoseconds())
	}
	avg := float64(sum) / float64(len(array))

	sort.Slice(ca, func(i, j int) bool {
		return ca[i] < ca[j]
	})
	tailIndex := TailRaio * float64(len(array))
	if tailIndex < 1 {
		tailIndex = 1
	}
	tail := ca[int(tailIndex)-1]

	return avg, tail
}

func (m *Monitor) RealTime() time.Duration {
	if !CMD_EnableMetrics {
		return time.Duration(0)
	}
	return m.GetFinishTime.Sub(m.GetStartTime)
}
func (m *Monitor) ModeledTime() time.Duration {
	if !CMD_EnableMetrics {
		return time.Duration(0)
	}
	var result int64
	Breakdown := m.GetBreakdown()

	blockServiceAVG, blockServiceTail := GetAvgAndTail(Breakdown.BlockService)
	bitswapAVG, bitswapTail := GetAvgAndTail(Breakdown.Bitswap)
	putStoreAVG, putStoreTail := GetAvgAndTail(Breakdown.PutStore)
	leafWWAVG, leafWWTail := GetAvgAndTail(Breakdown.LeafWaitToWant)
	visitAVG, visitTail := GetAvgAndTail(Breakdown.Visit)

	//fmt.Printf("%f %f\n", bitswapAVG/1000000, float64(bitswapTail)/1000000)
	//fmt.Printf("%f %f\n", putStoreAVG/1000000, float64(putStoreTail)/1000000)

	//Root Node Time
	result = Breakdown.NeighbourAsk.Nanoseconds() + Breakdown.FindProvider.Nanoseconds() + Breakdown.RootWaitToWant.Nanoseconds()
	result += int64(blockServiceAVG + bitswapAVG + putStoreAVG + visitAVG)

	if m.TotalFetches > 1 {
		// Leaf Blocks Time
		Avg := blockServiceAVG + bitswapAVG + putStoreAVG + leafWWAVG + visitAVG
		Tail := blockServiceTail + bitswapTail + putStoreTail + leafWWTail + visitTail

		TF := int64(m.TotalFetches - 1)
		result += TF * Tail
		if TF > 1 {
			//reduce wait time due to early request
			result -= TF * int64(ReduceRatio*(float64(Tail)-Avg))
		}
	}

	return time.Duration(result)
}

func (m *Monitor) blkWantToTarget(c cid.Cid) time.Time {
	if !CMD_EnableMetrics {
		return ZeroTime
	}
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		if v, ok = be.FirstWantTo.Load(be.ReceiveFrom); ok {
			return v.(time.Time)
		}
	}
	return ZeroTime
}
func (m *Monitor) blkGotProviderTarget(c cid.Cid) time.Time {
	if !CMD_EnableMetrics {
		return ZeroTime
	}
	if v, ok := m.EventList.Load(c); ok {
		be := v.(BlockEvent)
		if v, ok = be.FirstGotProvider.Load(be.ReceiveFrom); ok {
			return v.(time.Time)
		}
	}
	return ZeroTime
}

func (m *Monitor) TimeStamps() {
	if !CMD_EnableMetrics {
		return
	}
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

func (m *Monitor) SumBlksRedundant() int {
	if !CMD_EnableMetrics {
		return 0
	}
	sum := 0
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		sum += be.NumOfRedundantBlks
		return true
	})
	return sum
}

func (m *Monitor) SumReqsRedundant() int {
	if !CMD_EnableMetrics {
		return 0
	}
	sum := 0
	m.EventList.Range(func(key, value interface{}) bool {
		be := value.(BlockEvent)
		sum += be.NumOfRedundantReqs
		return true
	})
	return sum
}

func (be *BlockEvent) SetLevel(l int) {
	if !CMD_EnableMetrics {
		return
	}
	be.Level = l
}
