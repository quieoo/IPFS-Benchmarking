package metrics

import (
	"fmt"
	"time"

	"github.com/rcrowley/go-metrics"
)

//Provide metrics
var ProvideTime metrics.Timer
var SuccessfullyProvide int
var StartBackProvideTime time.Time
var LastFewProvides *Queue //record the Min CPL in top K peers for last a few provides

var QueryPeerTime = 60

func UpdateProvideMetric(StartProvideTime time.Time, key string) {
	if !CMD_EnableMetrics {
		return
	}
	// fmt.Printf("    Provided: %s\n", key)
	if StartBackProvideTime == ZeroTime {
		StartBackProvideTime = StartProvideTime
	}
	ProvideTime.Update(time.Now().Sub(StartBackProvideTime))
	SuccessfullyProvide++
}

func Output_ProvideMonitor() {
	if !CMD_EnableMetrics {
		return
	}

	fmt.Println("--------------------------DHT.Provide----------------------")
	fmt.Printf("ProvideLatency: %d ,     avg- %f ms, 0.9p- %f ms \n", ProvideTime.Count(), ProvideTime.Mean()/MS, ProvideTime.Percentile(float64(ProvideTime.Count())*0.9)/MS)
	fmt.Printf("ProvideThroughput: %f /min\n", float64(SuccessfullyProvide)/(time.Now().Sub(StartBackProvideTime).Seconds()/60))
}

func AverageLastFewMinCPLs() float64 {
	if !CMD_EarlyAbort {
		return 0
	}
	total := 0
	num := 0
	LastFewProvides.IterateQueue(func(data int) {
		total += data
		num++
	})
	if num == 0 {
		return 0
	}
	return float64(total) / float64(num)
}
