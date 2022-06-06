package metrics

import (
	"bufio"
	"encoding/csv"
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	gometrics "github.com/rcrowley/go-metrics"
	ks "github.com/whyrusleeping/go-keyspace"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PeerLatency struct {
	plm  sync.Map
	pltm sync.Map // 记录时间列表

	beg time.Time
}

func NewPeerLatency(alpha float64) *PeerLatency {

	var pl PeerLatency

	pl = PeerLatency{
		plm:  sync.Map{},
		pltm: sync.Map{},
		beg:  time.Now(),
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

type CacheItem struct {
	responseDur float64
}

func NewCacheItem(rspd float64) *CacheItem {
	var ci CacheItem
	ci = CacheItem{
		responseDur: rspd,
	}
	return &ci
}

// PeerResponseHistory
type PeerResponseHistory struct {
	// a, b, score 公式的权重
	a float64
	b float64
	//a *big.Float
	//b *big.Float

	// lruCache 用来存储每个peer最近一次的 response 时间
	// <peerID, *CacheItem>
	lruCache *lru.Cache

	// metaMp 存储一些关键的元数据
	// <"avgTime", float64> 存储当前的 mp 记录过的时间的平均值
	// to rm <"miss", int64> 存储为命中的次数
	metaMp sync.Map

	hit  gometrics.Counter // findTime hit 了多少次
	miss gometrics.Counter // findTime miss 了多少次

	Compromise gometrics.Counter // 因为时间而妥协的次数
	AllCmp     gometrics.Counter // 所有的比较次数

	updateCnt gometrics.Counter // Update 了多少次

}

// NewPeerRH 创建一个 PeerResponseHistory 对象
func NewPeerRH(a, b float64, size int) *PeerResponseHistory {
	var prh PeerResponseHistory
	lc, err := lru.New(size)
	if err != nil {
		fmt.Println("fail to create cache !!!")
		os.Exit(13)
	}
	prh = PeerResponseHistory{
		a:        a,
		b:        b,
		lruCache: lc,
		metaMp:   sync.Map{},

		hit:        gometrics.NewCounter(),
		miss:       gometrics.NewCounter(),
		updateCnt:  gometrics.NewCounter(),
		Compromise: gometrics.NewCounter(),
		AllCmp:     gometrics.NewCounter(),
	}
	err1 := gometrics.Register("hit", prh.hit)
	err2 := gometrics.Register("miss", prh.miss)
	err3 := gometrics.Register("updateCnt", prh.updateCnt)
	err4 := gometrics.Register("Compromise", prh.Compromise)
	err5 := gometrics.Register("AllCmp", prh.AllCmp)

	if err1 != nil && err2 != nil && err3 != nil && err4 != nil && err5 != nil {
		fmt.Println("fail to register metrics")
		os.Exit(14)
	}

	prh.metaMp.Store("avgTime", float64(0)) // 初始值为 0 是可以的 —— 意味着不会产生影响

	return &prh
}

//func (prh *PeerResponseHistory) GetScore(dis *big.Int, peerID string) *big.Float {
func (prh *PeerResponseHistory) GetScore(dis *big.Int, peerID string) *float64 {
	// expDHT:
	//  1. big.Int 类型转换
	//  2. 对于两个分数，如果一个有，一个没有怎么办？
	// 		2-1. 同时都没有怎么办？
	// 		2-2. 1有1没有怎么办？如果一个有一个没有？那就同时看没有的那个？
	//  一种解决的方案是我们记录一个历史的query时间的平均时间。如果没有，那我们就拿平均的时间进来作比较

	//findT64 := prh.findTime(peerID)
	//
	//if findT64 == -1 {
	//	os.Exit(12)
	//}
	//
	//disF := new(big.Float).SetInt(dis)
	//findTF := new(big.Float).SetFloat64(findT64)
	//var ans_p1 big.Float
	//var ans_p2 big.Float
	//var ans big.Float
	//ans_p1.Mul(prh.a, disF)
	//ans_p2.Mul(prh.b, findTF)
	//ans.Add(&ans_p1, &ans_p2)

	logicDis := (32-len(dis.Bytes()))*8 + ks.ZeroPrefixLen(dis.Bytes())

	// NOTE: 我们让取出来的时间 / 1000
	findT64 := prh.findTime(peerID) / 1000

	if findT64 == -1 {
		os.Exit(12)
	}
	var ans float64

	// 1. 此时我们的逻辑距离是 logicDis, 它是公共前缀的长度；公共前缀越长，逻辑距离越近
	//    所以我们对逻辑距离取相反数
	// 2. 因为历史时间大概在 1e3 ms 这个数量级，所以不妨先让历史时间 / 1000
	// 3. (1-b) * (-逻辑距离) + b * 历史时间
	// 4. 我们暂时先不使用 a 这个参数，我们先用 b 吧

	ans = (1-prh.b)*float64(-logicDis) + prh.b*findT64

	return &ans
}

func (prh *PeerResponseHistory) findTime(peerID string) float64 {
	// 查询 prh.mp 找到对应的时间，暂时不做LRU相关的操作，我们后续再考虑这个问题
	if val, ok := prh.lruCache.Get(peerID); ok {
		ci := val.(*CacheItem)

		prh.hit.Inc(1)

		//fmt.Println("hit", ci.responseDur)

		return ci.responseDur
	}

	if val, ok2 := prh.metaMp.Load("avgTime"); ok2 {
		avgTime := val.(float64)

		prh.miss.Inc(1)

		//fmt.Println("miss", avgTime)

		return avgTime
	}

	return -1
}

// Update 记录 / 更新 某个 peer 的queryTime的时间？
// TODO: 这样做会不会导致因为一次 response 太慢，然后后续更不太可能用到它，从而一直？
func (prh *PeerResponseHistory) Update(peerID string, dur time.Duration) int {
	// 更新 lru-cache
	var tm int64
	tm = dur.Milliseconds()

	prh.lruCache.Add(peerID, NewCacheItem(float64(tm)))
	//prh.mp.Store(peerID, dur.Milliseconds())

	// 更新 prh.metaMp
	var avgTime float64

	if val, ok3 := prh.metaMp.Load("avgTime"); ok3 {
		avgTime = val.(float64)
	}

	allCntF := float64(prh.updateCnt.Count())
	avgTime = avgTime*(allCntF/(allCntF+1)) + float64(tm)/(allCntF+1)
	prh.updateCnt.Inc(1)

	prh.metaMp.Store("avgTime", avgTime)

	return 0
}

// 暂且令这个本地文件叫做 cache.txt

// Load 从一个文件中读取之前保存下来的 Cache 信息
func (prh *PeerResponseHistory) Load() {
	// 读取文件中的每一个 peerID responseTime，然后添加到 Cache 里
	inputFile, inputError := os.Open("cache.txt")
	if inputError != nil {
		fmt.Printf("An error occurred on opening the inputfile\n" +
			"Does the file exist?\n" +
			"Have you got acces to it?\n")
		return // exit the function on error
	}
	defer inputFile.Close()

	inputReader := bufio.NewReader(inputFile)
	cnt := 0
	for {
		peerIDtmp, readerError := inputReader.ReadString(' ')
		if readerError == io.EOF {
			break
		}
		peerID := strings.TrimRight(peerIDtmp, " ")

		peerResponseTmp, readerError := inputReader.ReadString('\n')
		if readerError == io.EOF {
			break
		}
		peerResponse := strings.TrimRight(peerResponseTmp, "\n")

		rspd, err := strconv.ParseFloat(peerResponse, 64)
		if err != nil {
			fmt.Println("fail to convert string to float64.")
			return
		}

		ci := &CacheItem{
			responseDur: rspd,
		}

		GPeerRH.lruCache.Add(peerID, ci)
		cnt++
	}

	fmt.Printf("Preload PeerResponseHistory cache : %v\n", cnt)
}

// Store 从一个文件中读取之前保存下来的 Cache 信息
func (prh *PeerResponseHistory) Store() {
	// 将 Cache 信息写到本地的文件中
	outputFile, outputError := os.OpenFile("cache.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if outputError != nil {
		fmt.Printf("An error occurred with file opening or creation\n")
		return
	}
	defer outputFile.Close()

	outputWriter := bufio.NewWriter(outputFile)

	keys := GPeerRH.lruCache.Keys()
	len := GPeerRH.lruCache.Len()
	cnt := 0
	for i := 0; i < len; i++ {
		val, _ := GPeerRH.lruCache.Peek(keys[i])
		key := keys[i].(string)
		rspds := fmt.Sprintf("%f", val.(*CacheItem).responseDur)
		res := key + " " + rspds + "\n"
		outputWriter.WriteString(res)
		cnt++
		//fmt.Println(res)
	}

	outputWriter.Flush()
	fmt.Printf("Store PeerResponseHistory cache : %v\n", cnt)
}
