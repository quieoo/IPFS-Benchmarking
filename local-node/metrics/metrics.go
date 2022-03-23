package metrics

import (
	"fmt"
	"github.com/rcrowley/go-metrics"
	"runtime"
	"time"
)

// some global flag

var CMD_CloseBackProvide = true
var CMD_CloseLANDHT = false
var CMD_CloseDHTRefresh = false
var CMD_CloseAddProvide = false
var CMD_ProvideFirst = true
var CMD_EnableMetrics = true
var CMD_StallAfterUpload = false

var TimerPin []metrics.Timer
var pinNumber = 2
var TimePin time.Time

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

//findProvider metrics
var FPMonitor *FindProviderMonitor
var DHTChoose metrics.Timer
var DHRResponse metrics.Timer
var DHTReplace metrics.Timer
var RealFindProvider metrics.Timer
var ModelFindProvider metrics.Timer
var Samplefp metrics.Sample
var FPVariance metrics.Histogram
var Samplefpn metrics.Sample
var FPInner metrics.Histogram

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

func TimersInit() {
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

	DHTChoose = metrics.NewTimer()
	metrics.Register("DHTChoose", DHTChoose)
	DHRResponse = metrics.NewTimer()
	metrics.Register("DHRResponse", DHRResponse)
	DHTReplace = metrics.NewTimer()
	metrics.Register("DHTReplace", DHTReplace)

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

}

const MS = 1000000

func CollectMonitor() {
	if !CMD_EnableMetrics {
		return
	}
	//BDMonitor.TimeStamps()

	BlockServiceTime.Update(time.Duration(int64(BDMonitor.TotalFetches) * BDMonitor.AvgBlockServiceTime().Nanoseconds()))
	RootNeighbourAskingTime.Update(BDMonitor.RootNeighbourAskingTime())
	RootFindProviderTime.Update(BDMonitor.RootFindProviderTime())
	RootWaitToWantTime.Update(BDMonitor.RootWaitToWantTime())
	LeafWaitToWantTime.Update(time.Duration(int64(BDMonitor.TotalFetches-1) * BDMonitor.AvgLeafWaitToWantTime().Nanoseconds()))
	BitswapTime.Update(time.Duration(int64(BDMonitor.TotalFetches) * BDMonitor.AvgBitswapTime().Nanoseconds()))
	PutStoreTime.Update(time.Duration(int64(BDMonitor.TotalFetches) * BDMonitor.AvgPutToStore().Nanoseconds()))
	VisitTime.Update(time.Duration(int64(BDMonitor.TotalFetches) * BDMonitor.AvgVisitTime().Nanoseconds()))

	realtime := BDMonitor.RealTime()
	modeltime := BDMonitor.ModeledTime()
	v := int64(((realtime.Seconds() - modeltime.Seconds()) / realtime.Seconds()) * 1000000000) //Histogram requires int data
	RealGet.Update(realtime)
	ModelGet.Update(modeltime)
	Variance.Update(v)

	BDMonitor = Newmonitor()
}

func (m *FindProviderMonitor) CollectFPMonitor() {
	if !CMD_EnableMetrics {
		return
	}
	m.EventList.Range(func(key, value interface{}) bool {
		pe := value.(*ProviderEvent)
		//if got providers for this block
		pe.FirstGotProviderFrom.Range(func(key, value interface{}) bool {
			var dhtchoose []time.Duration
			var dhtresponse []time.Duration
			var dhtreplace []time.Duration

			// walk through critical path
			cur := key.(string)
			father := value.(string)
			gotprovider := true
			for true {
				requestFather, ok1 := pe.FirstRequestTime.Load(father)
				findFather, ok2 := m.FirstFindPeerTime.Load(father)
				responseFather, ok3 := pe.FirstResponseTime.Load(father)
				findcur, ok4 := m.FirstFindPeerTime.Load(cur)
				//fmt.Printf("find cur %s: %s\n", cur, findcur.(time.Time).String())
				//fmt.Printf("response from %s: %s\n", father, responseFather.(time.Time).String())
				//fmt.Printf("send request to %s: %s\n", father, requestFather.(time.Time).String())
				//fmt.Printf("find father %s: %s\n", father, findFather.(time.Time).String())
				//the first round is the provider-provider giving provider, we don't count it as replace time
				if gotprovider {
					gotprovider = false
				} else {
					if ok3 && ok4 {
						dhtreplace = append(dhtreplace, findcur.(time.Time).Sub(responseFather.(time.Time)))
						//fmt.Printf("replace: %f\n", findcur.(time.Time).Sub(responseFather.(time.Time)).Seconds())
					}
				}
				if ok1 && ok3 {
					dhtresponse = append(dhtresponse, responseFather.(time.Time).Sub(requestFather.(time.Time)))
					//fmt.Printf("response: %f\n", responseFather.(time.Time).Sub(requestFather.(time.Time)).Seconds())
				}
				//reach the peer who we know it before call for Async call. We get this peer from local routing table
				if ok2 && findFather.(time.Time).Before(pe.FindProviderAsync) {
					dhtchoose = append(dhtchoose, requestFather.(time.Time).Sub(pe.FindProviderAsync))
					//fmt.Printf("choose %f\n", requestFather.(time.Time).Sub(pe.FindProviderAsync).Seconds())
					break
				}
				if ok1 && ok2 {
					//sometimes (the first intermediate node), we send request to it before we first find it
					if requestFather.(time.Time).After(findFather.(time.Time)) {
						dhtchoose = append(dhtchoose, requestFather.(time.Time).Sub(findFather.(time.Time)))
						//fmt.Printf("choose %f\n", requestFather.(time.Time).Sub(findFather.(time.Time)).Seconds())
					}
				}
				//检查log，看一下是否以上公式能够永远成立， 出现负数？
				cur = father
				if newf, ok := pe.FirstGotCloserFrom.Load(father); ok {
					father = newf.(string)
				} else {
					break
				}
			}
			//fmt.Printf("%s: choose %d, response %d, replace %d\n", pe.c, len(dhtchoose), len(dhtresponse), len(dhtreplace))
			var dhtchoosetime time.Duration
			var dhtresponsetime time.Duration
			var dhtreplacetime time.Duration
			var modeltime time.Duration
			var realtime time.Duration
			innernodes := 0
			for _, d := range dhtchoose {
				dhtchoosetime = time.Duration(dhtchoosetime.Nanoseconds() + d.Nanoseconds())
				modeltime = time.Duration(modeltime.Nanoseconds() + d.Nanoseconds())
			}
			for _, d := range dhtresponse {
				dhtresponsetime = time.Duration(dhtresponsetime.Nanoseconds() + d.Nanoseconds())
				modeltime = time.Duration(modeltime.Nanoseconds() + d.Nanoseconds())
				innernodes++
			}
			for _, d := range dhtreplace {
				dhtreplacetime = time.Duration(dhtreplacetime.Nanoseconds() + d.Nanoseconds())
				modeltime = time.Duration(modeltime.Nanoseconds() + d.Nanoseconds())
			}
			modeltime = time.Duration(modeltime.Nanoseconds() + pe.FinishLocalSearch.Sub(pe.FindProviderAsync).Nanoseconds())

			DHTChoose.Update(dhtchoosetime)
			DHTReplace.Update(dhtreplacetime)
			DHRResponse.Update(dhtresponsetime)
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

	//inherit peer find time, because unlike other metrics which only live during this provider-finding period, this one is effective for all files
	newFPM := NewFPMonitor()
	newFPM.InheritFindPeer(FPMonitor)
	FPMonitor = newFPM
}
func OutputMetrics0() {
	for i := 0; i < pinNumber; i++ {
		fmt.Printf("TimerPin-%d: %d ,     avg- %f ms, 0.9p- %f ms \n", i, TimerPin[i].Count(), TimerPin[i].Mean()/MS, TimerPin[i].Percentile(float64(TimerPin[i].Count())*0.9)/MS)
	}
}

func Output_addBreakdown() {
	fmt.Println("-------------------------ADD-------------------------")
	fmt.Printf("		avg(ms)    0.9p(ms)\n")
	fmt.Printf("Provide: %f %f\n", Provide.Mean()/MS, Provide.Percentile(float64(Provide.Count())*0.9)/MS)
	fmt.Printf("Persist: %f %f\n", Persist.Mean()/MS, Persist.Percentile(float64(Persist.Count())*0.9)/MS)
	fmt.Printf("Dag: %f %f\n", Dag.Mean()/MS, Dag.Percentile(float64(Dag.Count())*0.9)/MS)
	if Persist.Count() == 0 {
		return
	}
	fmt.Printf("SyncTime: %d ,     avg- %f ms, 0.9p- %f ms \n", SyncTime.Count()/Persist.Count(), SyncTime.Mean()/MS, SyncTime.Percentile(float64(SyncTime.Count())*0.9)/MS)
	fmt.Printf("DeduplicateOverhead: %d ,     avg- %f ms, 0.9p- %f ms \n", DeduplicateOverhead.Count()/Persist.Count(), DeduplicateOverhead.Mean()/MS, DeduplicateOverhead.Percentile(float64(DeduplicateOverhead.Count())*0.9)/MS)
}

func Output_Get() {
	fmt.Println("-------------------------GET-------------------------")
	fmt.Printf(" BlockServiceTime: %d ,     avg- %f ms, 0.9p- %f ms \n", BlockServiceTime.Count(), BlockServiceTime.Mean()/MS, BlockServiceTime.Percentile(float64(BlockServiceTime.Count())*0.9)/MS)
	fmt.Printf(" RootNeighbourAskingTime: %d ,     avg- %f ms, 0.9p- %f ms \n", RootNeighbourAskingTime.Count(), RootNeighbourAskingTime.Mean()/MS, RootNeighbourAskingTime.Percentile(float64(RootNeighbourAskingTime.Count())*0.9)/MS)
	fmt.Printf(" RootFindProviderTime: %d ,     avg- %f ms, 0.9p- %f ms \n", RootFindProviderTime.Count(), RootFindProviderTime.Mean()/MS, RootFindProviderTime.Percentile(float64(RootFindProviderTime.Count())*0.9)/MS)
	fmt.Printf(" RootWaitToWantTime: %d ,     avg- %f ms, 0.9p- %f ms \n", RootWaitToWantTime.Count(), RootWaitToWantTime.Mean()/MS, RootWaitToWantTime.Percentile(float64(RootWaitToWantTime.Count())*0.9)/MS)
	fmt.Printf(" LeafWaitToWantTime: %d ,     avg- %f ms, 0.9p- %f ms \n", LeafWaitToWantTime.Count(), LeafWaitToWantTime.Mean()/MS, LeafWaitToWantTime.Percentile(float64(LeafWaitToWantTime.Count())*0.9)/MS)
	fmt.Printf(" BitswapTime: %d ,     avg- %f ms, 0.9p- %f ms \n", BitswapTime.Count(), BitswapTime.Mean()/MS, BitswapTime.Percentile(float64(BitswapTime.Count())*0.9)/MS)
	fmt.Printf(" PutStoreTime: %d ,     avg- %f ms, 0.9p- %f ms \n", PutStoreTime.Count(), PutStoreTime.Mean()/MS, PutStoreTime.Percentile(float64(PutStoreTime.Count())*0.9)/MS)
	fmt.Printf(" VisitTime: %d ,     avg- %f ms, 0.9p- %f ms \n", VisitTime.Count(), VisitTime.Mean()/MS, VisitTime.Percentile(float64(VisitTime.Count())*0.9)/MS)

	fmt.Printf(" RealGet: %d ,     avg- %f ms, 0.9p- %f ms \n", RealGet.Count(), RealGet.Mean()/MS, RealGet.Percentile(float64(RealGet.Count())*0.9)/MS)
	fmt.Printf(" ModelGet: %d ,     avg- %f ms, 0.9p- %f ms \n", ModelGet.Count(), ModelGet.Mean()/MS, ModelGet.Percentile(float64(ModelGet.Count())*0.9)/MS)
	fmt.Printf(" Variance: %d ,     avg- %f, 0.9p- %f \n", Variance.Count(), Variance.Mean()/1000000000, Variance.Percentile(float64(Variance.Count())*0.9)/1000000000)

	fmt.Printf(" GetNode: %d ,     avg- %f ms, 0.9p- %f ms \n", GetNode.Count(), GetNode.Mean()/MS, GetNode.Percentile(float64(GetNode.Count())*0.9)/MS)
	fmt.Printf(" WriteTo: %d ,     avg- %f ms, 0.9p- %f ms \n", WriteTo.Count(), WriteTo.Mean()/MS, WriteTo.Percentile(float64(WriteTo.Count())*0.9)/MS)

}

func Output_FP() {
	fmt.Println("-------------------------FindProvider-------------------------")
	fmt.Printf(" DHTChoose: %d ,     avg- %f ms, 0.9p- %f ms \n", DHTChoose.Count(), DHTChoose.Mean()/MS, DHTChoose.Percentile(float64(DHTChoose.Count())*0.9)/MS)
	fmt.Printf(" DHRResponse: %d ,     avg- %f ms, 0.9p- %f ms \n", DHRResponse.Count(), DHRResponse.Mean()/MS, DHRResponse.Percentile(float64(DHRResponse.Count())*0.9)/MS)
	fmt.Printf(" DHTReplace: %d ,     avg- %f ms, 0.9p- %f ms \n", DHTReplace.Count(), DHTReplace.Mean()/MS, DHTReplace.Percentile(float64(DHTReplace.Count())*0.9)/MS)
	fmt.Printf(" RealFindProvider: %d ,     avg- %f ms, 0.9p- %f ms \n", RealFindProvider.Count(), RealFindProvider.Mean()/MS, RealFindProvider.Percentile(float64(RealFindProvider.Count())*0.9)/MS)
	fmt.Printf(" ModelFindProvider: %d ,     avg- %f ms, 0.9p- %f ms \n", ModelFindProvider.Count(), ModelFindProvider.Mean()/MS, ModelFindProvider.Percentile(float64(ModelFindProvider.Count())*0.9)/MS)

	fmt.Printf(" FPInnerNodes: %d ,     avg- %f, 0.9p- %f \n", FPInner.Count(), FPInner.Mean(), FPInner.Percentile(float64(FPInner.Count())*0.9))
	fmt.Printf(" FPVariance: %d ,     avg- %f, 0.9p- %f \n", FPVariance.Count(), FPVariance.Mean()/1000000000, FPVariance.Percentile(float64(FPVariance.Count())*0.9)/1000000000)

}

//--------------------------------TO-REMOVE----------------------------------------------------------------------

var AddTimeUse float64
var LastAverage float64

var FileNumber float64

var ProvideTimeUse float64
var StoreBlocksTimeUse float64
var AddfileTImeUse float64
var RestInAddAll float64
var BufferCommitTIme float64

func AddBreakDownInit() {
	FileNumber = 0
	LastAverage = 0
	AddTimeUse = 0
	ProvideTimeUse = 0
	StoreBlocksTimeUse = 0
	AddfileTImeUse = 0
	RestInAddAll = 0
	BufferCommitTIme = 0
}
func AddBreakDownSummery() {
	storeTime := RestInAddAll + StoreBlocksTimeUse + BufferCommitTIme
	merkleTime := AddfileTImeUse - StoreBlocksTimeUse - BufferCommitTIme
	fmt.Printf("add time: %f ms(%f), merkle dag time: %f ms, store blocks time: %f ms, provide time: %f ms\n", AddTimeUse/FileNumber, (AddTimeUse/FileNumber-LastAverage)/LastAverage, merkleTime/FileNumber, storeTime/FileNumber, ProvideTimeUse/FileNumber)
	LastAverage = AddTimeUse / FileNumber
}

var GetTimeUse float64
var ResolveTimeUse float64
var GetFileNumber int

func GetBreakDownInit() {
	GetTimeUse = 0
	ResolveTimeUse = 0
	GetFileNumber = 0
}
func standardOutput(function string, t metrics.Timer) string {
	return fmt.Sprintf("%s: %d files, average latency: %f ms, 0.99P latency: %f ms\n", function, t.Count(), t.Mean()/MS, t.Percentile(float64(t.Count())*0.99)/MS)
}
