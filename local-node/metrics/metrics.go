package metrics

import (
	"bufio"
	"fmt"
	"github.com/rcrowley/go-metrics"
	"github.com/syndtr/goleveldb/leveldb"
	"os"
	"runtime"
	"sync"
	"time"
)

// some global flag

var CMD_CloseBackProvide = false
var CMD_CloseLANDHT = false
var CMD_CloseDHTRefresh = false
var CMD_CloseAddProvide = false
var CMD_ProvideFirst = false
var CMD_ProvideEach = false
var CMD_EnableMetrics = false
var CMD_StallAfterUpload = false
var CMD_FastSync = false
var ProviderWorker = 8
var CMD_EarlyAbort = false
var EarlyAbortCheck = 5
var CMD_PeerRH = false
var CMD_LoadSaveCache = false
var EnablePbitswap = false

var BlockSizeLimit = 1 * 1024 * 1024

var TimerPin []metrics.Timer
var pinNumber = 2
var TimePin time.Time
var Times []time.Duration

// ADD Metrics

var AddTimer metrics.Timer
var Provide metrics.Timer
var Persist metrics.Timer
var Dag metrics.Timer

var PersistDura time.Duration
var ProvideDura time.Duration
var AddDura time.Duration

var FlatfsHasTimer metrics.Timer
var FlatfsHasDura time.Duration

var FlatfsPut metrics.Timer
var FlatfsPutDura time.Duration

var SyncTime metrics.Timer

/*
	two parts of it:
		keytransform.(*Datastore).Has in go-ipfs-blockstore.(*blockstore).Put
		keytransform.(*Datastore).Has in go-ipfs-blockstore.(*blockstore).PutMany
		go-ipfs-blockstore.(*idstore).Has
*/
var DeduplicateOverhead metrics.Timer

// Get Metrics

var BDMonitor *Monitor
var BlockServiceTime metrics.Timer
var RootNeighbourAskingTime metrics.Timer
var RootFindProviderTime metrics.Timer
var RootWaitToWantTime metrics.Timer
var LeafWaitToWantTime metrics.Timer
var BitswapTime metrics.Timer
var PutStoreTime metrics.Timer
var VisitTime metrics.Timer

var RealGet metrics.Timer
var ModelGet metrics.Timer
var Sample metrics.Sample
var Variance metrics.Histogram

var GetNode metrics.Timer
var WriteTo metrics.Timer

var BlocksRedundant metrics.Histogram
var RequestsRedundant metrics.Histogram

//findProvider metrics
var FPMonitor *FindProviderMonitor
var ChoosePeer metrics.Timer
var DailPeer metrics.Timer
var ResponsePeer metrics.Timer

var RealFindProvider metrics.Timer
var ModelFindProvider metrics.Timer
var Samplefp metrics.Sample
var FPVariance metrics.Histogram
var Samplefpn metrics.Sample
var FPInner metrics.Histogram

// trace-download period throughput output

var DownloadedFileSize []int
var AvgDownloadLatency metrics.Timer
var ALL_DownloadedFileSize []int
var ALL_AvgDownloadLatency metrics.Timer

var GetBreakDownLog = false
var CPLInDHTQureyLog = false

// expDHT:
var GPeerRH *PeerResponseHistory
var B float64 = 1e70

var DataStorePut metrics.Histogram
var MetricsStartTime time.Time

func TraceDownMetricsInit() {
	AvgDownloadLatency = metrics.NewTimer()
	metrics.Register("AvgDownloadLatency", AvgDownloadLatency)
	ALL_AvgDownloadLatency = metrics.NewTimer()
	metrics.Register("ALL_AvgDownloadLatency", ALL_AvgDownloadLatency)

	go func() {
		timeUnit := time.Minute
		file, err := os.OpenFile("PeriodLog", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			fmt.Printf("failed to open file, %s\n", err.Error())
		}
		write := bufio.NewWriter(file)
		defer func() {
			file.Close()
		}()
		for {
			totalsize := 0
			for _, s := range DownloadedFileSize {
				totalsize += s
			}
			throughput := float64(totalsize) / 1024 / 1024 / (timeUnit.Seconds())

			//time throughput(MB/s) count averageLatency(ms) 99-percentile Latency
			line := fmt.Sprintf("%s %f %d %f %f\n", time.Now().String(), throughput, AvgDownloadLatency.Count(), AvgDownloadLatency.Mean()/MS, AvgDownloadLatency.Percentile(0.99)/MS)
			_, err := write.WriteString(line)
			if err != nil {
				fmt.Printf("failed to write string to log file, %s\n", err.Error())
			}
			write.Flush()

			DownloadedFileSize = []int{}
			AvgDownloadLatency = metrics.NewTimer()
			metrics.Register("AvgDownloadLatency", AvgDownloadLatency)
			time.Sleep(timeUnit)
		}

	}()
}

func TimersInit() {
	if !CMD_EnableMetrics {
		return
	}
	MetricsStartTime = time.Now()
	for i := 0; i < pinNumber; i++ {
		pin := metrics.NewTimer()
		metrics.Register("pin"+string(rune(i)), pin)
		TimerPin = append(TimerPin, pin)
	}

	AddTimer = metrics.NewTimer()
	metrics.Register("Add", AddTimer)

	Provide = metrics.NewTimer()
	metrics.Register("Provide", Provide)

	Persist = metrics.NewTimer()
	metrics.Register("Persist-Flush", Persist)

	Dag = metrics.NewTimer()
	metrics.Register("dag", Dag)

	FlatfsHasTimer = metrics.NewTimer()
	metrics.Register("flafshas", FlatfsHasTimer)

	FlatfsPut = metrics.NewTimer()
	metrics.Register("flatfsPut", FlatfsPut)

	SyncTime = metrics.NewTimer()
	metrics.Register("SyncTime", SyncTime)
	DeduplicateOverhead = metrics.NewTimer()
	metrics.Register("DeduplicateOverhead", DeduplicateOverhead)

	BDMonitor = Newmonitor()
	BlockServiceTime = metrics.NewTimer()
	metrics.Register("BlockServiceTime", BlockServiceTime)
	RootNeighbourAskingTime = metrics.NewTimer()
	metrics.Register("RootNeighbourAskingTime", RootNeighbourAskingTime)
	RootFindProviderTime = metrics.NewTimer()
	metrics.Register("RootFindProviderTime", RootFindProviderTime)
	RootWaitToWantTime = metrics.NewTimer()
	metrics.Register("RootWaitToWantTime", RootWaitToWantTime)
	LeafWaitToWantTime = metrics.NewTimer()
	metrics.Register("LeafWaitToWantTime", LeafWaitToWantTime)
	BitswapTime = metrics.NewTimer()
	metrics.Register("BitswapTime", BitswapTime)
	PutStoreTime = metrics.NewTimer()
	metrics.Register("PutStoreTime", PutStoreTime)
	VisitTime = metrics.NewTimer()
	metrics.Register("VisitTime", VisitTime)

	RealGet = metrics.NewTimer()
	metrics.Register("RealGet", RealGet)
	ModelGet = metrics.NewTimer()
	metrics.Register("ModelGet", ModelGet)

	Sample = metrics.NewUniformSample(102400)
	Variance = metrics.NewHistogram(Sample)
	metrics.Register("Variance", Variance)

	GetNode = metrics.NewTimer()
	metrics.Register("GetNode", GetNode)
	WriteTo = metrics.NewTimer()
	metrics.Register("WriteTo", WriteTo)

	BlocksRedundant = metrics.NewHistogram(metrics.NewExpDecaySample(1028, 0.015))
	metrics.Register("BlocksRedundant", BlocksRedundant)
	RequestsRedundant = metrics.NewHistogram(metrics.NewExpDecaySample(1028, 0.015))
	metrics.Register("RequestsRedundant", RequestsRedundant)

	ChoosePeer = metrics.NewTimer()
	metrics.Register("ChoosePeer", ChoosePeer)
	DailPeer = metrics.NewTimer()
	metrics.Register("DailPeer", DailPeer)
	ResponsePeer = metrics.NewTimer()
	metrics.Register("ResponsePeer", ResponsePeer)

	RealFindProvider = metrics.NewTimer()
	metrics.Register("RealFindProvider", RealFindProvider)
	ModelFindProvider = metrics.NewTimer()
	metrics.Register("ModelFindProvider", ModelFindProvider)

	Samplefp = metrics.NewUniformSample(102400)
	FPVariance = metrics.NewHistogram(Samplefp)
	metrics.Register("FPVariance", FPVariance)

	Samplefpn = metrics.NewUniformSample(102400)
	FPInner = metrics.NewHistogram(Samplefpn)
	metrics.Register("FPInner", FPInner)

	FPMonitor = NewFPMonitor()
	//go metrics.Log(metrics.DefaultRegistry, 1 * time.Second,log.New(os.Stdout, "metrics: ", log.Lmicroseconds))

	ProvideTime = metrics.NewTimer()
	metrics.Register("Provide", ProvideTime)
	SuccessfullyProvide = 0
	StartBackProvideTime = ZeroTime

	// 假设一个cacheline 需要 200 个字节，那么我们让最多设置 5e6 个 cacheline
	// 此时需要 1GB 内存，为了测试方便，我们先设置如上个数
	GPeerRH = NewPeerRH(1, B, 5*1e6) // 历史信息不起作用
	//GPeerRH = NewPeerRH(1, 1) // 历史信息与逻辑距离 1:1

	DataStorePut = metrics.NewHistogram(metrics.NewExpDecaySample(1028, 0.015))

}

const MS = 1000000

func CollectMonitor() {
	if !CMD_EnableMetrics {
		return
	}
	//BDMonitor.TimeStamps()

	//fmt.Printf("total fetch %d\n", BDMonitor.TotalFetches)
	bd := BDMonitor.GetBreakdown()
	for _, t := range bd.BlockService {
		BlockServiceTime.Update(t)
	}
	RootNeighbourAskingTime.Update(bd.NeighbourAsk)
	RootFindProviderTime.Update(bd.FindProvider)
	RootWaitToWantTime.Update(bd.RootWaitToWant)
	for _, t := range bd.LeafWaitToWant {
		LeafWaitToWantTime.Update(t)
	}
	for _, t := range bd.Bitswap {
		BitswapTime.Update(t)
	}
	for _, t := range bd.PutStore {
		PutStoreTime.Update(t)
	}
	for _, t := range bd.Visit {
		VisitTime.Update(t)
	}

	realtime := BDMonitor.RealTime()
	modeltime := BDMonitor.ModeledTime()
	v := int64(((realtime.Seconds() - modeltime.Seconds()) / realtime.Seconds()) * 1000000000) //Histogram requires int data
	RealGet.Update(realtime)
	ModelGet.Update(modeltime)
	Variance.Update(v)

	RequestsRedundant.Update(int64(BDMonitor.SumReqsRedundant()))
	BlocksRedundant.Update(int64(BDMonitor.SumBlksRedundant()))

	BDMonitor = Newmonitor()
}

func (m *FindProviderMonitor) CollectFPMonitor() {
	if !CMD_EnableMetrics {
		return
	}
	m.EventList.Range(func(key, value interface{}) bool {
		target := key.(string)
		pe := value.(*ProviderEvent)
		//if got providers for this block
		pe.FirstGotProviderFrom.Range(func(key, value interface{}) bool {
			var peerChoose []time.Duration
			var peerDail []time.Duration
			var peerResponse []time.Duration

			// walk through critical path
			current := value.(string)
			//fmt.Printf("SeedPeers: %v\n", pe.SeedPeers)
			//fmt.Printf("Critical Path for %s: \n", target)
			for true {
				//m.PeerTimePrint(target, current)
				father := ""
				if !m.IsSeedPeers(target, current) {
					if f, ok := pe.FirstGotCloserFrom.Load(current); ok {
						father = f.(string)
						responseFather, ok1 := pe.FirstResponseTime.Load(father)
						queryCurrent, ok2 := pe.FirstQueryTime.Load(current)
						if ok1 && ok2 && queryCurrent.(time.Time).After(responseFather.(time.Time)) {
							peerChoose = append(peerChoose, queryCurrent.(time.Time).Sub(responseFather.(time.Time)))
						}
					}
				}
				queryCurrent, ok2 := pe.FirstQueryTime.Load(current)
				requestCurrent, ok3 := pe.FirstRequestTime.Load(current)
				responseCurrent, ok4 := pe.FirstResponseTime.Load(current)
				if ok2 && ok3 && requestCurrent.(time.Time).After(queryCurrent.(time.Time)) {
					peerDail = append(peerDail, requestCurrent.(time.Time).Sub(queryCurrent.(time.Time)))
				}
				if ok3 && ok4 && responseCurrent.(time.Time).After(requestCurrent.(time.Time)) {
					peerResponse = append(peerResponse, responseCurrent.(time.Time).Sub(requestCurrent.(time.Time)))
				}

				if father == "" {
					break
				}
				current = father
			}

			var peerChooseTime time.Duration
			var peerDailTime time.Duration
			var peerResponseTime time.Duration
			var modeltime time.Duration
			var realtime time.Duration
			innernodes := len(peerResponse)
			for _, d := range peerChoose {
				peerChooseTime = time.Duration(peerChooseTime.Nanoseconds() + d.Nanoseconds())
				modeltime = time.Duration(modeltime.Nanoseconds() + d.Nanoseconds())
			}
			for _, d := range peerDail {
				peerDailTime = time.Duration(peerDailTime.Nanoseconds() + d.Nanoseconds())
				modeltime = time.Duration(modeltime.Nanoseconds() + d.Nanoseconds())
			}
			for _, d := range peerResponse {
				peerResponseTime = time.Duration(peerResponseTime.Nanoseconds() + d.Nanoseconds())
				modeltime = time.Duration(modeltime.Nanoseconds() + d.Nanoseconds())
			}

			modeltime = time.Duration(modeltime.Nanoseconds() + pe.FinishLocalSearch.Sub(pe.FindProviderAsync).Nanoseconds())

			ChoosePeer.Update(peerChooseTime)
			DailPeer.Update(peerDailTime)
			ResponsePeer.Update(peerResponseTime)
			ModelFindProvider.Update(modeltime)
			FPInner.Update(int64(innernodes))
			if outputProviderTime, ok := pe.FirstOutputProviderTime.Load(key.(string)); ok {
				realtime = outputProviderTime.(time.Time).Sub(pe.FindProviderAsync)
				RealFindProvider.Update(realtime)
			}
			v := int64(((realtime.Seconds() - modeltime.Seconds()) / realtime.Seconds()) * 1000000000)
			FPVariance.Update(v)

			return true
		})

		return true
	})

	newFPM := NewFPMonitor()
	FPMonitor = newFPM
}
func OutputMetrics0() {
	if !CMD_EnableMetrics {
		return
	}
	for i := 0; i < pinNumber; i++ {
		fmt.Printf("TimerPin-%d: %d ,     avg- %f ms, 0.9p- %f ms 0.1p- %f \n", i, TimerPin[i].Count(), TimerPin[i].Mean()/MS, TimerPin[i].Percentile(0.9)/MS, TimerPin[i].Percentile(0.1)/MS)
	}

	for _, t := range Times {
		fmt.Printf("%f\n", t.Seconds())
	}
}

func Output_addBreakdown() {
	if !CMD_EnableMetrics {
		return
	}
	fmt.Println("-------------------------ADD-------------------------")
	fmt.Printf("		avg(ms)    0.9p(ms)\n")
	fmt.Printf("AddTimer: %d %f %f\n", AddTimer.Count(), AddTimer.Mean()/MS, AddTimer.Percentile(0.999)/MS)
	fmt.Printf("Provide: %d %f %f\n", Provide.Count(), Provide.Mean()/MS, Provide.Percentile(0.999)/MS)
	fmt.Printf("Persist: %d %f %f\n", Persist.Count(), Persist.Mean()/MS, Persist.Percentile(0.999)/MS)
	fmt.Printf("Dag&Other: %d %f %f\n", Dag.Count(), Dag.Mean()/MS, Dag.Percentile(0.999)/MS)
	if Persist.Count() == 0 {
		return
	}
	fmt.Printf("SyncTime: %d ,     avg- %f ms, 0.9p- %f ms \n", SyncTime.Count()/Persist.Count(), SyncTime.Mean()/MS, SyncTime.Percentile(0.9)/MS)
	fmt.Printf("DeduplicateOverhead: %d ,     avg- %f ms, 0.9p- %f ms \n", DeduplicateOverhead.Count()/Persist.Count(), DeduplicateOverhead.Mean()/MS, DeduplicateOverhead.Percentile(0.9)/MS)
	//fmt.Printf("FlatfsPut: %d ,     avg- %f, 0.9p- %f \n", FlatfsPut.Count(), FlatfsPut.Mean()/MS, FlatfsPut.Percentile(float64(FlatfsPut.Count())*0.9)/MS)
}

func Output_Get() {
	if !CMD_EnableMetrics {
		return
	}
	fmt.Println("-------------------------GET-------------------------")
	fmt.Printf(" BlockServiceTime: %d ,     avg- %f ms, 0.9p- %f ms \n", BlockServiceTime.Count(), BlockServiceTime.Mean()/MS, BlockServiceTime.Percentile(0.9)/MS)
	fmt.Printf(" RootNeighbourAskingTime: %d ,     avg- %f ms, 0.9p- %f ms \n", RootNeighbourAskingTime.Count(), RootNeighbourAskingTime.Mean()/MS, RootNeighbourAskingTime.Percentile(0.9)/MS)
	fmt.Printf(" RootFindProviderTime: %d ,     avg- %f ms, 0.9p- %f ms \n", RootFindProviderTime.Count(), RootFindProviderTime.Mean()/MS, RootFindProviderTime.Percentile(0.9)/MS)
	fmt.Printf(" RootWaitToWantTime: %d ,     avg- %f ms, 0.9p- %f ms \n", RootWaitToWantTime.Count(), RootWaitToWantTime.Mean()/MS, RootWaitToWantTime.Percentile(0.9)/MS)
	fmt.Printf(" LeafWaitToWantTime: %d ,     avg- %f ms, 0.9p- %f ms \n", LeafWaitToWantTime.Count(), LeafWaitToWantTime.Mean()/MS, LeafWaitToWantTime.Percentile(0.9)/MS)
	fmt.Printf(" BitswapTime: %d ,     avg- %f ms, 0.9p- %f ms \n", BitswapTime.Count(), BitswapTime.Mean()/MS, BitswapTime.Percentile(0.9)/MS)
	fmt.Printf(" PutStoreTime: %d ,     avg- %f ms, 0.9p- %f ms \n", PutStoreTime.Count(), PutStoreTime.Mean()/MS, PutStoreTime.Percentile(0.9)/MS)
	fmt.Printf(" VisitTime: %d ,     avg- %f ms, 0.9p- %f ms \n", VisitTime.Count(), VisitTime.Mean()/MS, VisitTime.Percentile(0.9)/MS)

	fmt.Printf(" RealGet: %d ,     avg- %f ms, 0.9p- %f ms \n", RealGet.Count(), RealGet.Mean()/MS, RealGet.Percentile(0.9)/MS)
	fmt.Printf(" ModelGet: %d ,     avg- %f ms, 0.9p- %f ms \n", ModelGet.Count(), ModelGet.Mean()/MS, ModelGet.Percentile(0.9)/MS)
	fmt.Printf(" Variance: %d ,     avg- %f, 0.9p- %f \n", Variance.Count(), Variance.Mean()/1000000000, Variance.Percentile(0.9)/1000000000)

	fmt.Printf(" GetNode: %d ,     avg- %f ms, 0.9p- %f ms \n", GetNode.Count(), GetNode.Mean()/MS, GetNode.Percentile(0.9)/MS)
	fmt.Printf(" WriteTo: %d ,     avg- %f ms, 0.9p- %f ms \n", WriteTo.Count(), WriteTo.Mean()/MS, WriteTo.Percentile(0.9)/MS)

	fmt.Printf(" BlocksRedundant: %d,     avg- %f, 0.9p- %f\n", BlocksRedundant.Sum(), BlocksRedundant.Mean(), BlocksRedundant.Percentile(0.9))
	fmt.Printf(" RequestsRedundant: %d,     avg- %f, 0.9p- %f\n", RequestsRedundant.Sum(), RequestsRedundant.Mean(), RequestsRedundant.Percentile(0.9))

	if PutStoreTime.Count() == 0 {
		return
	}
	fmt.Printf("SyncTime: %d ,     avg- %f ms, 0.9p- %f ms \n", SyncTime.Count()/RealGet.Count(), SyncTime.Mean()/MS, SyncTime.Percentile(0.9)/MS)
	fmt.Printf("DeduplicateOverhead: %d ,     avg- %f ms, 0.9p- %f ms \n", DeduplicateOverhead.Count()/RealGet.Count(), DeduplicateOverhead.Mean()/MS, DeduplicateOverhead.Percentile(0.9)/MS)

}

func Output_PeerRH() {

	hit := GPeerRH.hit.Count()
	miss := GPeerRH.miss.Count()

	fmt.Println("-------------------------PeerResponseHistory-------------------------")
	fmt.Printf("PRH: FindPeerHistory hit %v, miss %v\n", hit, miss)
	fmt.Printf("PRH: FindPeerHistory hit rate : %v\n", float64(hit)/float64(hit+miss))
	fmt.Printf("PRH: Cache size %v\n", GPeerRH.lruCache.Len())
	fmt.Printf("PRH: Compromise %v, AllCmp %v, compromise rate: %v\n", GPeerRH.Compromise.Count(),
		GPeerRH.AllCmp.Count(), float64(GPeerRH.Compromise.Count())/float64(GPeerRH.AllCmp.Count()))
}

func Output_FP() {
	if !CMD_EnableMetrics {
		return
	}
	fmt.Println("-------------------------FindProvider-------------------------")
	fmt.Printf(" ChoosePeer: %d ,     avg- %f ms, 0.9p- %f ms \n", ChoosePeer.Count(), ChoosePeer.Mean()/MS, ChoosePeer.Percentile(0.9)/MS)
	fmt.Printf(" DailPeer: %d ,     avg- %f ms, 0.9p- %f ms \n", DailPeer.Count(), DailPeer.Mean()/MS, DailPeer.Percentile(0.9)/MS)
	fmt.Printf(" ResponsePeer: %d ,     avg- %f ms, 0.9p- %f ms \n", ResponsePeer.Count(), ResponsePeer.Mean()/MS, ResponsePeer.Percentile(0.9)/MS)
	fmt.Printf(" RealFindProvider: %d ,     avg- %f ms, 0.9p- %f ms \n", RealFindProvider.Count(), RealFindProvider.Mean()/MS, RealFindProvider.Percentile(0.9)/MS)
	fmt.Printf(" ModelFindProvider: %d ,     avg- %f ms, 0.9p- %f ms \n", ModelFindProvider.Count(), ModelFindProvider.Mean()/MS, ModelFindProvider.Percentile(0.9)/MS)

	fmt.Printf(" FPInnerNodes: %d ,     avg- %f, 0.9p- %f \n", FPInner.Count(), FPInner.Mean(), FPInner.Percentile(0.9))
	fmt.Printf(" FPVariance: %d ,     avg- %f, 0.9p- %f \n", FPVariance.Count(), FPVariance.Mean()/1000000000, FPVariance.Percentile(0.9)/1000000000)

	fmt.Printf("DataStore Put total size: %f MB, rate: %f MB/s\n", float64(DataStorePut.Sum())/1024/1024, float64(DataStorePut.Sum())/1024/1024/(time.Now().Sub(MetricsStartTime).Seconds()))
}

func call(skip int) {
	pc, file, line, _ := runtime.Caller(skip)
	pcName := runtime.FuncForPC(pc).Name()
	fmt.Println(fmt.Sprintf("%s   %d   %s", file, line, pcName))
}

func PrintStack(toUP int) {
	fmt.Println("-----------------------")
	for i := 2; i < toUP+2; i++ {
		call(i)
	}
	fmt.Println("-----------------------")
}

// BackGround running metrics report

var RecvFrom sync.Map
var provided int

func RecordRecv(cid, peer string) {
	if v, ok := RecvFrom.Load(peer); ok {
		copyCount := v.(int)
		copyCount++
		RecvFrom.Store(peer, copyCount)
	} else {
		RecvFrom.Store(peer, 1)
	}
	//fmt.Printf("recv %s from %s\n", cid, peer)
}
func RecordProvide(cid string) {
	provided++
}

func StartBackReport() {
	dur := time.Minute
	go func() {
		for {
			time.Sleep(dur)

			fmt.Printf("%s Report: \n", time.Now().String())
			fmt.Printf("    recvFrom:\n")
			RecvFrom.Range(func(key, value interface{}) bool {
				p := key.(string)
				c := value.(int)
				fmt.Printf("        %s %d\n", p, c)
				return true
			})
			RecvFrom = sync.Map{}

			fmt.Printf("    Provide: %d\n", provided)
			provided = 0
		}
	}()
}

func StandardOutput(function string, t metrics.Timer, filesize int) string {
	throughput := float64(filesize) / 1024 / 1024 / (t.Mean() / 1000000000)
	return fmt.Sprintf("%f %f %f\n", throughput, t.Mean()/1000000, t.Percentile(0.99)/MS)
	// return fmt.Sprintf("%s: %d files, average latency: %f ms, 0.99P latency: %f ms\n", function, t.Count(), t.Mean()/MS, t.Percentile(float64(t.Count())*0.99)/MS)
}

func StramLevelDBStats(stat leveldb.DBStats) {
	fmt.Printf("---------------------------------------------------------\n")
	fmt.Printf("WriteDelayCount: %d\n", stat.WriteDelayCount)
	fmt.Printf("WriteDelayDuration: %s\n", stat.WriteDelayDuration.String())
	fmt.Printf("AliveSnapshots: %d\n", stat.AliveSnapshots)
	fmt.Printf("AliveIterators: %d\n", stat.AliveIterators)
	fmt.Printf("IOWrite: %d\n", stat.IOWrite)
	fmt.Printf("IORead: %d\n", stat.IORead)
	fmt.Printf("BlockCacheSize: %d\n", stat.BlockCacheSize)
	fmt.Printf("OpenedTablesCount: %d\n", stat.OpenedTablesCount)
	fmt.Printf("LevelSizes: %v\n", stat.LevelSizes)
	fmt.Printf("LevelTablesCounts: %v\n", stat.LevelTablesCounts)
	fmt.Printf("LevelRead: %v\n", stat.LevelRead)
	fmt.Printf("LevelWrite: %v\n", stat.LevelWrite)
	fmt.Printf("LevelDurations: %v\n", stat.LevelDurations)
	fmt.Printf("----------------------------------------------------------\n")
}
