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

// Get Metrics

var BDMonitor *Monitor

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

	BDMonitor = Newmonitor()

	//go metrics.Log(metrics.DefaultRegistry, 1 * time.Second,log.New(os.Stdout, "metrics: ", log.Lmicroseconds))

}

const MS = 1000000

func OutputMetrics0() {
	for i := 0; i < pinNumber; i++ {
		fmt.Printf("TimerPin-%d: %d ,     avg- %f ms, 0.9p- %f ms \n", i, TimerPin[i].Count(), TimerPin[i].Mean()/MS, TimerPin[i].Percentile(float64(TimerPin[i].Count())*0.9)/MS)
	}
	fmt.Println("-----------Add-----------------------------")
	fmt.Printf("Add: %d ,     avg- %f ms, 0.9p- %f ms \n", AddTimer.Count(), AddTimer.Mean()/MS, AddTimer.Percentile(float64(AddTimer.Count())*0.9)/MS)
	fmt.Printf("Provide: %d ,     avg- %f ms, 0.9p- %f ms \n", Provide.Count(), Provide.Mean()/MS, Provide.Percentile(float64(Provide.Count())*0.9)/MS)
	fmt.Printf("Persist: %d ,     avg- %f ms, 0.9p- %f ms \n", Persist.Count(), Persist.Mean()/MS, Persist.Percentile(float64(Persist.Count())*0.9)/MS)
	fmt.Printf("Dag: %d ,     avg- %f ms, 0.9p- %f ms \n", Dag.Count(), Dag.Mean()/MS, Dag.Percentile(float64(Dag.Count())*0.9)/MS)
	fmt.Printf("HasTimer: %d ,     avg- %f ms, 0.9p- %f ms \n", FlatfsHasTimer.Count(), FlatfsHasTimer.Mean()/MS, FlatfsHasTimer.Percentile(float64(FlatfsHasTimer.Count())*0.9)/MS)
	fmt.Printf("FlatfsPut: %d ,     avg- %f ms, 0.9p- %f ms \n", FlatfsPut.Count(), FlatfsPut.Mean()/MS, FlatfsPut.Percentile(float64(FlatfsPut.Count())*0.9)/MS)
	fmt.Println("------------Get----------------------------")
	
}

func Output_addBreakdown() {
	fmt.Printf("avg(ms)    0.9p(ms)\n")
	fmt.Printf("%f %f\n", Provide.Mean()/MS, Provide.Percentile(float64(Provide.Count())*0.9)/MS)
	fmt.Printf("%f %f\n", Persist.Mean()/MS, Persist.Percentile(float64(Persist.Count())*0.9)/MS)
	fmt.Printf("%f %f\n", Dag.Mean()/MS, Dag.Percentile(float64(Dag.Count())*0.9)/MS)
}

func Output_Get_SingleFile() {
	fmt.Printf("root: %.2f %.2f %.2f %.2f %.2f %.2f %.2f\n",
		BDMonitor.RootBlockServiceTime().Seconds()*1000,
		BDMonitor.RootNeighbourAskingTime().Seconds()*1000,
		BDMonitor.RootFindProviderTime().Seconds()*1000,
		BDMonitor.RootWaitToWantTime().Seconds()*1000,
		BDMonitor.RootBitswapTime().Seconds()*1000,
		BDMonitor.RootBeforeVisitTime().Seconds()*1000,
		BDMonitor.RootVisitTime().Seconds()*1000)
	fmt.Printf("leaf node average(%d): %.2f %.2f %.2f %.2f %.2f\n", BDMonitor.LeafNumber(),
		BDMonitor.AvgLeafBlockServiceTime().Seconds()*1000,
		BDMonitor.AvgLeafWaitToWantTime().Seconds()*1000,
		BDMonitor.AvgLeafBitswapTime().Seconds()*1000,
		BDMonitor.AvgLeafBeforeVisitTime().Seconds()*1000,
		BDMonitor.AvgLeafVisitTime().Seconds()*1000)
	realtime := BDMonitor.RealTime().Seconds()
	modeltime := BDMonitor.ModeledTime().Seconds()
	v := (realtime - modeltime) / realtime
	fmt.Printf("real time: %.2f. modeled time: %.2f. variance: %.2f\n", realtime*1000, modeltime*1000, v)
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
