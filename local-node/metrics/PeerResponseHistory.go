package metrics

import (
	"encoding/csv"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"sync"
	"time"
)

type PeerLatency struct {
	plm  sync.Map
	pltm sync.Map // 记录时间列表

	beg time.Time

	//cplMaxInAQuery int
	//cplMinInAQuery int
	//tMax           time.Duration
	//tMin           time.Duration
	//
	//accessCounter metrics.Gauge
	//hitCounter    int
	//
	//alpha float64
}

func NewPeerLatency(alpha float64) *PeerLatency {

	var pl PeerLatency

	pl = PeerLatency{
		plm:  sync.Map{},
		pltm: sync.Map{},
		beg:  time.Now(),
		//cplMaxInAQuery: 0,
		//cplMinInAQuery: 0,
		//tMax:           time.Duration(0),
		//tMin:           time.Duration(0),
		//accessCounter:  0,
		//hitCounter:     0,
		//alpha:          alpha,
	}

	return &pl
}

func (pl *PeerLatency) Update(pid string, t time.Time) {
	if val, ok := pl.plm.Load(pid); ok {
		cnt := val.(int)
		cnt += 1
		pl.plm.Store(pid, cnt)
	} else if !ok {
		cnt := 1
		pl.plm.Store(pid, cnt)
	}

	plus := t.Sub(pl.beg)

	if val, ok := pl.pltm.Load(pid); ok {
		lst := val.([]string)
		lst = append(lst, strconv.FormatInt(plus.Milliseconds(), 10))
		pl.pltm.Store(pid, lst)
	} else if !ok {
		lst := make([]string, 0)
		lst = append(lst, strconv.FormatInt(plus.Milliseconds(), 10))
		pl.pltm.Store(pid, lst)
	}
}

func (pl *PeerLatency) PrintToCsv() {
	data := make([][]string, 0)

	pl.plm.Range(func(key, value interface{}) bool {
		pid := key.(string)
		cnt := value.(int)
		tmp := []string{pid, strconv.Itoa(cnt)}

		// 然后将具体的时间也追加进去
		if val, ok := pl.pltm.Load(pid); ok {
			lst := val.([]string)
			tmp = append(tmp, lst...)
		}
		data = append(data, tmp)

		return true
	})

	f, err := os.OpenFile("PL.csv", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := csv.NewWriter(f) //创建一个新的写入文件流
	w.WriteAll(data)      //写入数据
	w.Flush()
}

// Record 记录一个时间
// 只会在 query 一个 peer 之后调用，用来记录数据
// 需要保证线程安全
//func (pl *PeerLatency) Record(pid string, t time.Duration, cpl int) {
//
//}

// GetScore 查找一个 Peer 的 访问时间，并返回一个综合得分
//// TODO: 需要保证线程安全，所以把counter换成metrics对象
//func (pl *PeerLatency) GetScore(pid string, cpl int) float64 {
//
//}

type ListItem struct {
	prv    *ListItem
	nxt    *ListItem
	peerID string
}

type HistoryItem struct {
	latency int64 // Mil sec
	ptr     *ListItem
}

// PeerResponseHistory
type PeerResponseHistory struct {
	// a, b, score 公式的权重
	a *big.Float
	b *big.Float

	// mp 存储 <peerID, <queryLatency, 一个指针>>
	// mp 暂时只存 <peerID, 访问延迟(int64)>
	// 更新策略如下：
	//  1. 数量的上限为
	//  2. 超过数量上限的时候我们使用 LRU 替换策略
	//    2-1. 指向一个排队队列，如果更新？我们就把旧节点放到链表头
	//         如果新增我们就在链表头新增新节点
	//    2-2. 当我们删除一个节点时，我们就从链表的结尾指向我们的这个map的peerID，
	//         然后再删除(使用sync.Map的删除)
	// 我们的时间单位都取 微秒
	mp sync.Map

	// metaMp 存储一些关键的元数据
	// <"cnt", int64> 存储当前的 mp 记录了多少个peer
	// <"avgTime", float64> 存储当前的 mp 记录过的时间的平均值
	// <"allCnt", int64> 存储当前的 mp 曾经添加过多少次记录
	metaMp sync.Map

	// q 是一个链表。每一个链表项为<peerID>
	// 我们再删除一个数据的时候，我们从链表的结尾删除这个元素
	// q list

	// todo: 线程安全
	// cnt 记录
	//cnt int64

	// avgTime, allCnt 用来计算历史所有记录的 query time 平均时间
	//avgTime float64
	//allCnt  int64
}

func NewPeerRH(a, b float64) *PeerResponseHistory {
	var prh PeerResponseHistory
	prh = PeerResponseHistory{
		a:      new(big.Float).SetFloat64(a),
		b:      new(big.Float).SetFloat64(b),
		mp:     sync.Map{},
		metaMp: sync.Map{},
	}

	prh.metaMp.Store("cnt", int64(0))
	prh.metaMp.Store("avgTime", float64(0)) // 初始值为 0 是可以的 —— 意味着不会产生影响
	prh.metaMp.Store("allCnt", int64(0))

	return &prh
}

func (prh *PeerResponseHistory) GetScore(dis *big.Int, peerID string) *big.Float {
	// expDHT:
	//  1. big.Int 类型转换
	//  2. 对于两个分数，如果一个有，一个没有怎么办？
	// 		2-1. 同时都没有怎么办？
	// 		2-2. 1有1没有怎么办？如果一个有一个没有？那就同时看没有的那个？
	//  一种解决的方案是我们记录一个历史的query时间的平均时间。如果没有，那我们就拿平均的时间进来作比较

	disF := new(big.Float).SetInt(dis)

	findT64 := prh.findTime(peerID)

	if findT64 == -1 {
		os.Exit(12)
	}

	findTF := new(big.Float).SetFloat64(findT64)
	var ans_p1 big.Float
	var ans_p2 big.Float
	var ans big.Float
	ans_p1.Mul(prh.a, disF)
	ans_p2.Mul(prh.b, findTF)
	ans.Add(&ans_p1, &ans_p2)
	return &ans
}

func (prh *PeerResponseHistory) findTime(peerID string) float64 {
	// 查询 prh.mp 找到对应的时间，暂时不做LRU相关的操作，我们后续再考虑这个问题
	if val, ok := prh.mp.Load(peerID); ok {
		tm := val.(int64)
		fmt.Println("hit")
		return float64(tm)
	}

	if val, ok2 := prh.metaMp.Load("avgTime"); ok2 {
		avgTime := val.(float64)
		fmt.Println("miss")
		return avgTime
	}

	return -1
}

// Update 记录 / 更新 某个 peer 的queryTime的时间？
// TODO: 这样做会不会导致因为一次 response 太慢，然后后续更不太可能用到它，从而一直？
func (prh *PeerResponseHistory) Update(peerID string, dur time.Duration) int {
	// 更新 prh.mp
	var tm int64
	prh.mp.Store(peerID, dur.Milliseconds())
	//if val, ok := prh.mp.Load(peerID); ok {
	//	//hi := val.(*HistoryItem)
	//	//hi.latency = dur.Milliseconds()
	//	tm := val.(int64)
	//	tm = dur.Milliseconds()
	//	prh.mp.Store(peerID, tm) // expDHT:此处我有点困惑，如果直接 store 会不会覆盖旧的值？
	//}

	// 更新 prh.metaMp
	var cnt int64
	var avgTime float64
	var allCnt int64

	if val, ok2 := prh.metaMp.Load("cnt"); ok2 {
		cnt = val.(int64)
	}
	if val, ok3 := prh.metaMp.Load("avgTime"); ok3 {
		avgTime = val.(float64)
	}
	if val, ok4 := prh.metaMp.Load("allCnt"); ok4 {
		allCnt = val.(int64)
	}

	cnt = cnt + 1
	allCntF := float64(allCnt)
	avgTime = avgTime*(allCntF/(allCntF+1)) + float64(tm)/(allCntF+1)
	allCnt = allCnt + 1

	prh.metaMp.Store("cnt", cnt)
	prh.metaMp.Store("avgTime", avgTime)
	prh.metaMp.Store("allCnt", allCnt)

	// 更新 prh.q

	return 0
}
