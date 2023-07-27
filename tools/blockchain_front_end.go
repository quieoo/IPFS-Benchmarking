package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func sendRequest(requests []byte, addr string) []byte {
	// 连接服务端
	// conn, err := net.Dial("tcp", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("Error connecting to server: ", addr, err)
		return nil
	}
	defer conn.Close()
	// 发送请求给服务端
	_, err = conn.Write(requests)
	if err != nil {
		fmt.Println("Error sending request to server: ", addr, err)
		return nil
	}

	// 接收服务端返回的结果
	result := make([]byte, 1024)
	_, err = conn.Read(result)
	if err != nil {
		fmt.Println("Error receiving result from server: ", addr, err)
		return nil
	}

	// fmt.Println("Received result:", string(result))
	return result
}

var total_blks int
var verbos bool
var peers_addr string
var bitcoin_trace string

func debug(s string) {
	if verbos {
		fmt.Printf(s)
	}
}

func makeRequests(addr string, request []byte, wg *sync.WaitGroup, resultChan chan<- string) {
	defer wg.Done()
	result := sendRequest(request, addr)
	if result == nil {
		fmt.Println("out chan nil")
		os.Exit(0)
	} else {
		resultChan <- string(result)
	}
}

var BlockNum int
var LastBlockNum int
var Transactions int

func TPS_Monitor() {
	BlockNum = 0
	LastBlockNum = 0

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
		line := fmt.Sprintf(" %s Blocks last minute: %f, TPS last minute: %f\n", time.Now().String(), float32(BlockNum-LastBlockNum)/60, float32(Transactions)/60)
		fmt.Printf("%s", line)
		_, err := write.WriteString(line)
		if err != nil {
			fmt.Printf("failed to write string to log file, %s\n", err.Error())
		}
		write.Flush()
		LastBlockNum = BlockNum
		Transactions = 0
		time.Sleep(timeUnit)
	}
}

func parse_bitcoin() ([]int, []int) {
	var blk_size []int
	var txs []int
	filePath := bitcoin_trace // 文件路径

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("无法打开文件:", err)
		return nil, nil
	}
	defer file.Close()

	// 创建一个 bufio.Scanner 以逐行读取文件内容
	scanner := bufio.NewScanner(file)

	// 逐行读取文件内容
	for scanner.Scan() {
		line := scanner.Text()            // 当前行内容
		words := strings.Split(line, " ") // 使用空格拆分行内容生成切片

		tx, err := strconv.Atoi(words[0])
		if err != nil {
			fmt.Println("无法将字符串转换为整数:", err)
			return nil, nil
		}
		size, err := strconv.ParseFloat(words[1], 32)
		if err != nil {
			fmt.Println("无法将字符串转换为浮点数:", err)
			return nil, nil
		}
		blk_size = append(blk_size, int(size*1024))
		txs = append(txs, tx)
		// fmt.Printf("%d %d\n", size, int(floatVal*1024))
	}

	// 检查扫描过程是否出错
	if err := scanner.Err(); err != nil {
		fmt.Println("扫描文件出错:", err)
		return nil, nil
	}
	fmt.Printf("Total blocks: %d\n", len(blk_size))
	return blk_size, txs
}

func main() {

	flag.IntVar(&total_blks, "n", 0, "number of blocks")
	flag.BoolVar(&(verbos), "v", false, "show detail output")
	flag.StringVar(&peers_addr, "p", "blockchain_peers", "address for peers")
	flag.StringVar(&bitcoin_trace, "b", "bitcoin_blk_one_week", "trace of bitcoin to paly on IPFS")

	flag.Parse()

	//load peer address
	var addrs []string
	filename := peers_addr
	file, err := os.Open(filename)
	if err != nil {
		fmt.Errorf("failed to open neighbour file: %v\n", err)
		return
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		addrs = append(addrs, line)
		fmt.Printf("ipfs back_end peers: %s\n", line)

	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Cannot scanner text file: %s, err: [%v]\n", file.Name(), err)
		return
	}
	file.Close()
	// go TPS_Monitor()
	file_size, txs := parse_bitcoin()
	if file_size != nil {
		total_blks = len(file_size)
	}
	if total_blks == 0 {
		for {
			debug("Generate Block")
			add_req := "1 " + strconv.Itoa(1024*1024)
			ret := sendRequest([]byte(add_req), addrs[0])
			if ret == nil {
				fmt.Printf("ADD file to server error\n")
				return
			} else {
				add_resps := strings.Split(string(ret), " ")
				if add_resps[0] == "0" {
					debug("Broadcast Block")
					// send GET requests to clients
					get_req := []byte("2 " + add_resps[1] + " ")
					// fmt.Println(string(get_req))
					var wg sync.WaitGroup
					wg.Add(len(addrs) - 1)
					resultChan := make(chan string, len(addrs)-1)
					for _, addr := range addrs[1:] {
						go makeRequests(addr, get_req, &wg, resultChan)
					}
					wg.Wait()
					close(resultChan)
					debug("Commit Block")
					fmt.Printf("%s: finish %s\n", time.Now().String(), add_resps[1])
					BlockNum++
				}
			}
		}
	} else {

		//send requests to back ends
		for i := 0; i < total_blks; i++ {
			debug("Generate Block")
			// send ADD reqeusts to provider
			add_req := "1 " + strconv.Itoa(file_size[i])
			ret := sendRequest([]byte(add_req), addrs[0])
			if ret == nil {
				fmt.Printf("ADD file to server error\n")
				return
			} else {
				add_resps := strings.Split(string(ret), " ")
				if add_resps[0] == "0" {
					debug("Broadcast Block")
					// send GET requests to clients
					get_req := []byte("2 " + add_resps[1] + " ")
					// fmt.Println(string(get_req))
					var wg sync.WaitGroup
					wg.Add(len(addrs) - 1)
					resultChan := make(chan string, len(addrs)-1)
					for _, addr := range addrs[1:] {
						go makeRequests(addr, get_req, &wg, resultChan)
					}
					wg.Wait()
					close(resultChan)
					debug("Commit Block")
					fmt.Printf("%d %s: finish %s\n", i, time.Now().String(), add_resps[1])
					BlockNum++
					Transactions += txs[i]
				}
			}
		}
	}

}
