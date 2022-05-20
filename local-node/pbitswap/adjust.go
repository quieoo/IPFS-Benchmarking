package pbitswap

import (
	"fmt"
	"time"
)

const (
	L0                = 1
	CollectWindow     = 3
	HitRatioThreshold = 0.75
	INC0              = 2
	INC1              = 1
	DEC0              = 2
	MaxWaitTime       = 5 * time.Second

	AllowedDelayVariation = 0.2
)

type point struct {
	total  float64
	number int
}

func (p *point) in(x float64) {
	p.total += x
	p.number++
}
func (p *point) average() float64 {
	if p.number > 0 {
		return p.total / float64(p.number)
	}
	return 0
}

type DynamicAdjuster struct {
	role           ProviderRole
	minRequestTime float64

	historyRequestTime []float64
	historyDifference  []float64
	continuousHitTimes int

	L int

	tolerate time.Duration

	lastEfficiency float64
	increaing      bool
	top            bool
	init           bool

	averageEfficiency map[int]point

	continuousmisstimes int
}

func NewDynamicAdjuster() *DynamicAdjuster {
	return &DynamicAdjuster{
		role:                Role_CoWorker,
		L:                   10,
		minRequestTime:      1000 * time.Second.Seconds(),
		historyRequestTime:  make([]float64, CollectWindow),
		continuousHitTimes:  0,
		historyDifference:   make([]float64, CollectWindow),
		tolerate:            MaxWaitTime,
		increaing:           false,
		top:                 false,
		init:                true,
		lastEfficiency:      0,
		averageEfficiency:   map[int]point{},
		continuousmisstimes: 0,
	}
}

func (da *DynamicAdjuster) AverageRequestTime() float64 {
	r := 0.0
	for _, c := range da.historyRequestTime {
		r += c
	}
	return r / CollectWindow
}

func (da *DynamicAdjuster) AverageDiff() float64 {
	r := 0.0
	for _, c := range da.historyDifference {
		r += c
	}
	return r / CollectWindow
}

func (da *DynamicAdjuster) Adjust(hr float64, d time.Duration, n int) int {
	LastRequestTime := d.Seconds() / (float64(n))

	average := da.AverageRequestTime()

	if da.role == Role_CoWorker {
		if hr > HitRatioThreshold {
			da.L += INC0
		}
	} else {

		if LastRequestTime > average && ((LastRequestTime-average)/(LastRequestTime-da.minRequestTime) > AllowedDelayVariation) {
			if da.L > 1 {
				da.L = da.L / 2
			}
		}
		if LastRequestTime < average && ((average-LastRequestTime)/(LastRequestTime-da.minRequestTime) < AllowedDelayVariation) {
			da.L += INC1
		}
	}

	// update sates
	if hr > HitRatioThreshold {
		da.continuousHitTimes++
		if da.continuousHitTimes > CollectWindow {
			da.role = Role_FullProvider
		}
	} else {
		da.role = Role_CoWorker
	}

	for i := 0; i < CollectWindow-1; i++ {
		da.historyRequestTime[i] = da.historyRequestTime[i+1]
	}
	da.historyRequestTime[CollectWindow-1] = LastRequestTime
	if LastRequestTime < da.minRequestTime {
		da.minRequestTime = LastRequestTime
	}

	return da.L
}

func (da *DynamicAdjuster) Adjust3(hr float64, d time.Duration, n int) int {
	LastRequestTime := d.Seconds() / (float64(n))

	averagediff := da.AverageDiff()

	if da.role == Role_CoWorker {
		if hr > HitRatioThreshold {
			da.L += INC0
		}
	} else {
		if averagediff/da.minRequestTime < 0 {
			da.L += INC1
		} else {
			da.L = da.L * int(1-0.8*averagediff/da.minRequestTime)
		}
	}

	// update sates
	if hr > HitRatioThreshold {
		da.continuousHitTimes++
		if da.continuousHitTimes > CollectWindow {
			da.role = Role_FullProvider
		}
	} else {
		da.role = Role_CoWorker
	}

	for i := 0; i < CollectWindow-1; i++ {
		da.historyRequestTime[i] = da.historyRequestTime[i+1]
		da.historyDifference[i] = da.historyDifference[i+1]
	}
	da.historyDifference[CollectWindow-1] = LastRequestTime - da.historyRequestTime[CollectWindow-1]
	da.historyRequestTime[CollectWindow-1] = LastRequestTime
	if LastRequestTime < da.minRequestTime {
		da.minRequestTime = LastRequestTime
	}

	return da.L
}

func (da *DynamicAdjuster) Adjust2(hr float64, d time.Duration, n int) int {

	if n == 0 {
		return da.L
	}
	LastRequestTime := d.Seconds() / float64(n)
	average := da.AverageRequestTime()

	if da.role == Role_CoWorker {
		if hr > HitRatioThreshold {
			da.L += INC0
		}
	} else {
		if (LastRequestTime-average)/(LastRequestTime-da.minRequestTime) > AllowedDelayVariation {
			if da.L > 1 {
				da.L = da.L / 2
			}
		}
		da.L += INC1

	}

	// update sates
	if hr > HitRatioThreshold {
		da.continuousHitTimes++
		if da.continuousHitTimes > CollectWindow {
			da.role = Role_FullProvider
		}
	} else {
		da.role = Role_CoWorker
	}

	for i := 0; i < CollectWindow-1; i++ {
		da.historyRequestTime[i] = da.historyRequestTime[i+1]
	}
	da.historyRequestTime[CollectWindow-1] = LastRequestTime
	if LastRequestTime < da.minRequestTime {
		da.minRequestTime = LastRequestTime
	}

	return da.L
}

func (da *DynamicAdjuster) Adjust4(hr float64, d time.Duration, n int) int {

	if hr <= HitRatioThreshold {
		return da.L
	}
	if da.init {
		da.init = false
		return da.L
	}
	currentEfficiency := float64(n) / d.Seconds()
	fmt.Printf("current efficiency %f blk/s\n", currentEfficiency)
	if da.lastEfficiency == 0 {
		da.lastEfficiency = currentEfficiency
	}

	flag := currentEfficiency / da.lastEfficiency
	if flag < 1-AllowedDelayVariation {
		if da.L > 1 {
			da.L = da.L / 2
		}
		da.increaing = false
		da.top = false
	} else {
		if da.top {
			//remain
		} else {
			if da.increaing && flag <= 1 {
				//restore last length which is better than current one
				//we assume this is the top length
				da.L -= INC1
				da.top = true
			} else {
				da.L += INC1
				da.increaing = true
			}
		}
	}
	da.lastEfficiency = currentEfficiency

	return da.L
}

func (da *DynamicAdjuster) Adjust5(hr float64, d time.Duration, n int) int {
	if hr <= HitRatioThreshold {
		if hr <= 0 {
			da.continuousmisstimes++
			if da.continuousmisstimes >= 3 {
				da.tolerate *= 2
			}
		}
		return da.L
	}
	da.continuousmisstimes = 0
	da.tolerate = MaxWaitTime

	if da.init {
		da.init = false
		return da.L
	}
	currentEfficiency := float64(n) / d.Seconds()
	logger.Debugf("current efficiency: %f blk/s \n", currentEfficiency)
	//update status
	x := da.averageEfficiency[da.L]
	x.in(currentEfficiency)
	da.averageEfficiency[da.L] = x

	if da.lastEfficiency > currentEfficiency*1.5 {
		da.L = da.L / 2
	} else {
		//search history
		maxefficiency := currentEfficiency
		maxlength := da.L

		historyHaveBetter := false
		for k, v := range da.averageEfficiency {
			e := v.average()
			if e > maxefficiency {
				maxefficiency = e
				maxlength = k
				historyHaveBetter = true
			}
		}
		if historyHaveBetter && maxefficiency > currentEfficiency*1.2 {
			da.L = maxlength
		} else {
			da.L++
		}
	}

	da.lastEfficiency = currentEfficiency
	return da.L
}
