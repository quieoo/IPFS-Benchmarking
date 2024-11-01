package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"metrics"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	logging "github.com/ipfs/go-log"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	icorepath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"

	"math"
	"math/rand"
	gometrcs "github.com/rcrowley/go-metrics"
	cid "github.com/ipfs/go-cid"
	"encoding/json"
)

func setupPlugins(externalPluginsPath string) error {
	// Load any external plugins if available on externalPluginsPath
	plugins, err := loader.NewPluginLoader(filepath.Join(externalPluginsPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	// Load preloaded and external plugins
	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}

func createTempRepo(ctx context.Context) (string, error) {
	/*repoPath, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}*/

	u, err := user.Current()
	if err != nil {
		fmt.Println("failed to get home dir. ")
	}
	repoPath := u.HomeDir + "/.ipfs"

	//repoPath := "~/.ipfs"  // '~'
	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}

	// Create the repo with the config
	//err = fsrepo.Init(repoPath, cfg)
	//err = fsrepo.Init("~/.ipfs", cfg)
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}

/// ------ Spawning the node

// Creates an IPFS node and returns its coreAPI
func createNode(ctx context.Context, repoPath string) (icore.CoreAPI, error) {
	// Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}

	// Construct the node

	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption, // This option sets the node to be a full DHT node (both fetching and storing DHT Records)
		// Routing: libp2p.DHTClientOption, // This option sets the node to be a client DHT node (only fetching records)
		Repo: repo,
	}

	node, err := core.NewNode(ctx, nodeOptions)
	if err != nil {
		return nil, err
	}

	// Attach the Core API to the constructed node
	return coreapi.NewCoreAPI(node)
}

// Spawns a node to be used just for this run (i.e. creates a tmp repo)
func spawnEphemeral(ctx context.Context) (icore.CoreAPI, error) {
	if err := setupPlugins(""); err != nil {
		return nil, err
	}

	// Create a Temporary Repo
	repoPath, err := createTempRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp repo: %s", err)
	}

	// Spawning an ephemeral IPFS node
	return createNode(ctx, repoPath)
}

// Spawns a node on the default repo location, if the repo exists
func spawnDefault(ctx context.Context) (icore.CoreAPI, error) {
	defaultPath, err := config.PathRoot()
	if err != nil {
		// shouldn't be possible
		return nil, err
	}

	if err := setupPlugins(defaultPath); err != nil {
		return nil, err

	}

	return createNode(ctx, defaultPath)
}

func Ini() (context.Context, icore.CoreAPI, context.CancelFunc) {
	fmt.Println("-- Getting an IPFS node running -- ")

	ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	// Spawn a node using the default path (~/.ipfs), assuming that a repo exists there already
	fmt.Println("Spawning node on default repo")
	ipfs, err := spawnDefault(ctx)
	if err != nil {
		fmt.Println("No IPFS repo available on the default path")
	}

	/*
		// Spawn a node using a temporary path, creating a temporary repo for the run
		fmt.Println("Spawning node on a temporary repo")
		ipfs, err := spawnEphemeral(ctx)
		if err != nil {
			panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
		}*/

	fmt.Println("IPFS node is running")
	return ctx, ipfs, cancel
}

func getUnixfsNode(path string) (files.Node, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := files.NewSerialFile(path, false, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// NOTE: I modified function here adding a chunker para.
func UploadFile(file string, ctx context.Context, ipfs icore.CoreAPI, chunker string, ProvideThrough bool) (icorepath.Resolved, error) {
	defer func() {
		if !metrics.CMD_EnableMetrics {
			return
		}
		metrics.AddTimer.Update(metrics.AddDura)
		metrics.Provide.Update(metrics.ProvideDura)
		metrics.Persist.Update(metrics.PersistDura)

		dagTime := metrics.AddDura - metrics.ProvideDura - metrics.PersistDura
		metrics.Dag.Update(dagTime)

		metrics.FlatfsHasTimer.Update(metrics.FlatfsHasDura)
		metrics.FlatfsPut.Update(metrics.FlatfsPutDura)

		metrics.FlatfsPutDura = 0
		metrics.FlatfsHasDura = 0
		metrics.AddDura = 0
		metrics.ProvideDura = 0
		metrics.PersistDura = 0
	}()
	start := time.Now()
	somefile, err := getUnixfsNode(file)
	if err != nil {
		return nil, err
	}

	opts := []options.UnixfsAddOption{
		options.Unixfs.Chunker(chunker),
	}
	if ProvideThrough {
		opts = append(opts, options.Unixfs.ProvideThrough())
	}

	cid, err := ipfs.Unixfs().Add(ctx, somefile, opts...)

	if err != nil {
		fmt.Println("failed to upload file in function : UploadFile")
		return nil, err
	}

	metrics.AddDura += time.Now().Sub(start)
	//quieoo.AddTimer.UpdateSince(start)
	return cid, err
}

// NOTE: I modified the function for adding a para named chunker.
func Upload(size, number, cores int, ctx context.Context, ipfs icore.CoreAPI, cids string, redun int, chunker string, reGenerate bool) {
	cidFile, _ := os.Create(cids)
	fmt.Printf("Uploading files with size %d B\n", size)
	coreNumber := cores
	stallchan := make(chan int)
	sendFunc := func(i int) {
		tempDir := fmt.Sprintf("./temp%d", i)
		if reGenerate {
			//remove old files
			err := os.RemoveAll(tempDir)
			if err != nil {
				fmt.Println(err.Error())
				stallchan <- i
				return
			}

			//mkdir temp dir for temp files
			err = os.MkdirAll(tempDir, os.ModePerm)
			if err != nil {
				fmt.Println(err)
				stallchan <- i
				return
			}

			//create new temp files
			for j := 0; j < number/coreNumber; j++ {
				var subs string
				subs = NewLenChars(size, StdChars)

				// if redundancy rate is set, first upload files: [:redun/100*size]
				if redun > 0 && redun <= 100 {
					rsubs := subs[:redun*size/100]
					tempfile := tempDir + "/temp"
					err := ioutil.WriteFile(tempfile, []byte(rsubs), 0666)
					if err != nil {
						fmt.Println(err.Error())
						stallchan <- i
						return
					}
					start := time.Now()
					cid, err := UploadFile(tempfile, ctx, ipfs, chunker, metrics.CMD_ProvideEach)
					if err != nil {
						fmt.Println(err.Error())
						stallchan <- i
						return
					}
					fmt.Printf("%s sub-file %f ms\n", cid.Cid(), time.Now().Sub(start).Seconds()*1000)
				}

				inputpath := tempDir + "/" + subs[:10]
				err = ioutil.WriteFile(inputpath, []byte(subs), 0666)
				if err != nil {
					fmt.Println(err.Error())
					stallchan <- i
					return
				}
			}
		}
		// if redundancy rate is set, we clear the metrics before real uploads
		if redun > 0 && redun <= 100 {
			metrics.TimersInit()
		}
		//upload temp files
		tempfiles, err := ioutil.ReadDir(tempDir)
		firstupload := true
		if err != nil {
			fmt.Println(err.Error())
			stallchan <- i
			return
		}
		for _, f := range tempfiles {
			tempFile := tempDir + "/" + f.Name()
			start := time.Now()
			//add file to ipfs local node

			//decide whether to provide this file
			provide := false
			if metrics.CMD_ProvideEach {
				provide = true
			} else {
				if metrics.CMD_ProvideFirst && firstupload {
					provide = true
					firstupload = false
				}
			}
			cid, err := UploadFile(tempFile, ctx, ipfs, chunker, provide)
			if err != nil {
				fmt.Println(err.Error())
				stallchan <- i
				return
			}
			finish := time.Now()
			fmt.Printf("%s upload %f ms\n", cid.Cid(), finish.Sub(start).Seconds()*1000)
			if err != nil {
				fmt.Println(err.Error())
				stallchan <- i
				return
			}
			_, err = io.WriteString(cidFile, strings.Split(cid.String(), "/")[2]+"\n")
			if err != nil {
				fmt.Println(err.Error())
				stallchan <- i
				return
			}
		}

		//finish
		stallchan <- i

	}
	for i := 0; i < coreNumber; i++ {
		go sendFunc(i)
	}

	stalls := coreNumber
	for {
		select {
		case <-stallchan:
			fmt.Printf("core finished\n")
			stalls--
			if stalls <= 0 {
				cidFile.Close()
				if metrics.CMD_StallAfterUpload {
					fmt.Println("Finish Front-End")
					sigChan := make(chan os.Signal)
					signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
					select {
					case <-sigChan:
					}
				}
				return
			}
		}
	}
}

func DownloadSerial(ctx context.Context, ipfs icore.CoreAPI, cids string, pag bool, np string, concurrentGet int, sad bool) {
	//peers to remove after each get
	// neighbours, err := LocalNeighbour(np)
	// if err != nil || len(neighbours) == 0 {
	// 	fmt.Printf("no neighbours file specified, will not disconnect any neighbours after geting\n")
	// }

	//known peers
	AddNeighbour(ipfs, ctx)

	//open cid file
	file, err := os.Open(cids)
	defer file.Close()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	//create tmp dir to store downloaded files
	tempDir := "./output"
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}
	// NOTE: I added some lines to split files' cid into several slices.
	//  We can get all files' cids in the 2D slices **fileCid**
	inputReader := bufio.NewReader(file)

	fileCid := make([][]string, concurrentGet)
	for i := range fileCid {
		fileCid[i] = make([]string, 0)
	}

	tmpCnt := 0
	for {
		aLine, readErr := inputReader.ReadString('\n')
		aLine = strings.TrimSuffix(aLine, "\n")
		if readErr == io.EOF {
			break
		}
		fileCid[tmpCnt%concurrentGet] = append(fileCid[tmpCnt%concurrentGet], aLine)
		tmpCnt++
	}
	var wg sync.WaitGroup
	wg.Add(concurrentGet)
	if sad {
		wg.Add(1)
	}
	for i := 0; i < concurrentGet; i++ {
		go func(theOrder int) {
			defer wg.Done()
			downTimer := gometrcs.NewTimer()
			fileSize := int64(0)
			// output file cids
			fmt.Printf("worker-%d downloading %d files\n", theOrder, len(fileCid[theOrder]))
			for j := 0; j < len(fileCid[theOrder]); j++ {
				ctx_time, _ := context.WithTimeout(ctx, 60*time.Minute)
				cid := fileCid[theOrder][j]
				p := icorepath.New(cid)
				start := time.Now()
				if metrics.CMD_EnableMetrics {
					metrics.BDMonitor.GetStartTime = start
				}
				rootNode, err := ipfs.Unixfs().Get(ctx_time, p)
				if err != nil {
					fmt.Printf("error while get %s: %s\n", cid, err.Error())
					continue
				}
				if fileSize == 0 {
					fileSize, _ = rootNode.Size()
				}
				// fmt.Println("file size: ", fileSize)
				if metrics.CMD_EnableMetrics {
					metrics.GetNode.UpdateSince(start)
				}
				startWrite := time.Now()
				err = files.WriteTo(rootNode, tempDir+"/"+cid)
				if err != nil {
					fmt.Printf("error while write to file %s : %s\n", cid, err.Error())
					continue
				}
				if metrics.CMD_EnableMetrics {
					metrics.WriteTo.UpdateSince(startWrite)
					metrics.BDMonitor.GetFinishTime = time.Now()
					//metrics.Output_Get_SingleFile()
					//metrics.BDMonitor = metrics.Newmonitor()
					metrics.CollectMonitor()
					metrics.FPMonitor.CollectFPMonitor()
				}
				downTimer.UpdateSince(start)

				if metrics.CMD_PeerRH {
					metrics.Output_PeerRH()
				}
				fmt.Printf("Thread %d get file %s %f\n", theOrder, cid, time.Now().Sub(start).Seconds()*1000)

				//provide after get
				if pag {
					err := ipfs.Dht().Provide(ctx, p)
					if err != nil {
						fmt.Printf("failed to provide file after get: %v\n", err.Error())
						return
					}
				}

				// DO NOT WORK
				//DisconnectAllPeers(ctx, ipfs)
				//remove neighbours

				if len(disconnectNeighbours) != 0 {
					for _, n := range disconnectNeighbours {
						//fmt.Printf("try to disconnect from %s\n", n)
						err := DisconnectAllPeers(ctx, ipfs, n)
						if err != nil {
							fmt.Printf("failed to disconnect: %v\n", err)
						}
					}
				}
			}
			fmt.Printf("worker-%d %s", theOrder, metrics.StandardOutput("ipfs-download", downTimer, int(fileSize)))
		}(i)
	}
	wg.Wait()
}
func FindProviderQPS(qps int, ctx context.Context, ipfs icore.CoreAPI, cidFile string, numProviders int) {

    // 打开 CID 文件
    file, err := os.Open(cidFile)
    if err != nil {
        fmt.Printf("无法打开 CID 文件: %v\n", err)
        return
    }
    defer file.Close()

    // 从 CID 文件中读取所有的 CID
    inputReader := bufio.NewReader(file)
    var cidList []string
    for {
        cidLine, readErr := inputReader.ReadString('\n')
        cidLine = strings.TrimSpace(cidLine) // 去掉换行符和多余空格
        if readErr == io.EOF {
            break
        }
        if cidLine != "" {
            cidList = append(cidList, cidLine) // 添加到 CID 列表
        }
    }

    if len(cidList) == 0 {
        fmt.Println("CID 文件中没有有效的 CID")
        return
    }
    fmt.Printf("Total CIDs: %d\n", len(cidList))

    // 用于累加所有请求的执行时间
    var totalDuration time.Duration
    var totalRequests int

    var wg sync.WaitGroup
    ticker := time.NewTicker(time.Second) // 定时器控制每秒发起的 FindProviders 请求数
    done := make(chan struct{})           // 用于主线程的等待
    var once sync.Once                    // 用于确保只关闭 done 通道一次

	// 捕捉退出信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)


    // FindProviders 请求的函数
    findProvidersFunc := func(cidStr string, index int) {
        defer wg.Done() // 请求完成后减少 WaitGroup 计数
        // fmt.Printf("Requesting CID: %s\n", cidStr)

        // 记录开始时间
        start := time.Now()

        // 解析 CID
        p := icorepath.New(cidStr)

        // 调用 FindProviders 接口，传递可变参数列表形式的选项
        pchan, err := ipfs.Dht().FindProviders(ctx, p, options.Dht.NumProviders(numProviders))
        if err != nil {
            fmt.Printf("FindProviders 请求 %d 时出错: %v\n", index, err)
            return
        }

        // 处理查找到的提供者信息
        foundProviders := 0
        for provider := range pchan {
            provider.ID.Pretty() // 模拟处理
            foundProviders++
        }

        // 记录结束时间
        duration := time.Since(start)

        // 累加执行时间
        totalDuration += duration
        totalRequests++
        // fmt.Printf("Request for CID %s finished, took %.2f ms\n", cidStr, duration.Seconds()*1000)
    }

     // 启动定时器，控制每秒发起 qps 个 FindProviders 请求
    go func() {
        waitingForCompletion := false // 引入标志位，防止重复创建等待线程

        for {
            select {
            case <-ticker.C:
                // 收集平均时间并输出
                if totalRequests > 0 {
                    averageTime := totalDuration.Seconds() * 1000 / float64(totalRequests)
                    fmt.Printf("Average Time: %.2f ms, totalCount: %d, cidlist: %d\n", averageTime, totalRequests, len(cidList))
                } else {
                    fmt.Println("No requests made yet.")
                }

                // 处理 CID 列表中的 CID
                if len(cidList) > 0 {
                    for i := 0; i < qps && len(cidList) > 0; i++ {
                        cidStr := cidList[0]
                        cidList = cidList[1:] // 移除第一个 CID
                        wg.Add(1)
                        go findProvidersFunc(cidStr, i)
                    }
                }

                // 当 cidList 为空时，只启动一次等待 Goroutine
                if len(cidList) == 0 && !waitingForCompletion {
                    waitingForCompletion = true
                    fmt.Println("All requests have been sent, waiting for completion...")

                    go func() {
                        wg.Wait() // 等待所有 Goroutine 完成
                        fmt.Println("All requests completed.")
                        once.Do(func() {
                            close(done) // 确保只关闭一次
                        })
                    }()
                }

			case <-quit: // 捕捉到退出信号时执行
                fmt.Println("Received interrupt signal, exiting immediately...")
                ticker.Stop() // 停止定时器
                once.Do(func() {
                    close(done) // 确保只关闭一次
                })
                return
            }
        }
    }()
	

    <-done // 主线程等待，直到所有请求完成

    // 计算平均执行时间
    if totalRequests > 0 {
        avgDuration := totalDuration.Seconds()*1000 / float64(totalRequests)
        fmt.Printf("所有 FindProviders 请求已完成，平均执行时间: %v ms\n", avgDuration)
    } else {
        fmt.Println("没有完成任何请求")
    }
}




func TraceUpload(index int, servers int, trace_docs string, chunker string, ipfs icore.CoreAPI, ctx context.Context) {
	// in trace workload simulation, we no longer do manually provide, those are all left to original mechanism handle
	metrics.CMD_CloseBackProvide = false
	if index < 0 || index >= servers {
		fmt.Println("Error, the index is out of range")
		return
	}
	tracefile := trace_docs
	metrics.StartBackReport()

	if traces, err := os.Open(tracefile); err != nil {
		fmt.Printf("failed to open trace file: %s, due to %s\n", tracefile, err.Error())
	} else {
		//parse and generate related file, with trace format:
		// ItemID | Popularity | Size(Bytes) | Application Type
		scanner := bufio.NewScanner(traces)
		var sizes []int
		var names []int
		for scanner.Scan() {
			line := scanner.Text()
			codes := strings.Split(line, "\t")
			size, _ := strconv.Atoi(codes[2])
			sizes = append(sizes, size)
			name, _ := strconv.Atoi(codes[0])
			names = append(names, name)
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Cannot scanner text file: %s, err: [%v]\n", tracefile, err)
			return
		}
		traces.Close()

		// temp dir
		tempDir := fmt.Sprintf("./temp%d", 0)
		//mkdir temp dir for temp files
		err := os.MkdirAll(tempDir, os.ModePerm)
		if err != nil {
			fmt.Println(err)
			return
		}

		// file saves all cids
		cidFile, _ := os.Create("ItemID-CID")

		fileNumbers := len(sizes)
		for i := 0; i < fileNumbers; i++ {
			if i%100 == 0 {
				fmt.Printf("uploading %.2f\n", float64(i)/float64(fileNumbers)*100)
			}
			if i%servers == index {
				// generate random file and save it as temp
				size := sizes[i]
				content := NewLenChars(size, StdChars)
				inputpath := tempDir + "/temp"
				//s := time.Now()
				err = ioutil.WriteFile(inputpath, []byte(content), 0666)
				if err != nil {
					fmt.Println(err.Error())
					return
				}
				//fmt.Printf("WriteFile %f\n", time.Now().Sub(s).Seconds())

				// put to ipfs store and background provide
				cid, err := UploadFile(inputpath, ctx, ipfs, chunker, false)
				// record file cid
				outline := fmt.Sprintf("%d\t%s\n", names[i], strings.Split(cid.String(), "/")[2])
				_, err = io.WriteString(cidFile, outline)
				if err != nil {
					fmt.Println(err.Error())
					return
				}

				if metrics.CMD_ProvideFirst {

				}
			}
		}
		cidFile.Close()
		fmt.Println("finished uploading")
		// stall after uploading all files until receive os signal of interrupt
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
		select {
		case <-sigChan:
		}
	}

}

func TraceDownload(traceFile string, traceDownload_randomRequest bool, ipfs icore.CoreAPI, ctx context.Context, pag bool, downloadNumber int) {
	metrics.CMD_CloseBackProvide = false

	//use different metrics, because monitor/FPMonitor those are too detailed and expensive, here we no longer need to track them
	metrics.CMD_EnableMetrics = false
	metrics.TraceDownMetricsInit()
	metrics.StartBackReport()

	if traces, err := os.Open(traceFile); err != nil {
		fmt.Printf("failed to open trace file: %s, due to %s\n", traceFile, err.Error())
		return
	} else {
		AddNeighbour(ipfs, ctx)

		// create download directory
		downloadfilepath := "./downloaded"
		if _, err := os.Stat(downloadfilepath); os.IsNotExist(err) {
			os.Mkdir(downloadfilepath, 0777)
		}

		// read ItemID-CID mapping
		mappfile, err := os.Open("ItemID-CID")
		if err != nil {
			fmt.Printf("failed to open mapping file %s, due to %s\n", "ItemID-CID", err.Error())
			return
		}
		scanner := bufio.NewScanner(mappfile)
		ItemCid := make(map[string]string)
		for scanner.Scan() {
			line := scanner.Text()
			codes := strings.Split(line, "\t")
			ItemCid[codes[0]] = codes[1]
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Cannot scanner text file: %s, err: [%v]\n", mappfile.Name(), err)
			return
		}
		mappfile.Close()

		// load workload requests
		scanner = bufio.NewScanner(traces)
		var names []string

		for scanner.Scan() {
			line := scanner.Text()
			codes := strings.Split(line, "\t")
			names = append(names, codes[1])
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Cannot scanner text file: %s, err: [%v]\n", traces.Name(), err)
			return
		}
		traces.Close()

		//randomize request order
		if traceDownload_randomRequest {
			fmt.Println("randomizing request queue")
			names = Shuffle(names)
		}

		//download according to requests
		n := len(names)
		if downloadNumber != 0 {
			n = downloadNumber
		}
		startTime := time.Now()
		for i := 0; i < n; i++ {
			toRequest := ItemCid[names[i]]
			p := icorepath.New(toRequest)
			start := time.Now()
			metrics.BDMonitor.GetStartTime = start
			rootNode, err := ipfs.Unixfs().Get(ctx, p)
			metrics.GetNode.UpdateSince(start)
			startWrite := time.Now()
			if err != nil {
				panic(fmt.Errorf("could not get file with CID: %s", err))
			}
			err = files.WriteTo(rootNode, downloadfilepath+"/"+names[i])

			if err != nil {
				panic(fmt.Errorf("could not write out the fetched CID: %s", err))
			}
			size, _ := rootNode.Size()
			metrics.DownloadedFileSize = append(metrics.DownloadedFileSize, int(size))
			metrics.AvgDownloadLatency.UpdateSince(start)
			metrics.ALL_DownloadedFileSize = append(metrics.ALL_DownloadedFileSize, int(size))
			metrics.ALL_AvgDownloadLatency.UpdateSince(start)

			metrics.WriteTo.UpdateSince(startWrite)
			metrics.BDMonitor.GetFinishTime = time.Now()
			metrics.CollectMonitor()
			metrics.FPMonitor.CollectFPMonitor()

			if pag {
				cid, err := cid.Parse(toRequest)
				if err != nil {
					fmt.Printf("failed to parse cid %s\n", err.Error())
				}
				err = ipfs.SimpleProvider().Provide(cid)
				if err != nil {
					fmt.Printf("failed to do pag %s\n", err.Error())
				}
			}
		}
		totalsize := 0
		for _, s := range metrics.ALL_DownloadedFileSize {
			totalsize += s
		}
		throughput := float64(totalsize) / 1024 / 1024 / (time.Now().Sub(startTime).Seconds())
		line := fmt.Sprintf("%s %f %d %f %f\n", time.Now().String(), throughput, metrics.ALL_AvgDownloadLatency.Count(), metrics.ALL_AvgDownloadLatency.Mean()/1000000, metrics.ALL_AvgDownloadLatency.Percentile(float64(metrics.ALL_AvgDownloadLatency.Count())*0.99)/1000000)
		fmt.Println(line)
	}
}

func handleRequest(conn net.Conn, ctx context.Context, ipfs icore.CoreAPI) {

	request := make([]byte, 1024)
	_, err := conn.Read(request)
	if err != nil {
		fmt.Println("Error reading request:", err)
		conn.Close()
		return
	}
	reqs := strings.Split(string(request), " ")
	var rep string
	switch reqs[0] {
	case "1":
		// fmt.Printf("handle file upload: %s\n", reqs[1])
		size_string := strings.Replace(reqs[1], "\x00", "", -1)
		start := time.Now()
		file_size, err := strconv.Atoi(size_string)
		if err != nil {
			fmt.Printf("error transform int: %s\n", err.Error())
			rep = "1 "
			break
		}
		//create tmp file
		tmpFile, err := os.Create("tmp_file")
		if err != nil {
			fmt.Printf("error create file: %s\n", err.Error())
			rep = "1 "
			break
		}
		defer tmpFile.Close()
		subs := NewLenChars(file_size, StdChars)
		// ioutil.WriteFile(tmpFile.Name(), []byte(subs), 0666)
		_, err = tmpFile.WriteString((subs))
		if err != nil {
			fmt.Printf("error write file: %s\n", err.Error())
			rep = "1 "
			break
		}
		//upload file to ipfs
		cid, err := UploadFile(tmpFile.Name(), ctx, ipfs, "size-262144", metrics.CMD_ProvideEach)
		if err != nil {
			fmt.Println(err.Error())
			rep = "1 "
			break
		} else {
			cid_outline := strings.Split(cid.String(), "/")[2]
			rep = "0 " + cid_outline + " "
			fmt.Printf("%s upload %f ms\n", cid.Cid(), time.Now().Sub(start).Seconds()*1000)
			if len(disconnectNeighbours) != 0 {
				for _, n := range disconnectNeighbours {
					//fmt.Printf("try to disconnect from %s\n", n)
					err := DisconnectAllPeers(ctx, ipfs, n)
					if err != nil {
						// fmt.Printf("failed to disconnect: %v\n", err)
					}
				}
			}
		}
	case "2":
		// fmt.Printf("handle file download: %s\n", reqs[1])
		start := time.Now()
		cid := reqs[1]
		cid = strings.Replace(cid, "\x00", "", -1)
		p := icorepath.New(cid)
		rootNode, err := ipfs.Unixfs().Get(ctx, p)
		if err != nil {
			fmt.Printf("error while get %s: %s\n", cid, err.Error())
			rep = "1 "
			break
		} else {
			rootget := time.Now()
			err = files.WriteTo(rootNode, "output_tmp_file")
			if err != nil {
				fmt.Printf("error while write to file %s : %s\n", cid, err.Error())
				rep = "1 "
				break
			}
			selfk, _ := ipfs.Key().Self(ctx)
			rep = "0 " + selfk.ID().Pretty()
			if metrics.CMD_PeerRH {
				metrics.Output_PeerRH()
			}
			fmt.Printf("%s download %fms (%f:%f)\n", cid, time.Now().Sub(start).Seconds()*1000, rootget.Sub(start).Seconds()*1000, time.Now().Sub(rootget).Seconds()*1000)
			if len(disconnectNeighbours) != 0 {
				for _, n := range disconnectNeighbours {
					//fmt.Printf("try to disconnect from %s\n", n)
					err := DisconnectAllPeers(ctx, ipfs, n)
					if err != nil {
						// fmt.Printf("failed to disconnect: %v\n", err)
					}
				}
			}
		}
	default:
		fmt.Printf("unrecognized op %s\n", reqs[0])
		rep = "1 "
	}

	_, err = conn.Write([]byte(rep))
	if err != nil {
		fmt.Println("Error sending result:", err)
	}
	conn.Close()
}

func startServer(ctx context.Context, ipfs icore.CoreAPI) {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()
	for {
		// 等待客户端连接
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// 处理客户端请求
		go handleRequest(conn, ctx, ipfs)
	}

}

func ipfs_backend(ctx context.Context, ipfs icore.CoreAPI) {
	fmt.Println("ipfs back_end listening on port 8080...")
	metrics.StartBackReport()
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)

	go func() {
		startServer(ctx, ipfs)
	}()
	sig := <-sigChan
	fmt.Printf("Received signal: %v\n", sig)
}

func UploadQPS(qps int, size, number int, ctx context.Context, ipfs icore.CoreAPI, cids string, redun int, chunker string, reGenerate bool) {
	cidFile, err := os.Create(cids)
	if err != nil {
		fmt.Printf("Failed to create CID file: %v", err)
	}
	defer func() {
		cidFile.Close()
		if metrics.CMD_StallAfterUpload {
			fmt.Println("Finish Front-End")
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
			select {
			case <-sigChan:
				fmt.Println("Interrupt received, shutting down.")
			case <-ctx.Done():
				fmt.Println("Context canceled, shutting down.")
			}
		}
	}()

	fmt.Printf("Uploading files with size %d B\n", size)
	stallChan := make(chan int, number) // 用于等待所有上传完成的 channel
	var totalUploadTime float64
	var totalFiles = number
	var mu sync.Mutex // 保护并发操作的锁
	cidsList := make([]string, 0)
	var uploadedFiles int
	startTime := time.Now() // 记录上传开始时间

	var wg sync.WaitGroup
	// 上传文件的具体逻辑
	sendFunc := func(i int) {
		defer wg.Done()
		// 在上传前生成随机文件数据
		fileContent := NewLenChars(size, StdChars) // 动态生成指定大小的随机数据
		start := time.Now()
		// 直接上传内存中的数据，而不需要保存到磁盘
		opts := []options.UnixfsAddOption{
			options.Unixfs.Chunker(chunker),
		}
		if metrics.CMD_ProvideEach {
			opts = append(opts, options.Unixfs.ProvideThrough())
		}

		cid, err := ipfs.Unixfs().Add(ctx, files.NewBytesFile([]byte(fileContent)), opts...)
		if err != nil {
			fmt.Printf("Error uploading file %d: %v\n", i, err)
			stallChan <- i
			return
		}

		finish := time.Now()
		uploadTime := finish.Sub(start).Seconds() * 1000

		mu.Lock()
		totalUploadTime += uploadTime
		cidsList = append(cidsList, cid.String())
		uploadedFiles++
		mu.Unlock()

		// _, err = io.WriteString(cidFile, strings.Split(cid.String(), "/")[2]+"\n")
		// if err != nil {
		// 	fmt.Println(err.Error())
		// }
		io.WriteString(cidFile, strings.Split(cid.String(), "/")[2]+"\n")

		// 只在channel未关闭时发送
        select {
        case stallChan <- i:
        default:
        }
	}

	// 启动定时器，每秒触发一次，启动 qps 个 goroutine 执行文件上传
	ticker := time.NewTicker(time.Second)
	lastUploadedFiles := 0 // 记录上一秒上传的文件数量
	go func() {
		for range ticker.C {
			mu.Lock()
			averageUploadTime := totalUploadTime / float64(uploadedFiles)
			filesUploadedThisSecond := uploadedFiles - lastUploadedFiles
			lastUploadedFiles = uploadedFiles
			throughput := filesUploadedThisSecond // 每秒上传的文件数
			mu.Unlock()

			fmt.Printf("Average upload time: %.2f ms, Throughput: %d files/sec\n", averageUploadTime, throughput)

			for i := 0; i < qps && uploadedFiles < totalFiles; i++ {
				wg.Add(1)
				go sendFunc(uploadedFiles) // 直接启动 goroutine 进行上传
			}
			if uploadedFiles >= totalFiles {
				ticker.Stop()
				wg.Wait()
				close(stallChan) // 在此关闭 channel
				return
			}
		}
	}()

	// 等待所有文件上传完成
	for range stallChan {
		if uploadedFiles >= totalFiles {
			mu.Lock()
			averageUploadTime := totalUploadTime / float64(uploadedFiles)
			mu.Unlock()

			// 计算总吞吐率
			totalTime := time.Since(startTime).Seconds() // 计算总共花费的时间（秒）
			averageThroughput := float64(uploadedFiles) / totalTime // 平均吞吐率（文件数/秒）

			fmt.Printf("All files uploaded. Average upload time: %.2f ms\n", averageUploadTime)
			fmt.Printf("Total time: %.2f seconds, Average throughput: %.2f files/sec\n", totalTime, averageThroughput)
			return
		}
	}
}



// blockchain + IPFS

// FullNode 配置结构
type Config struct {
	FullNodeIP string `json:"full_node_ip"`
	LightNodes []struct {
		IP string `json:"ip"`
	} `json:"light_nodes"`
	BlockPath string `json:"block_path"`
}

type FullNode struct {
    ipfs       icore.CoreAPI
    ctx        context.Context
    lightNodes []net.Conn
    blocks     []float64
    config     *Config  // 添加 config 字段

	totalLatency time.Duration  // 记录总延迟
    txCount      int            // 记录交易数量（成功上传的区块数量）
	start        time.Time

}
func NewFullNode(ipfs icore.CoreAPI, ctx context.Context, config *Config) *FullNode {
    return &FullNode{
        ipfs:   ipfs,
        ctx:    ctx,
        config: config,  // 初始化 config 字段
    }
}

func (fn *FullNode) createBlock(kiloBytes float64) []byte { 
	blockSize := kiloBytes * 1024
	// round the blocksize to integer

	block := make([]byte, int(math.Round(float64(blockSize))))

	// 随机填充区块内容
	rand.Read(block)
	return block
}

// 上传区块到IPFS，返回CID
func (fn *FullNode) uploadBlockToIPFS(block []byte) (string, error) {
	defer func(){
		if metrics.CMD_EnableMetrics {
			metrics.AddTimer.Update(metrics.AddDura)
			metrics.Provide.Update(metrics.ProvideDura)
			metrics.Persist.Update(metrics.PersistDura)

			dagTime := metrics.AddDura - metrics.ProvideDura - metrics.PersistDura
			metrics.Dag.Update(dagTime)

			metrics.FlatfsHasTimer.Update(metrics.FlatfsHasDura)
			metrics.FlatfsPut.Update(metrics.FlatfsPutDura)

			metrics.FlatfsPutDura = 0
			metrics.FlatfsHasDura = 0
			metrics.AddDura = 0
			metrics.ProvideDura = 0
			metrics.PersistDura = 0
		}
	}()
	start := time.Now()
	// 直接上传内存中的数据，而不需要保存到磁盘
	opts := []options.UnixfsAddOption{
		options.Unixfs.Chunker("size-262144"),
	}
	if metrics.CMD_ProvideEach {
		opts = append(opts, options.Unixfs.ProvideThrough())
	}
	cid, err := fn.ipfs.Unixfs().Add(fn.ctx, files.NewBytesFile(block), opts...)
	// cid, err := fn.ipfs.Unixfs().Add(fn.ctx, files.NewBytesFile([]byte(block)), opts...)
	if err != nil {
		fmt.Printf("Error uploading file: %v\n", err)
		return "", err
	}

	// finish := time.Now()
	// uploadTime := finish.Sub(start).Seconds() * 1000
	metrics.AddDura += time.Now().Sub(start)

	return cid.Cid().String(), nil
}
// 广播CID给所有轻节点
func (fn *FullNode) broadcastCID(cid string) {
	for _, conn := range fn.lightNodes {
		fmt.Fprintf(conn, "%s\n", cid)
	}
}

// 等待轻节点确认消息
func (fn *FullNode) waitForLightNodeConfirmations() {
	var wg sync.WaitGroup

	for _, conn := range fn.lightNodes {
		wg.Add(1) // 增加等待组计数
		go func(c net.Conn) { // 使用 goroutine 并发读取轻节点的确认
			defer wg.Done() // goroutine 结束后调用 Done
			buf := make([]byte, 1024)
			n, err := c.Read(buf)
			if err != nil {
				fmt.Println("Error reading from light node:", err)
				return
			}
			if msg := string(buf[:n]); msg == "Confirmation" {
				fmt.Println("Received confirmation from light node")
			}
		}(conn) // 将当前的连接传递给 goroutine
	}

	wg.Wait() // 等待所有 goroutine 完成
}
// 关闭 FullNode 的资源
func (fn *FullNode) shutdown() {
    for _, conn := range fn.lightNodes {
        conn.Close() // 关闭所有轻节点的连接
    }
    fmt.Println("FullNode shutdown complete.")
}

// 输出统计信息，每隔 10 秒调用一次
func (fn *FullNode) reportStats(ticker *time.Ticker) {
    for {
        select {
        case <-ticker.C:
			fmt.Println("==========================================");
                
            if fn.txCount > 0 {
                avgLatency := fn.totalLatency / time.Duration(fn.txCount) // 计算平均延迟
                tps := float64(fn.txCount) / time.Now().Sub(fn.start).Seconds()
				fmt.Printf("Average Latency: %v | TPS: %.2f\n", avgLatency, tps)
				

                // // 重置计数器和统计数据
                // fn.totalLatency = 0
                // fn.txCount = 0
            } else {
                fmt.Println("No transactions in the last 10 seconds.")
            }
			fmt.Println("==========================================");
                
        }
    }
}

func (fn *FullNode) run() {
	// 设置一个 ticker，每隔 10 秒输出一次统计信息
	ticker := time.NewTicker(10 * time.Second)
	go fn.reportStats(ticker)
	defer ticker.Stop()

	// 设置捕捉中断信号
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	fn.start = time.Now()
	fmt.Printf("%s: Starting full node...\n", time.Now().String())

	for i := 0; i < len(fn.blocks); i++ {
		// 创建管道用于接收上传结果和错误信息
		cidChan := make(chan string, 1)
		errChan := make(chan error, 1)
		
		start := time.Now() // 开始时间，用于计算延迟
		// 在 goroutine 中执行阻塞操作 (上传区块)
		go func(blockIndex int) {
			block := fn.createBlock(fn.blocks[blockIndex])
			fmt.Printf("%s: Generated block %d with size %.2f KB\n", time.Now().String(), blockIndex, fn.blocks[blockIndex])
			cid, err := fn.uploadBlockToIPFS(block)
			if err != nil {
				errChan <- err // 如果有错误，发送错误到管道
			} else {
				cidChan <- cid // 上传成功，发送 CID 到管道
			}
		}(i)

		select {
		case <-stop: // 如果收到中断信号，优雅退出
			fmt.Println("Received interrupt signal. Shutting down full node...")
			fn.shutdown() // 执行关闭操作
			return

		case cid := <-cidChan: // 正常上传完成
			fmt.Printf("%s: Uploaded block %d with CID %s\n", time.Now().String(), i, cid)
			fn.broadcastCID(cid)
			fmt.Printf("%s: Broadcasted CID to light nodes...\n", time.Now().String())
			fn.waitForLightNodeConfirmations()
			
			end := time.Now() // 上传结束时间
			duration := end.Sub(start) // 计算单次上传的延迟

			fn.totalLatency += duration   // 累加延迟
			fn.txCount++                  // 记录成功的交易数量
			fmt.Printf("%s: Received confirmations from light nodes...\n", time.Now().String())

		case err := <-errChan: // 处理上传过程中出现的错误
			fmt.Printf("Error uploading block: %v\n", err)
		// case <-time.After(5 * time.Second): // 超时处理
		// 	fmt.Printf("Timeout while uploading block %d.\n", i)
		}
	}
}


// // 运行全节点逻辑：生成区块、上传并广播
// func (fn *FullNode) run() {
// 	// 设置一个 ticker，每隔 10 秒输出一次统计信息
//     ticker := time.NewTicker(10 * time.Second)
//     go fn.reportStats(ticker)
// 	defer ticker.Stop()

// 	// 设置捕捉中断信号
//     stop := make(chan os.Signal, 1)
//     signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

// 	fn.start = time.Now()
// 	fmt.Printf("%s: Starting full node...\n", time.Now().String())
// 	for i := 0; i < len(fn.blocks); i++ {
//         select {
//         case <-stop: // 如果收到中断信号，优雅退出
//             fmt.Println("Received interrupt signal. Shutting down full node...")
// 			fn.shutdown() // 执行关闭操作
//             return
//         default:
// 			start := time.Now() // 开始时间，用于计算延迟
//             block := fn.createBlock(fn.blocks[i])
//             fmt.Printf("%s: Generated block %d with size %.2f KB\n", time.Now().String(), i, fn.blocks[i])
//             cid, err := fn.uploadBlockToIPFS(block)
//             if err != nil {
//                 fmt.Println("Error uploading block to IPFS:", err)
//                 continue
//             }
//             fmt.Printf("%s: Uploaded block %d with CID %s\n", time.Now().String(), i, cid)

//             fmt.Printf("Block uploaded with CID: %s\n", cid)
//             fn.broadcastCID(cid)
//             fmt.Printf("%s: Broadcasted CID to light nodes...\n", time.Now().String())
//             fn.waitForLightNodeConfirmations()
//             fmt.Printf("%s: Received confirmations from light nodes...\n", time.Now().String())
// 			end := time.Now() // 上传结束时间
// 			duration := end.Sub(start) // 计算单次上传的延迟
// 			fn.totalLatency += duration   // 累加延迟
// 			fn.txCount++                  // 记录成功的交易数量
//         }
//     }

// }

func (fn *FullNode) addLightNode(conn net.Conn) {
	fmt.Printf("New light node: %s\n", conn.RemoteAddr())
	fn.lightNodes = append(fn.lightNodes, conn)
}

// 读取配置文件
func loadConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// 从指定路径读取文件中每一行，每一行的格式为 "txs size", txs是一个整数，size是一个浮点数，返回读取的size
func readSizesFromFile(filePath string) ([]float64, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var sizes []float64
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        fields := strings.Fields(line)
        
        if len(fields) >= 2 { // 确保有足够的字段
            size, err := strconv.ParseFloat(fields[1], 64) // 解析第二个字段为浮点数
            if err != nil {
                return nil, fmt.Errorf("failed to parse size: %v", err)
            }
            sizes = append(sizes, size) // 将解析后的浮点数追加到 sizes 列表
        }
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("error reading file: %v", err)
    }

    return sizes, nil
}

// 接受轻节点的连接
func (fn *FullNode) acceptLightNodeConnection() (net.Conn, error) {
    ln, err := net.Listen("tcp", fn.config.FullNodeIP)
    if err != nil {
        return nil, fmt.Errorf("Error starting listener on %s: %v", fn.config.FullNodeIP, err)
    }
    defer ln.Close()

    // 接受连接
    conn, err := ln.Accept()
    if err != nil {
        return nil, err
    }
    return conn, nil
}

// 等待所有轻节点连接
func (fn *FullNode) waitForLightNodeConnections() {
    fmt.Println("Waiting for all light nodes to connect...")

    expectedLightNodes := make(map[string]bool)
    for _, ln := range fn.config.LightNodes {
        expectedLightNodes[ln.IP] = false // 初始化为 false，表示尚未连接
    }
	// 打印expectedLightNodes
	fmt.Println("Expected light nodes:")
	for ip, connected := range expectedLightNodes {
        fmt.Printf("%s: %t\n", ip, connected)
    }

    // 启动一个监听器
    ln, err := net.Listen("tcp", fn.config.FullNodeIP)
    if err != nil {
        fmt.Printf("Error starting listener on %s: %v\n", fn.config.FullNodeIP, err)
        return
    }
    defer ln.Close() // 在所有节点连接后关闭监听器

    for {
        // 检查是否所有的轻节点都已连接
        allConnected := true
        for _, connected := range expectedLightNodes {
            if !connected {
                allConnected = false
                break
            }
        }

        if allConnected {
            fmt.Println("All light nodes are connected.")
            return // 当所有节点连接时，退出等待
        }

        // 等待连接的轻节点
        conn, err := ln.Accept()
        if err != nil {
            fmt.Println("Error accepting connection:", err)
            continue
        }

        // 获取轻节点的 IP 地址
        remoteAddr := conn.RemoteAddr().String()
        ip := strings.Split(remoteAddr, ":")[0]

        // 检查该连接是否在配置文件中
        if _, exists := expectedLightNodes[ip]; exists {
            fn.lightNodes = append(fn.lightNodes, conn) // 将连接存储到 slice 中
            expectedLightNodes[ip] = true // 标记该 IP 的轻节点已经连接
            fmt.Printf("Light node %s connected.\n", ip)
        } else {
            fmt.Printf("Received connection from unknown node: %s\n", ip)
            conn.Close() // 关闭未知节点的连接
        }
    }
}
func FullNodeMain(ipfs icore.CoreAPI, ctx context.Context, configFile string) {
	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Printf("Error loading config file: %v\n", err)
		return
	}
	block_size, err := readSizesFromFile(config.BlockPath)
	if err != nil {
		fmt.Printf("Error reading block sizes from file: %v\n", err)
		return
	}
	// 初始化全节点，并传递 config 对象
    fullNode := NewFullNode(ipfs, ctx, config)
	for _, size := range block_size {
		fullNode.blocks = append(fullNode.blocks, size)
	}

	fullNode.waitForLightNodeConnections()

	// ln, err := net.Listen("tcp", config.FullNodeIP)
	// if err != nil {
	// 	fmt.Printf("Error starting listener on %s: %v\n", config.FullNodeIP, err)
	// 	return
	// }
	// defer ln.Close()

	// fmt.Printf("Full node listening on %s...\n", config.FullNodeIP)

	// // 接受轻节点的连接
	// go func() {
	// 	for {
	// 		conn, err := ln.Accept()
	// 		if err != nil {
	// 			fmt.Println("Error accepting connection:", err)
	// 			continue
	// 		}
	// 		fullNode.addLightNode(conn)
	// 	}
	// }()

	// 开始全节点区块生成和广播流程
	fullNode.run()
}

type LightNode struct {
	ipfs icore.CoreAPI
	conn net.Conn
	ctx context.Context
}
func NewLightNode(ipfs icore.CoreAPI, ctx context.Context, fullNodeAddr string) *LightNode {
	conn, err := net.Dial("tcp", fullNodeAddr)
	if err != nil {
		fmt.Println("Error connecting to full node:", err)
		return nil
	}
	fmt.Printf("Connected to full node at %s\n", fullNodeAddr)

	return &LightNode{
		ipfs: ipfs,
		conn: conn,
		ctx: ctx,
	}
}


// 关闭 LightNode 的资源
func (ln *LightNode) shutdown() {
    ln.conn.Close() // 关闭与全节点的连接
    fmt.Println("LightNode shutdown complete.")
}

// 监听全节点发送的CID消息，下载区块并验证
func (ln *LightNode) listenForBlocks() {
	tempDir := "./output"
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}
	// 设置捕捉中断信号
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)


	downTimer := gometrcs.NewTimer()
	reader := bufio.NewReader(ln.conn)
	for {
		cidChan := make(chan string, 1)
        errChan := make(chan error, 1)

        // 在新的 goroutine 中执行阻塞的 I/O 操作
        go func() {
            cid, err := reader.ReadString('\n')
            if err != nil {
                errChan <- err
            } else {
                cidChan <- strings.TrimSpace(cid)
            }
        }()

        select {
        case <-stop: // 收到中断信号时优雅退出
            fmt.Println("Received interrupt signal. Shutting down light node...")
            ln.shutdown() // 执行关闭操作
            return
        case cid := <-cidChan: // 正常读取 CID
            fmt.Printf("Received CID: %s\n", cid)
            cid = strings.TrimSpace(cid)
            fmt.Printf("%s: Received CID from full node: %s\n", time.Now().String(), cid)

            ctx_time, cancel := context.WithTimeout(ln.ctx, 60*time.Minute)
            defer cancel()
            p := icorepath.New(cid)
            start := time.Now()
            if metrics.CMD_EnableMetrics {
                metrics.BDMonitor.GetStartTime = start
            }
            rootNode, err := ln.ipfs.Unixfs().Get(ctx_time, p)
            if err != nil {
                fmt.Printf("error while get %s: %s\n", cid, err.Error())
                continue
            }
            if metrics.CMD_EnableMetrics {
                metrics.GetNode.UpdateSince(start)
            }
            startWrite := time.Now()
            err = files.WriteTo(rootNode, tempDir+"/"+cid)
            if err != nil {
                fmt.Printf("error while write to file %s : %s\n", cid, err.Error())
                continue
            }
			// fmt.Printf("%s: Got blocks for CID %s\n", time.Now().String(), cid)
            if metrics.CMD_EnableMetrics {
                metrics.WriteTo.UpdateSince(startWrite)
                metrics.BDMonitor.GetFinishTime = time.Now()
                metrics.CollectMonitor()
                metrics.FPMonitor.CollectFPMonitor()
            }
            downTimer.UpdateSince(start)
			if len(disconnectNeighbours) != 0 {
				for _, n := range disconnectNeighbours {
					//fmt.Printf("try to disconnect from %s\n", n)
					err := DisconnectAllPeers(ln.ctx, ln.ipfs, n)
					if err != nil {
						fmt.Printf("failed to disconnect: %v\n", err)
					}
				}
			}
            // 验证区块
            fmt.Printf("%s: Block verified. Sending confirmation to full node.\n", time.Now().String())
            ln.sendConfirmation()
        case err := <-errChan: // 处理读取错误
            fmt.Printf("Error reading CID: %v\n", err)
        }
    }
}

// 发送确认消息给全节点
// 发送确认消息给全节点
func (ln *LightNode) sendConfirmation() {
    confirmationMessage := "Confirmation"
    _, err := fmt.Fprintf(ln.conn, "%s\n", confirmationMessage) // 发送确认消息
    if err != nil {
        fmt.Printf("Error sending confirmation to full node: %v\n", err)
    } else {
        fmt.Printf("Sent confirmation to full node: %s\n", confirmationMessage)
    }
}
func LightNodeMain(ipfs icore.CoreAPI, ctx context.Context, configFile string) {
	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Printf("Error loading config file: %v\n", err)
		return
	}
	lightNode := NewLightNode(ipfs, ctx, config.FullNodeIP)
	if lightNode == nil {
		return
	}

	// 开始监听全节点广播的区块CID
	lightNode.listenForBlocks()
}

var disconnectNeighbours []string
var coworker bool

func main() {

	//read config option
	flag.BoolVar(&(metrics.CMD_CloseBackProvide), "closebackprovide", false, "wether to close background provider")
	flag.BoolVar(&(metrics.CMD_CloseLANDHT), "closelan", false, "whether to close lan dht")
	flag.BoolVar(&(metrics.CMD_CloseDHTRefresh), "closedhtrefresh", false, "whether to close dht refresh")
	flag.BoolVar(&(metrics.CMD_EnableMetrics), "enablemetrics", false, "whether to enable metrics")
	flag.BoolVar(&(metrics.CMD_ProvideFirst), "providefirst", false, "manually provide first file after upload")
	flag.BoolVar(&(metrics.CMD_ProvideEach), "provideeach", false, "manually provide(Provide_Through, the default IPFS Provide uses a Provide_Back strategy) every files after upload")
	flag.BoolVar(&(metrics.CMD_StallAfterUpload), "stallafterupload", false, "stall after upload")

	// expDHT:
	flag.BoolVar(&(metrics.CMD_PeerRH), "PeerRH", false, "Whether to enable PeerResponseHistory")
	// flag.BoolVar(&(metrics.CMD_LoadSaveCache), "loadsavecache", false, "Whether to load & save PeerResponseHistory")

	var cmd string
	var filesize int
	var sizestring string
	var filenumber int
	var parallel int
	var cidfile string
	var provideAfterGet bool

	var ipfsPath string
	var redun_rate int
	var seelogs string
	var chunker string    // NOTE: added a chunker option
	var concurrentGet int // NOTE: added a option for the number of threads to concurrent get files.
	var traceDownload_randomRequest bool

	var index int
	var servers int
	var traceFile string
	var downloadNumber int
	var rmNeighbourPath string
	var stallafterdownload = false

	var serach_provider_number int

	var qps int
	var bitcoin_config_path string

	flag.IntVar(&redun_rate, "redun", 0, "The redundancy of the file when Benchmarking upload, 100 indicates that there is exactly the same file in the node, 0 means there is no existence of same file.(default 0)")
	flag.StringVar(&cmd, "c", "", "operation type\n"+
		"upload: upload files to ipfs, with -s for file size, -n for file number, -p for concurrent upload threads, -cid for specified uploaded file cid stored\n"+
		"downloads: download file following specified cid file with single thread, -pag provide file after get, -np path to the file of neighbours which will be disconnected after each get\n"+
		"daemon: run ipfs daemon\n"+
		"traceUpload: upload generated trace files, return ItemID-Cid mapping\n"+
		"traceDownload: download according to workload trace file and ItemID-CID mapping\n")
	flag.StringVar(&cidfile, "cid", "cid", "name of cid file for uploading")

	flag.StringVar(&sizestring, "s", "262144", "file size, for example: 256k, 64m, 1024")
	flag.IntVar(&filenumber, "n", 1, "file number")
	flag.IntVar(&parallel, "p", 1, "concurrent operation number")
	flag.IntVar(&qps, "qps", 1, "Query per second")

	flag.BoolVar(&provideAfterGet, "pag", false, "whether to provide file after get it")

	flag.StringVar(&rmNeighbourPath, "rmn", "remove_neighbours", "the path of file that records neighbours id, neighbours will be removed after getting file")

	flag.StringVar(&ipfsPath, "ipfs", "./go-ipfs/cmd/ipfs/ipfs", "where go-ipfs exec exists")
	flag.StringVar(&seelogs, "seelogs", "", "configure the specified log level to 'debug', logs connect with'-', such as 'dht-bitswap-blockservice'")

	// NOTE: Added two option.
	// TODO: current monitor of Get-Breakdown are global, and not sufficiently tested in multi-threaded environment, may exists problems
	flag.IntVar(&concurrentGet, "cg", 1, "concurrent get number")
	flag.StringVar(&chunker, "chunker", "size-262144", "customized chunker")

	// for trace workload testing
	flag.StringVar(&traceFile, "f", "", "file indicates the path of doc file of generated trace")
	flag.IntVar(&index, "i", 0, "given each server has a part of entire workload, index indicates current server own the i-th part of data. Index starts from 0.")
	flag.IntVar(&servers, "servers", 1, "servers indicates the total number of servers")
	flag.BoolVar(&traceDownload_randomRequest, "randomRequest", false, "random request means that current client will randomly reorder requests from generated workload")

	flag.IntVar(&(metrics.ProviderWorker), "pw", 8, "Speed up IPFS by increasing the number of ProviderWorker, higher pw will increase provide bandwidth but brings higher memory overhead."+
		"As tested, with a pw=80, eac=3, Provides/Min can reach 100+")
	flag.BoolVar(&(metrics.CMD_FastSync), "fastsync", false, "Speed up IPFS by skipping some overcautious synchronization: 1. skip flat-fs synchronizing files, 2. skip leveldb synchronizing files")
	flag.BoolVar(&(metrics.CMD_EarlyAbort), "earlyabort", false, "Speed up IPFS by early abort during findCloserPeers. "+
		"For each ProviderWorker, it checks termination condition before send request to current nearest peer, "+
		"we add a condition to check whether the smallest CPL of current top K nearst peers hasn't be updated for a few rounds."+
		"If so, we do early abort.")
	flag.IntVar(&(metrics.EarlyAbortCheck), "eac", 5, "the number of former min cpl checked to be determined early abort. The smaller eac is, the faster Provide can achieve."+
		"But, as tested, when eac=0, some files cannot be found. eac should not be smaller than 3)")
	flag.IntVar(&downloadNumber, "dn", 0, "")

	var ReGenerateFile bool
	flag.BoolVar(&ReGenerateFile, "regenerate", false, "whether to regenerate random-content files for uploading. If not flagged, the upload will read local ")

	flag.IntVar(&(metrics.BlockSizeLimit), "blocksizelimit", 1024*1024, "chunk size")

	flag.BoolVar(&(metrics.EnablePbitswap), "enablepbitswap", false, "whether to enable pbitswap(re-order blk request sequence for each provider, load-balanced request batch, find co-workers from providers). "+
		"Note that if enable pbitswap the metrics will be no longer accurate.")
	flag.BoolVar(&(metrics.CMD_DisCoWorer), "discoworker", false, "whether to enable CoWorer")
	flag.BoolVar(&(metrics.CMD_PBitswap_Ticker), "pbticker", false, "whether to enable pbitswap ticker which periodically queries providers. This it is beneficial when the number of providers is low in the network.")

	flag.Float64Var(&(metrics.B), "B", 0.95, "parameter for ax + by")

	flag.BoolVar(&stallafterdownload, "sad", false, "stall after download")
	flag.IntVar(&(metrics.QueryPeerTime), "qpt", 60, "query peer time")
	flag.BoolVar(&(metrics.CMD_NoneNeighbourAsking), "nna", false, "skip NeighbourAsking")

	flag.IntVar(&serach_provider_number, "spn", 1, "search provider number")
	flag.StringVar(&bitcoin_config_path, "bc", "bitcoin_config", "path to bitcoin config file")

	flag.Parse()

	if metrics.EnablePbitswap {
		fmt.Printf("pbitswap is enabled\n")
		if metrics.CMD_DisCoWorer {
			fmt.Printf("CoWorer is Disabled\n")
		}
		if metrics.CMD_PBitswap_Ticker {
			fmt.Printf("PBitswap Ticker is Enabled\n")
		}
	}

	// NOTE: check the concurrentGet.
	if concurrentGet <= 0 {
		fmt.Printf("Bad parameter for the concurrerntGet. It should be an integer belong to [1, )")
		return
	}
	// file size
	base, _ := strconv.Atoi(sizestring[:len(sizestring)-1])
	if strings.HasSuffix(sizestring, "k") {
		filesize = base * 1024
	} else if strings.HasSuffix(sizestring, "m") {
		filesize = base * 1024 * 1024
	} else {
		filesize, _ = strconv.Atoi(sizestring[:len(sizestring)])
	}

	//metrics
	if metrics.CMD_EnableMetrics {
		metrics.TimersInit()
		defer func() {
			metrics.Output_addBreakdown()
			metrics.Output_Get()
			metrics.Output_FP()
			metrics.Output_PeerRH()
			metrics.OutputMetrics0()
			metrics.Output_ProvideMonitor()
		}()
	}

	if metrics.CMD_PeerRH {
		fmt.Println("PeerRH is enabled")
		// 假设一个cacheline 需要 200 个字节，那么我们让最多设置 5e6 个 cacheline
		// 此时需要 1GB 内存，为了测试方便，我们先设置如上个数
		metrics.GPeerRH = metrics.NewPeerRH(1, metrics.B, 5*1e6) // 历史信息不起作用
		//GPeerRH = NewPeerRH(1, 1) // 历史信息与逻辑距离 1:1
		metrics.GPeerRH.Load()
		defer metrics.GPeerRH.Store()
	}else {
		fmt.Println("PeerRH is disabled")
	}
	//see logs
	if len(seelogs) > 0 {
		sublogs := strings.Split(seelogs, "-")
		fmt.Println("See logs: ")
		for _, s := range sublogs {
			fmt.Println("--" + s)
			if s == "getbreakdown" {
				metrics.GetBreakDownLog = true
				continue
			}
			if s == "seecpl" {
				metrics.CPLInDHTQureyLog = true
				logging.SetLogLevel(s, "debug")
				continue
			}

			err := logging.SetLogLevel(s, "debug")
			if err != nil {
				fmt.Println("failed to set log level of: " + s + " ." + err.Error())
			}
		}
	}

	neighbours, err := LocalNeighbour(rmNeighbourPath)
	if err != nil || len(neighbours) == 0 {
		fmt.Printf("no neighbours file specified, will not disconnect any neighbours after geting\n")
	} else {
		fmt.Printf("following peers will be manully disconnected after each GET: \n")
		for _, n := range neighbours {
			fmt.Printf("	%s\n", n)
			disconnectNeighbours = append(disconnectNeighbours, n)
		}
	}

	if metrics.CMD_EarlyAbort {
		metrics.LastFewProvides = metrics.NewQueue(1024)
	}
	if cmd == "upload" {
		ctx, ipfs, cancel := Ini()

		defer cancel()
		// NOTE: I modified the function for adding a ** chunker ** .
		Upload(filesize, filenumber, parallel, ctx, ipfs, cidfile, redun_rate, chunker, ReGenerateFile)
		return
	}
	if cmd == "downloads" {
		ctx, ipfs, cancel := Ini()
		defer cancel()
		DownloadSerial(ctx, ipfs, cidfile, provideAfterGet, rmNeighbourPath, concurrentGet, stallafterdownload)
		return
	}
	if cmd == "findproviderqps" {
		ctx, ipfs, cancel := Ini()
		defer cancel()
		FindProviderQPS(qps, ctx, ipfs, cidfile, serach_provider_number)
		return 
	}

	if cmd == "uploadqps"{
		ctx, ipfs, cancel := Ini()
		defer cancel()
		UploadQPS(qps, filesize, filenumber, ctx, ipfs, cidfile, redun_rate, chunker, ReGenerateFile)
		return
	}
	if cmd == "daemon" {
		cmd := exec.Command(ipfsPath, "daemon")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Println("cmd.StdoutPipe: ", err)
			return
		}

		err = cmd.Start()
		if err != nil {
			fmt.Println("failed to exec ipfs daemon: ", err.Error())
			return
		}
		reader := bufio.NewReader(stdout)
		for {
			line, err2 := reader.ReadString('\n')
			if err2 != nil || io.EOF == err2 {
				break
			}
			fmt.Printf(line)
		}
		err = cmd.Wait()
		if err != nil {
			return
		}

	}
	if cmd == "traceUpload" {
		ctx, ipfs, cancel := Ini()
		defer cancel()
		TraceUpload(index, servers, traceFile, chunker, ipfs, ctx)
		return
	}
	if cmd == "traceDownload" {
		ctx, ipfs, cancel := Ini()
		defer cancel()
		TraceDownload(traceFile, traceDownload_randomRequest, ipfs, ctx, provideAfterGet, downloadNumber)
		return
	}
	if cmd == "ipfsbackend" {
		ctx, ipfs, cancel := Ini()
		defer cancel()
		ipfs_backend(ctx, ipfs)
		return
	}
	if cmd=="fullnode"{
		fmt.Println("fullnode")
		ctx, ipfs, cancel := Ini()
		defer cancel()
		FullNodeMain(ipfs, ctx, bitcoin_config_path)
		return
	}
	if cmd=="lightnode"{
		fmt.Println("lightnode")
		ctx, ipfs, cancel := Ini()
		defer cancel()
		LightNodeMain(ipfs, ctx, bitcoin_config_path)
		return
	}
	_, _, cancel := Ini()
	defer cancel()
	stall := make(chan int, 1)
	select {
	case s := <-stall:
		fmt.Println(s)
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

func LocalNeighbour(np string) ([]string, error) {
	var neighbours []string

	filename := np
	file, err := os.Open(filename)
	if err != nil {
		return neighbours, fmt.Errorf("failed to open neighbour file: %v\n", err)
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		neighbours = append(neighbours, line)
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Cannot scanner text file: %s, err: [%v]\n", file.Name(), err)
		return nil, err
	}
	file.Close()
	fmt.Printf("Load Neighbours to remove: %v\n", neighbours)
	return neighbours, nil
}

func DisconnectAllPeers(ctx context.Context, ipfs icore.CoreAPI, remove string) error {
	//peerInfos := make(map[peer.ID]*peerstore.PeerInfo, len(peers))
	//var peerInfos map[peer.ID]*peerstore.PeerInfo
	peerInfos, err := ipfs.Swarm().Peers(ctx)
	if err != nil {
		//fmt.Println(err.Error())
		return err
	}
	for _, con := range peerInfos {
		if con.ID().String() != remove {
			// continue
		}
		ci := peer.AddrInfo{
			Addrs: []multiaddr.Multiaddr{con.Address()},
			ID:    con.ID(),
		}

		addrs, err := peer.AddrInfoToP2pAddrs(&ci)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		// fmt.Printf("disconnect from %v\n", addrs)

		for _, addr := range addrs {
			err = ipfs.Swarm().Disconnect(ctx, addr)
			if err != nil {
				fmt.Printf("error while disconnect %s\n", err.Error())
				break
			}
		}
	}

	return nil
}

func Shuffle(vals []string) []string {
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
	ret := make([]string, len(vals))
	perm := r.Perm(len(vals))
	for i, randIndex := range perm {
		ret[i] = vals[randIndex]
	}
	return ret
}
func BytesToInt64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}

func AddNeighbour(ipfs icore.CoreAPI, ctx context.Context) {
	//manually add neighbours
	addNeighbourPath := "add_neighbours"
	addnf, err := os.Open(addNeighbourPath)
	if err == nil {
		fmt.Printf("Adding Neighbours:\n")
		scanner := bufio.NewScanner(addnf)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Printf("    %s\n", line)
			ma, err := multiaddr.NewMultiaddr(line)
			if err != nil {
				fmt.Printf("    failed to parse multiAddress, due to %s\n", err.Error())
			} else {

				addrs, err := peer.AddrInfoFromP2pAddr(ma)
				if err != nil {
					fmt.Printf("    failed to transform multiaddr to addrInfo, due to %s\n", err.Error())
				} else {
					err := ipfs.Swarm().Connect(ctx, *addrs)
					if err != nil {
						fmt.Printf("    failed to swarm connect to peer, due to %s\n", err.Error())
					}
				}
			}
		}

		addnf.Close()
	}
}
