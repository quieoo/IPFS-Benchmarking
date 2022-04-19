package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/rcrowley/go-metrics"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MS = 1000000
const downloadfilepath = "./downloaded"

var downloadNumber int // download part of trace files, debug use

func main() {
	var cmd string
	var filesize int
	var filenumber int
	var concurrentGet int
	var filenames string
	var host string
	var ips []string
	var trace_workload string
	var servers int
	var traceDownload_randomRequest bool

	flag.StringVar(&cmd, "c", "", "operation type\n"+
		"upload: upload file with -s for file size, -n for file number, -fn for specified name of file storing all uploaded file name\n"+
		"download: download files following specified filename\n"+
		"traceDownload: download according to generated trace file\n")

	flag.IntVar(&filesize, "size", 256*1024, "file size")
	flag.IntVar(&filenumber, "n", 1, "file number")
	flag.IntVar(&concurrentGet, "cg", 1, "concurrent get number")
	flag.StringVar(&filenames, "fn", "filenames", "name of files for uploading")

	flag.StringVar(&host, "h", "127.0.0.1", "server addresses, seperated by commas")
	flag.BoolVar(&traceDownload_randomRequest, "randomRequest", false, "random request means that current client will randomly reorder requests from generated workload")
	flag.StringVar(&trace_workload, "f", "", "file indicates the path of workload file of generated trace")
	flag.IntVar(&servers, "s", 1, "servers indicates the total number of servers")
	flag.IntVar(&downloadNumber, "dn", 0, "")

	flag.Parse()

	uploadTimer := metrics.NewTimer()
	downloadTimer := metrics.NewTimer()

	ips = strings.Split(host, ",")
	if len(ips) != servers {
		fmt.Printf("the number of given ips %d is not equal with server number %d\n", len(ips), servers)
		return
	}

	if cmd == "upload" {
		Upload(filenames, filesize, filenumber, ips[0], uploadTimer)
	} else if cmd == "download" {
		Download(filenames, downloadfilepath, concurrentGet, ips[0], downloadTimer)
	} else if cmd == "traceDownload" {
		TraceDownload(trace_workload, traceDownload_randomRequest, ips, servers)
	}
}

var StdChars = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

// NewLenChars returns a new random string of the provided length, consisting of the provided byte slice of allowed characters(maximum 256).
func NewLenChars(length int, chars []byte) string {
	if length == 0 {
		return ""
	}
	clen := len(chars)
	if clen < 2 || clen > 256 {
		panic("Wrong charset length for NewLenChars()")
	}
	maxrb := 255 - (256 % clen)
	b := make([]byte, length)
	r := make([]byte, length+(length/4)) // storage for random bytes.
	i := 0
	for {
		if _, err := rand.Read(r); err != nil {
			panic("Error reading random bytes: " + err.Error())
		}
		for _, rb := range r {
			c := int(rb)
			if c > maxrb {
				continue // Skip this number to avoid modulo bias.
			}
			b[i] = chars[c%clen]
			i++
			if i == length {
				return string(b)
			}
		}
	}
}

func createDirIfNotExist(path string) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir(path, 0777)
			if err != nil {
				fmt.Printf("failed to mkdir: %v\n", err.Error())
				return
			}
		} else {
			fmt.Println(err.Error())
			return
		}
	}
}

func standardOutput(function string, t metrics.Timer) string {
	return fmt.Sprintf("%s: %d files, average latency: %f ms, 0.99P latency: %f ms\n", function, t.Count(), t.Mean()/MS, t.Percentile(float64(t.Count())*0.99)/MS)
}

func Upload(filenames string, filesize int, filenumber int, host string, uploadTimer metrics.Timer) {
	fn, _ := os.Create(filenames)
	fmt.Printf("Uploading files with size %d B\n", filesize)

	createDirIfNotExist("./temp")

	for i := 0; i < filenumber; i++ {
		//create temporary file
		name := fmt.Sprintf("%d-%d", filesize, i)
		subs := NewLenChars(filesize, StdChars)
		err := ioutil.WriteFile("./temp/"+name, []byte(subs), 0666)
		if err != nil {
			fmt.Println("failed to write temporary file: ", err.Error())
			return
		}

		uploadstart := time.Now()
		//upload file
		bodyBuffer := &bytes.Buffer{}
		bodyWriter := multipart.NewWriter(bodyBuffer)

		fileWriter, _ := bodyWriter.CreateFormFile("files", name)

		file, _ := os.Open("./temp/" + name)

		_, err = io.Copy(fileWriter, file)
		if err != nil {
			fmt.Println("failed to copy file to buffer: ", err.Error())
			return
		}

		contentType := bodyWriter.FormDataContentType()
		bodyWriter.Close()

		resp, _ := http.Post("http://"+host+":8080/upload", contentType, bodyBuffer)

		resp.Body.Close()
		file.Close()
		//resp_body, _ := ioutil.ReadAll(resp.Body)
		uploadTimer.Update(time.Now().Sub(uploadstart))
		//name -> filename
		_, err = io.WriteString(fn, name+"\n")
		if err != nil {
			fmt.Println("failed to store filename: ", err.Error())
			return
		}
	}

	//cleaning temporary files
	_, err := os.Stat("./temp")
	if !(err != nil && os.IsNotExist(err)) {
		err := os.RemoveAll("./temp")
		if err != nil {
			fmt.Println("failed to remove temp directory: ", err.Error())
			return
		}
	}

	//output metrics
	fmt.Printf(standardOutput("http-upload", uploadTimer))
}

func Download(filenames string, downloadfilepath string, concurrentGet int, host string, downloadTimer metrics.Timer) {
	file, err := os.Open(filenames)
	defer file.Close()
	if err != nil {
		fmt.Println("failed to open filenames to read file name: ", err.Error())
		return
	}
	//br := bufio.NewReader(file)

	createDirIfNotExist(downloadfilepath)

	// 把cid读到多个切片中
	inputReader := bufio.NewReader(file)

	allFileName := make([][]string, concurrentGet)
	for i := range allFileName {
		allFileName[i] = make([]string, 0)
	}

	tmpCnt := 0
	for {
		aLine, readErr := inputReader.ReadString('\n')
		aLine = strings.TrimSuffix(aLine, "\n")
		if readErr == io.EOF {
			break
		}
		allFileName[tmpCnt%concurrentGet] = append(allFileName[tmpCnt%concurrentGet], aLine)
		tmpCnt++
	}

	// 创建多个协程，分别去get文件
	var wg sync.WaitGroup
	wg.Add(concurrentGet)
	for i := 0; i < concurrentGet; i++ {
		go func(theOrder int) {
			defer wg.Done()
			for j := 0; j < len(allFileName[theOrder]); j++ {
				toRequest := allFileName[theOrder][j]

				downloadstrat := time.Now()
				url := "http://" + host + ":8080/files/" + toRequest
				res, err := http.Get(url)
				if err != nil {
					fmt.Println("failed to request file: ", err.Error())
					return
				}

				f, err := os.Create(downloadfilepath + "/" + toRequest)
				if err != nil {
					fmt.Println("failed to create local file: ", err.Error())
					return
				}
				_, err = io.Copy(f, res.Body)
				if err != nil {
					fmt.Println("failed to copy response to local file: ", err.Error())
					return
				}
				downloadTimer.Update(time.Now().Sub(downloadstrat))

				if err != nil {
					fmt.Printf("thread %d downloading stall caused by %s", theOrder, err.Error())
					fmt.Printf(standardOutput("http-download", downloadTimer))
					return
				} else if j == len(allFileName[theOrder])-1 {
					fmt.Printf("thread %d downloading stall caused by havingfilesystem:https://docs.google.com/persistent/docs/documents/1HT8zjCAQULYnfpc_EahYXJ0MRoEIW53cR_zucqMB7wU/image/PLACEHOLDER_1e1273deb9e66546_0 downloaded all needed files.\n", theOrder)
					fmt.Printf(standardOutput("http-download", downloadTimer))
					return
				}
			}

		}(i)
	}
	wg.Wait()
}

var DownloadedFileSize []int
var AvgDownloadLatency metrics.Timer
var ALL_AvgDownloadLatency metrics.Timer
var ALL_DownloadedFileSize []int

func TraceDownload(trace_workload string, traceDownload_randomRequest bool, ips []string, servers int) {
	AvgDownloadLatency = metrics.NewTimer()
	metrics.Register("AvgDownloadLatency", AvgDownloadLatency)
	ALL_AvgDownloadLatency = metrics.NewTimer()
	metrics.Register("ALL_AvgDownloadLatency", ALL_AvgDownloadLatency)

	// read trace workload file
	tracefile := trace_workload
	if traces, err := os.Open(tracefile); err != nil {
		fmt.Printf("failed to open trace file: %s, due to %s\n", tracefile, err.Error())
		return
	} else {
		if _, err := os.Stat(downloadfilepath); os.IsNotExist(err) {
			os.Mkdir(downloadfilepath, 0777)
			//os.Chmod(downloadfilepath, 0777)
		}

		defer traces.Close()
		scanner := bufio.NewScanner(traces)
		var names []int

		for scanner.Scan() {
			line := scanner.Text()
			codes := strings.Split(line, "\t")
			name, _ := strconv.Atoi(codes[1])
			names = append(names, name)
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Cannot scanner text file: %s, err: [%v]\n", tracefile, err)
			return
		}

		//randomize request order
		if traceDownload_randomRequest {
			fmt.Println("randomizing request queue")
			names = Shuffle(names)
		}

		n := len(names)
		if downloadNumber != 0 {
			n = downloadNumber
		}

		StartRequest := time.Now()

		//period log
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
				line := fmt.Sprintf("%s %f %d %f %f\n", time.Now().String(), throughput, AvgDownloadLatency.Count(), AvgDownloadLatency.Mean()/MS, AvgDownloadLatency.Percentile(float64(AvgDownloadLatency.Count())*0.99)/MS)
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

		//request to each server
		for i := 0; i < n; i++ {

			toRequest := names[i]
			//fmt.Println(toRequest)
			hs := ips[toRequest%servers]
			requestFile := strconv.Itoa(toRequest)
			fmt.Printf("downloading %.2f from %s \n", float64(i)/float64(n)*100, hs)
			downloadstrat := time.Now()
			url := "http://" + hs + ":8080/files/" + requestFile
			res, err := http.Get(url)
			if err != nil {
				fmt.Println("failed to request file: ", err.Error())
				return
			}

			f, err := os.Create(downloadfilepath + "/" + requestFile)
			if err != nil {
				fmt.Println("failed to create local file: ", err.Error())
				return
			}
			_, err = io.Copy(f, res.Body)
			if err != nil {
				fmt.Println("failed to copy response to local file: ", err.Error())
				return
			}

			ALL_AvgDownloadLatency.UpdateSince(downloadstrat)
			fi, _ := f.Stat()
			DownloadedFileSize = append(DownloadedFileSize, int(fi.Size()))
			ALL_DownloadedFileSize = append(ALL_DownloadedFileSize, int(fi.Size()))
			AvgDownloadLatency.UpdateSince(downloadstrat)

			f.Close()
		}

		//summarize
		TotalSize := 0
		TotalTime := time.Now().Sub(StartRequest).Seconds()
		for i := 0; i < len(ALL_DownloadedFileSize); i++ {
			TotalSize += ALL_DownloadedFileSize[i]
		}
		throughput := float64(TotalSize/1024/1024) / TotalTime

		line := fmt.Sprintf("%s %f %d %f %f\n", time.Now().String(), throughput, ALL_AvgDownloadLatency.Count(), ALL_AvgDownloadLatency.Mean()/1000000, ALL_AvgDownloadLatency.Percentile(float64(ALL_AvgDownloadLatency.Count())*0.99)/1000000)
		fmt.Println(line)
	}
}
func Shuffle(vals []int) []int {
	interfaces, _ := net.Interfaces()
	mac := int64(0)
	for _, i := range interfaces {
		//fmt.Println(len(i.HardwareAddr))
		if ha := i.HardwareAddr; len(ha) > 0 {
			ha = append(ha, 8)
			ha = append(ha, 9)
			mac += BytesToInt64(ha)
		}
	}
	r := rand.New(rand.NewSource(time.Now().Unix() + mac))
	ret := make([]int, len(vals))
	perm := r.Perm(len(vals))
	for i, randIndex := range perm {
		ret[i] = vals[randIndex]
	}
	return ret
}
func BytesToInt64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}
