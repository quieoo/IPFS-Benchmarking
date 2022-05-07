package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/ipfs/go-cid"
	_ "github.com/ipfs/go-cid"
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

	gometrcs "github.com/rcrowley/go-metrics"
	"math/rand"
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
					cid, err := UploadFile(tempfile, ctx, ipfs, chunker, false)
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
		if !metrics.CMD_StallAfterUpload {
			stallchan <- i
		}
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
				return
			}
		}
	}
}

func DownloadSerial(ctx context.Context, ipfs icore.CoreAPI, cids string, pag bool, np string, concurrentGet int) {
	//peers to remove after each get
	neighbours, err := LocalNeighbour(np)
	if err != nil || len(neighbours) == 0 {
		fmt.Printf("no neighbours file specified, will not disconnect any neighbours after geting\n")
	}

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
	for i := 0; i < concurrentGet; i++ {
		go func(theOrder int) {
			defer wg.Done()
			downTimer := gometrcs.NewTimer()
			fileSize := int64(0)
			for j := 0; j < len(fileCid[theOrder]); j++ {
				cid := fileCid[theOrder][j]
				p := icorepath.New(cid)
				start := time.Now()
				if metrics.CMD_EnableMetrics {
					metrics.BDMonitor.GetStartTime = start
				}
				rootNode, err := ipfs.Unixfs().Get(ctx, p)
				if fileSize == 0 {
					fileSize, _ = rootNode.Size()
				}
				if metrics.CMD_EnableMetrics {
					metrics.GetNode.UpdateSince(start)
				}
				startWrite := time.Now()
				if err != nil {
					panic(fmt.Errorf("could not get file with CID: %s", err))
				}
				err = files.WriteTo(rootNode, tempDir+"/"+cid)
				if err != nil {
					panic(fmt.Errorf("could not write out the fetched CID: %s", err))
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

				fmt.Printf("Thread %d get file %s %f\n", theOrder, cid, time.Now().Sub(start).Seconds()*1000)
				//provide after get
				if pag {
					err := ipfs.Dht().Provide(ctx, p)
					if err != nil {
						fmt.Printf("failed to provide file after get: %v\n", err.Error())
						return
					}
				}

				// DisconnectAllPeers(ctx, ipfs)
				//remove neighbours
				if len(neighbours) != 0 {
					for _, n := range neighbours {
						//fmt.Printf("try to disconnect from %s\n", n)
						err := DisconnectToPeers(ctx, ipfs, n)
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

func main() {

	//read config option
	flag.BoolVar(&(metrics.CMD_CloseBackProvide), "closebackprovide", false, "wether to close background provider")
	flag.BoolVar(&(metrics.CMD_CloseLANDHT), "closelan", false, "whether to close lan dht")
	flag.BoolVar(&(metrics.CMD_CloseDHTRefresh), "closedhtrefresh", false, "whether to close dht refresh")
	flag.BoolVar(&(metrics.CMD_EnableMetrics), "enablemetrics", false, "whether to enable metrics")
	flag.BoolVar(&(metrics.CMD_ProvideFirst), "providefirst", false, "manually provide first file after upload")
	flag.BoolVar(&(metrics.CMD_ProvideEach), "provideeach", false, "manually provide(Provide_Through, the default IPFS Provide uses a Provide_Back strategy) every files after upload")
	flag.BoolVar(&(metrics.CMD_StallAfterUpload), "stallafterupload", false, "stall after upload")

	var cmd string
	var filesize int
	var sizestring string
	var filenumber int
	var parallel int
	var cidfile string
	var provideAfterGet bool
	var rmNeighbourPath string

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

	flag.Parse()

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
			metrics.OutputMetrics0()
			metrics.Output_ProvideMonitor()
		}()
	}

	//see logs
	if len(seelogs) > 0 {
		sublogs := strings.Split(seelogs, "-")
		fmt.Println("See logs: ")
		for _, s := range sublogs {
			if s == "getbreakdown" {
				metrics.GetBreakDownLog = true
				continue
			}
			fmt.Println("--" + s)
			err := logging.SetLogLevel(s, "debug")
			if err != nil {
				fmt.Println("failed to set log level of: " + s + " ." + err.Error())
			}
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
		DownloadSerial(ctx, ipfs, cidfile, provideAfterGet, rmNeighbourPath, concurrentGet)
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

func DisconnectToPeers(ctx context.Context, ipfs icore.CoreAPI, remove string) error {
	//peerInfos := make(map[peer.ID]*peerstore.PeerInfo, len(peers))
	//var peerInfos map[peer.ID]*peerstore.PeerInfo
	peerInfos, err := ipfs.Swarm().Peers(ctx)
	if err != nil {
		//fmt.Println(err.Error())
		return err
	}
	for _, con := range peerInfos {
		if con.ID().String() != remove {
			continue
		}
		ci := peer.AddrInfo{
			Addrs: []multiaddr.Multiaddr{con.Address()},
			ID:    con.ID(),
		}

		addrs, err := peer.AddrInfoToP2pAddrs(&ci)
		if err != nil {
			fmt.Println(err.Error())
		}
		//fmt.Printf("disconnect from %v\n", addrs)

		for _, addr := range addrs {
			err = ipfs.Swarm().Disconnect(ctx, addr)
			if err != nil {
				return err
			}
		}
		break
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
