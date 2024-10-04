package pbitswap

import (
	"time"
)

const (
	L0                = 1
	CollectWindow     = 3
	HitRatioThreshold = 0.75
	INC0              = 2
	INC1              = 1
	DEC0              = 2
	MaxWaitTime       = 3 * time.Second // 缩短等待时间，提高效率

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

func (da *DynamicAdjuster) Adjust6(total_blk int, received_blk int, redundant_blk int, used_time time.Duration) int {
	logger.Debugf("total_blk %d received_blk %d redundant_blk %d used_time %f\n", total_blk, received_blk, redundant_blk, used_time.Seconds())

	receivedRatio := float64(received_blk) / float64(total_blk)
	redundantRatio := float64(redundant_blk) / float64(received_blk)

	// 批次大小动态调整规则
	if redundantRatio > 0.5 {
		// 冗余块比例高于50%，批次大小大幅减少
		da.L = int(float64(da.L) * 0.7)
		logger.Debugf("减少批次大小，因为冗余块比例过高 (%.2f)", redundantRatio)
	} else if redundantRatio < 0.1 {
		// 冗余块比例低于10%，批次大小增加
		da.L = int(float64(da.L) * 1.3)
		logger.Debugf("增加批次大小，因为冗余块比例较低 (%.2f)", redundantRatio)
	} else {
		// 冗余比例保持稳定时，不改变批次大小
		logger.Debugf("保持批次大小，因为冗余块比例适中 (%.2f)", redundantRatio)
	}

	// 接收比例动态调整规则
	if receivedRatio < 0.3 {
		// 成功率较低时，增加等待时间，减少批次大小
		da.tolerate = time.Duration(float64(da.tolerate) * 2)
		da.L = int(float64(da.L) * 0.8)
		logger.Debugf("减少批次大小，因为接收成功率较低 (%.2f)", receivedRatio)
	} else if receivedRatio > 0.9 {
		// 成功率高时，减少等待时间
		da.tolerate = time.Duration(float64(da.tolerate) * 0.9)
		logger.Debugf("减少等待时间，因为接收成功率较高 (%.2f)", receivedRatio)
	} else {
		logger.Debugf("保持批次大小和等待时间，因为接收成功率在合理范围内 (%.2f)", receivedRatio)
	}

	// 限制批次大小在合理范围内
	if da.L < 1 {
		da.L = 1
	}

	// 限制等待时间在合理范围内
	if da.tolerate < 5*time.Second {
		da.tolerate = 5 * time.Second
	} else if da.tolerate > 15*time.Second {
		da.tolerate = 15 * time.Second
	}

	logger.Debugf("调整后的请求批次大小: %d, 最大等待时间: %s\n", da.L, da.tolerate)
	return da.L
}

// 优化的调整策略：动态调整请求的块数量，减少过长的惩罚时间
func (da *DynamicAdjuster) Adjust5(hr float64, d time.Duration, n int) int {

	if hr > 0.9 {
		da.tolerate -= 1 * time.Second
	}
	if hr <= HitRatioThreshold {
		if hr <= 0 {
			da.continuousmisstimes++
			// 这里减少超时时间的增长比例，避免过度惩罚
			if da.continuousmisstimes >= 3 {
				da.tolerate += 1 * time.Second // 缩小超时时间的增加
			}
		}
		return da.L
	}
	da.continuousmisstimes = 0
	da.tolerate = MaxWaitTime // 恢复到最大等待时间

	if da.init {
		da.init = false
		return da.L
	}
	currentEfficiency := float64(n) / d.Seconds()
	logger.Debugf("当前效率: %f 块/秒 \n", currentEfficiency)

	// 估算带宽，并动态调整请求数量
	estimatedBandwidth := float64(n) / d.Seconds()
	if estimatedBandwidth > 0 {
		if currentEfficiency < estimatedBandwidth*0.8 {
			da.L = int(float64(da.L) * 1.2) // 增加请求数量
		} else if currentEfficiency > estimatedBandwidth {
			da.L = int(float64(da.L) * 0.8) // 减少请求数量
		}
	}

	// 更新平均效率数据
	x, exist := da.averageEfficiency[da.L]
	if exist {
		x.in(currentEfficiency)
		da.averageEfficiency[da.L] = x
	}

	// 如果历史效率明显高于当前效率，则减小L值
	if da.lastEfficiency > currentEfficiency*1.5 {
		da.L = da.L / 2
	} else {
		maxEfficiency := currentEfficiency
		maxLength := da.L

		historyHaveBetter := false
		for k, v := range da.averageEfficiency {
			e := v.average()
			if e > maxEfficiency {
				maxEfficiency = e
				maxLength = k
				historyHaveBetter = true
			}
		}

		if historyHaveBetter && maxEfficiency > currentEfficiency*1.2 {
			da.L = maxLength
		} else {
			da.L++ // 增加请求数量
		}
	}

	da.lastEfficiency = currentEfficiency
	return da.L
}
