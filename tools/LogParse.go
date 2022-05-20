package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

var timeLayoutStr = "2006-01-02 15:04:05"

func TraceDownload() {
	filename := "PL_HTTP_CloserPeers_2/PLAMG"
	startTime, _ := time.Parse(timeLayoutStr, "2022-04-24 17:00:00.00")

	var throughputs []float64
	var reqps []float64
	var latencies []float64
	interval := 60

	if file, err := os.Open(filename); err != nil {
		fmt.Printf("failed to open trace file: %s, due to %s\n", filename, err.Error())
		return
	} else {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			// 2022-04-19 02:58:07.904669821 +0800 CST m=+24544.924354645 0.513517 316 189.324362 3499.347537
			ss := strings.Split(line, " ")
			ts := ss[0] + " " + ss[1]
			t, _ := time.Parse(timeLayoutStr, ts)
			if t.After(startTime) {
				th, _ := strconv.ParseFloat(ss[5], 64)
				throughputs = append(throughputs, th)

				reqs, _ := strconv.Atoi(ss[6])
				reqps = append(reqps, float64(reqs)/float64(60))

				la, _ := strconv.ParseFloat(ss[7], 64)
				latencies = append(latencies, la)
			}
		}
		file.Close()

		total := 0.0
		inter := 0.0
		totalreq := 0.0
		interreq := 0.0
		totallat := 0.0
		interlat := 0.0
		for i := 0; i < len(throughputs); i++ {
			total += throughputs[i]
			inter += throughputs[i]

			totalreq += reqps[i]
			interreq += reqps[i]

			totallat += latencies[i]
			interlat += latencies[i]

			if i%interval == interval-1 {
				fmt.Printf("%f %f %f\n", inter/float64(interval), interreq/float64(interval), interlat/float64(interval))
				inter = 0.0
				interreq = 0.0
				interlat = 0.0
			}
		}
		fmt.Printf("%f %f %f\n", inter/float64(len(throughputs)%interval), interreq/float64(len(throughputs)%interval), interlat/float64(len(throughputs)%interval))
		fmt.Printf("%f %f %f \n", total/float64(len(throughputs)), totalreq/float64(len(throughputs)), totallat/float64(len(throughputs)))
	}
}

func FindProvider() {
	filename := "findProvider_WAN_log2"
	prefixs := []string{"TimerPin-0:", "TimerPin-1:"}
	errPrefix := "error while get"
	interval := 100

	fmt.Printf("-------------%s-------------\n", "time")
	if file, err := os.Open(filename); err != nil {
		fmt.Printf("failed to open trace file: %s, due to %s\n", filename, err.Error())
		return
	} else {
		scanner := bufio.NewScanner(file)
		i := 0
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "2022-05-") {
				if i%interval == 0 {
					ss := strings.Split(line, " ")
					fmt.Printf("%s\n", ss[1])
				}
				i++
			}
		}

		file.Close()
	}

	for _, prefix := range prefixs {
		fmt.Printf("-------------%s-------------\n", prefix)
		var points []float64
		if file, err := os.Open(filename); err != nil {
			fmt.Printf("failed to open trace file: %s, due to %s\n", filename, err.Error())
			return
		} else {
			// read data samples filtered with specified prefix
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				// error
				// only count valuable data
				if strings.HasPrefix(line, errPrefix) {
					break
				}
				if strings.HasPrefix(line, prefix) {
					fmt.Println(line)
					ss := strings.Split(line, " ")
					ls, _ := strconv.ParseFloat(ss[9], 64)
					points = append(points, ls)
				}
			}

			// calculate the average
			var average []float64
			inter := 0.0
			n := 0
			for i := 0; i < len(points); i++ {
				inter += points[i]
				n++
				if i%interval == interval-1 || (i+1) > len(points)-1 {
					average = append(average, inter/float64(n))
					inter = 0.0
					n = 0
				}

			}

			// calculate the deviation
			var StandardDeviation []float64
			devia := 0.0
			n = 0
			for i := 0; i < len(points); i++ {
				group := i / interval
				devia += math.Pow(points[i]-average[group], 2)
				n++
				if (i+1)/interval > group || (i+1) > len(points)-1 {
					StandardDeviation = append(StandardDeviation, math.Pow(devia/float64(n), 0.5))
					devia = 0.0
					n = 0
				}
			}
			// output
			for i := 0; i < len(average); i++ {
				fmt.Printf("%f %f\n", average[i], StandardDeviation[i])
			}

			file.Close()
		}

	}

}

func main() {
	FindProvider()
}
