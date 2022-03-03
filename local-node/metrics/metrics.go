package metrics

import (
	"fmt"
	"github.com/rcrowley/go-metrics"
	"runtime"
	"time"
)

var CMD_CloseBackProvide = true
var CMD_CloseLANDHT = false
var CMD_CloseDHTRefresh = false
var CMD_CloseAddProvide = false
var CMD_ImmeProvide = true
var CMD_EnableMetrics = true

var UploadTimer metrics.Timer
var AddTimer metrics.Timer
var Provide metrics.Timer
var Persist metrics.Timer
var Dag metrics.Timer
var HasTimer metrics.Timer

var HasDura time.Duration
var PersistDura time.Duration
var ProvideDura time.Duration
var AddDura time.Duration
var UploadDura time.Duration

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

	UploadTimer = metrics.NewTimer()
	metrics.Register("Upload", UploadTimer)

	AddTimer = metrics.NewTimer()
	metrics.Register("Add", AddTimer)

	Provide = metrics.NewTimer()
	metrics.Register("Provide", Provide)

	Persist = metrics.NewTimer()
	metrics.Register("Persist-Flush", Persist)

	Dag = metrics.NewTimer()
	metrics.Register("dag", Dag)

	HasTimer = metrics.NewTimer()
	metrics.Register("has", HasTimer)
	//go metrics.Log(metrics.DefaultRegistry, 1 * time.Second,log.New(os.Stdout, "metrics: ", log.Lmicroseconds))

}

const MS = 1000000

func OutputMetrics0() {

	//fmt.Printf(standardOutput("Upload", UploadTimer))
	//addtotal := float64(AddTimer.Sum())
	fmt.Printf("Upload: %d ,     avg- %f ms, 0.9p- %f ms \n", UploadTimer.Count(), UploadTimer.Mean()/MS, UploadTimer.Percentile(float64(UploadTimer.Count())*0.9)/MS)
	fmt.Printf("Add: %d ,     avg- %f ms, 0.9p- %f ms \n", AddTimer.Count(), AddTimer.Mean()/MS, AddTimer.Percentile(float64(AddTimer.Count())*0.9)/MS)
	fmt.Printf("Provide: %d ,     avg- %f ms, 0.9p- %f ms \n", Provide.Count(), Provide.Mean()/MS, Provide.Percentile(float64(Provide.Count())*0.9)/MS)
	fmt.Printf("Persist: %d ,     avg- %f ms, 0.9p- %f ms \n", Persist.Count(), Persist.Mean()/MS, Persist.Percentile(float64(Persist.Count())*0.9)/MS)
	fmt.Printf("Dag: %d ,     avg- %f ms, 0.9p- %f ms \n", Dag.Count(), Dag.Mean()/MS, Dag.Percentile(float64(Dag.Count())*0.9)/MS)

	fmt.Printf("Has: %d ,     avg- %f ms, 0.9p- %f ms \n", HasTimer.Count(), HasTimer.Mean()/MS, HasTimer.Percentile(float64(HasTimer.Count())*0.9)/MS)

}

func Output_addBreakdown() {
	fmt.Printf("avg(ms)    0.9p(ms)\n")
	fmt.Printf("%f %f\n", UploadTimer.Mean()/MS, UploadTimer.Percentile(float64(UploadTimer.Count())*0.9)/MS)
	fmt.Printf("%f %f\n", Provide.Mean()/MS, Provide.Percentile(float64(Provide.Count())*0.9)/MS)
	fmt.Printf("%f %f\n", Persist.Mean()/MS, Persist.Percentile(float64(Persist.Count())*0.9)/MS)
	fmt.Printf("%f %f\n", Dag.Mean()/MS, Dag.Percentile(float64(Dag.Count())*0.9)/MS)

}

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
