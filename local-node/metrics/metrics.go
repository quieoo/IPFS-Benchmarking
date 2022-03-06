package metrics

import (
	"fmt"
	"github.com/rcrowley/go-metrics"
)

var CMD_CloseBackProvide = true
var CMD_CloseLANDHT = false
var CMD_CloseDHTRefresh = false
var CMD_CloseAddProvide = false
var CMD_ImmeProvide = true
var CMD_EnableMetrics = true

var AddTimer metrics.Timer
var HasTimer metrics.Timer
var GetTimer metrics.Timer
var PutTimer metrics.Timer
var PutManyTimer metrics.Timer
var GetSizeTimer metrics.Timer
var SyncFileTimer metrics.Timer
var AddProvideTimer metrics.Timer

var UploadTimer metrics.Timer
var DownloadTimer metrics.Timer

// TimersInit 实现了
func TimersInit() {
	UploadTimer = metrics.NewTimer()
	metrics.Register("Upload", UploadTimer)

	DownloadTimer = metrics.NewTimer()
	metrics.Register("Download", DownloadTimer)

	AddTimer = metrics.NewTimer()
	metrics.Register("Add", AddTimer)

	HasTimer = metrics.NewTimer()
	metrics.Register("Has", HasTimer)

	GetTimer = metrics.NewTimer()
	metrics.Register("Get", GetTimer)

	PutTimer = metrics.NewTimer()
	metrics.Register("Put", PutTimer)

	PutManyTimer = metrics.NewTimer()
	metrics.Register("PutMany", PutManyTimer)

	SyncFileTimer = metrics.NewTimer()
	metrics.Register("SyncFile", SyncFileTimer)

	GetSizeTimer = metrics.NewTimer()
	metrics.Register("GetSize", GetSizeTimer)

	AddProvideTimer = metrics.NewTimer()
	metrics.Register("AddProvide", AddProvideTimer)

	//go metrics.Log(metrics.DefaultRegistry, 1 * time.Second,log.New(os.Stdout, "metrics: ", log.Lmicroseconds))

}

const MS = 1000000

func OutputMetrics() {
	fmt.Printf(standardOutput("Upload", UploadTimer))
	fmt.Printf(standardOutput("Download", DownloadTimer))
	fmt.Printf(standardOutput("AddProvide", AddProvideTimer))

	addtotal := float64(AddTimer.Sum())
	hastotal := float64(HasTimer.Sum())
	gettotal := float64(GetTimer.Sum())
	puttotal := float64(PutTimer.Sum())
	putmanytotal := float64(PutManyTimer.Sum())
	synctotal := float64(SyncFileTimer.Sum())

	fmt.Printf("Add: %d ,     avg- %f ms, 0.9p- %f ms \n", AddTimer.Count(), AddTimer.Mean()/MS, AddTimer.Percentile(float64(AddTimer.Count())*0.9)/MS)
	fmt.Printf("Has: %d ,     avg- %f ms, %f, 0.9p- %f ms \n", HasTimer.Count(), HasTimer.Mean()/MS, hastotal/addtotal, HasTimer.Percentile(float64(HasTimer.Count())*0.9)/MS)
	fmt.Printf("Get: %d ,     avg- %f ms, %f, 0.9p- %f ms \n", GetTimer.Count(), GetTimer.Mean()/MS, gettotal/addtotal, GetTimer.Percentile(float64(GetTimer.Count())*0.9)/MS)
	fmt.Printf("Put: %d ,     avg- %f ms, %f, 0.9p- %f ms \n", PutTimer.Count(), PutTimer.Mean()/MS, puttotal/addtotal, PutTimer.Percentile(float64(PutTimer.Count())*0.9)/MS)
	fmt.Printf("PutMany: %d , avg- %f ms, %f, 0.9p- %f ms \n", PutManyTimer.Count(), PutManyTimer.Mean()/MS, putmanytotal/addtotal, PutManyTimer.Percentile(float64(PutManyTimer.Count())*0.9)/MS)
	fmt.Printf("SyncFile: %d ,avg- %f ms, %f, 0.9p- %f ms \n", SyncFileTimer.Count(), SyncFileTimer.Mean()/MS, synctotal/addtotal, SyncFileTimer.Percentile(float64(SyncFileTimer.Count())*0.9)/MS)

	fmt.Printf("GetSize: %d , avg- %f ms,0.9p- %f ms \n", GetSizeTimer.Count(), GetSizeTimer.Mean()/MS, GetSizeTimer.Percentile(float64(GetSizeTimer.Count())*0.9)/MS)

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
