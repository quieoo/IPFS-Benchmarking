package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	_ "github.com/ipfs/go-cid"
	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	icorepath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"io"
	"io/ioutil"
	"metrics"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"crypto/rand"
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

func Ini() (context.Context, icore.CoreAPI, context.CancelFunc) {
	fmt.Println("-- Getting an IPFS node running -- ")

	ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	/*
		// Spawn a node using the default path (~/.ipfs), assuming that a repo exists there already
		fmt.Println("Spawning node on default repo")
		ipfs, err := spawnDefault(ctx)
		if err != nil {
			fmt.Println("No IPFS repo available on the default path")
		}
	*/

	// Spawn a node using a temporary path, creating a temporary repo for the run
	fmt.Println("Spawning node on a temporary repo")
	ipfs, err := spawnEphemeral(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

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
func UploadFile(file string, ctx context.Context, ipfs icore.CoreAPI, chunker string) (icorepath.Resolved, error) {
	ustart := time.Now()
	defer func() {
		metrics.UploadDura += time.Now().Sub(ustart)

		metrics.UploadTimer.Update(metrics.UploadDura - metrics.AddDura)
		metrics.AddTimer.Update(metrics.AddDura)
		metrics.Provide.Update(metrics.ProvideDura)
		metrics.Persist.Update(metrics.PersistDura)

		dagTime := metrics.AddDura - metrics.ProvideDura - metrics.PersistDura
		metrics.Dag.Update(dagTime)

		metrics.FlatfsHasTimer.Update(metrics.FlatfsHasDura)
		metrics.FlatfsPut.Update(metrics.FlatfsPutDura)

		metrics.FlatfsPutDura = 0
		metrics.FlatfsHasDura = 0
		metrics.UploadDura = 0
		metrics.AddDura = 0
		metrics.ProvideDura = 0
		metrics.PersistDura = 0
	}()
	somefile, err := getUnixfsNode(file)
	if err != nil {
		return nil, err
	}

	// NOTE: I added an option for setting the chunker.
	opts := []options.UnixfsAddOption{
		options.Unixfs.Chunker(chunker),
	}
	start := time.Now()
	cid, err := ipfs.Unixfs().Add(ctx, somefile, opts)
	metrics.AddDura += time.Now().Sub(start)

	// TODO: it is possibly better to add a para in the function para. It would effect
	//  the speed if we do a test for adding files locally. I didn't comment these lines
	//  for expr2~4 working normally.
	cid, err := ipfs.Unixfs().Add(ctx, somefile, opts...)
	p := icorepath.New(cid.String())
	startProvide := time.Now()
	err = ipfs.Dht().Provide(ctx, p)
	metrics.AddProvideTimer.UpdateSince(startProvide)

	if err != nil {
		fmt.Println("failed to upload file in function : UploadFile")
		return nil, err
	}

	//quieoo.AddTimer.UpdateSince(start)
	return cid, err
}


// NOTE: I modified the function for adding a para named chunker.
func Upload(size, number, cores int, ctx context.Context, ipfs icore.CoreAPI, cids string, redun int, chunker string) {
	cidFile, _ := os.Create(cids)
	fmt.Printf("Uploading files with size %d B\n", size)
	coreNumber := cores
	stallchan := make(chan int)
	sendFunc := func(i int) {
		tempDir := fmt.Sprintf("./temp%d", i)
		//mkdir temp dir for temp files
		err := os.MkdirAll(tempDir, os.ModePerm)
		if err != nil {
			fmt.Println(err)
			stallchan <- i
			return
		}

		//create temp files
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
				cid, err := UploadFile(tempfile, ctx, ipfs)
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
		// if redundancy rate is set, we clear the metrics before real uploads
		if redun > 0 && redun <= 100 {
			metrics.TimersInit()
		}
		//upload temp files
		tempfiles, err := ioutil.ReadDir(tempDir)
		if err != nil {
			fmt.Println(err.Error())
			stallchan <- i
			return
		}
		for _, f := range tempfiles {
			tempFile := tempDir + "/" + f.Name()
			start := time.Now()

			// NOTE：I modified the func here adding a chunker para.
			cid, err := UploadFile(tempFile, ctx, ipfs, chunker)
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

		//remove temp dir
		err = os.RemoveAll(tempDir)
		if err != nil {
			fmt.Println(err.Error())
			stallchan <- i
			return
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
				return
			}
		}
	}
}

func DownloadSerial(ctx context.Context, ipfs icore.CoreAPI, cids string, pag bool, np string, concurrentGet int) {
	//logging.SetLogLevel("dht","debug")
	neighbours, err := LocalNeighbour(np)
	if err != nil || len(neighbours) == 0 {
		fmt.Printf("no neighbours file specified, will not disconnect any neighbours after geting\n")
	}

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

	defer func() {
		//finish downloading, remove temp dir
		err = os.RemoveAll(tempDir)
		if err != nil {
			fmt.Println(err.Error())
		}
	}()

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


	// download
	//br := bufio.NewReader(file)
	//for {
	//	torequest, _, err := br.ReadLine()
	//	if err != nil {
	//		// EOF -> finish downloading all files
	//		fmt.Println(err.Error())
	//		return
	//	}
	//
	//	cid := string(torequest)
	//	p := icorepath.New(cid)
	//	start := time.Now()
	//	rootNode, err := ipfs.Unixfs().Get(ctx, p)
	//	if err != nil {
	//		panic(fmt.Errorf("Could not get file with CID: %s", err))
	//	}
	//	err = files.WriteTo(rootNode, tempDir+"/"+cid)
	//	if err != nil {
	//		panic(fmt.Errorf("Could not write out the fetched CID: %s", err))
	//	}
	//
	//	metrics.DownloadTimer.UpdateSince(start)
	//
	//	fmt.Printf("get file %s %f\n", cid, time.Now().Sub(start).Seconds()*1000)
	//	//remove peers
	//	if pag {
	//		err := ipfs.Dht().Provide(ctx, p)
	//		if err != nil {
	//			fmt.Printf("failed to provide file after get: %v\n", err.Error())
	//			return
	//		}
	//
	//	}
	//}


	// NOTE: I change the lines above which is for getting files
	//  so that it will get files concurrently.
	var wg sync.WaitGroup
	wg.Add(concurrentGet)
	for i := 0; i < concurrentGet; i++ {
		go func(theOrder int) {
			defer wg.Done()

			// TODO: If you still want to learn about the first request time,
			//  set firstRequest false
			firstRequest := true
			for j := 0; j < len(fileCid[theOrder]); j++ {
				cid := fileCid[theOrder][j]

				p := icorepath.New(cid)
				start := time.Now()
				rootNode, err := ipfs.Unixfs().Get(ctx, p)
				if err != nil {
					panic(fmt.Errorf("could not get file with CID: %s", err))
				}
				//err = files.WriteTo(rootNode, "./output/"+cid)
				err = files.WriteTo(rootNode, tempDir+"/"+cid)
				if err != nil {
					panic(fmt.Errorf("could not write out the fetched CID: %s", err))
				}

				// NOTE: I still keep the firstRequest option.
				if firstRequest {
					firstRequest = false
				} else {
					metrics.DownloadTimer.UpdateSince(start)
				}

				// NOTE: I added a thread id in the output.
				fmt.Printf("Thread %d get file %s %f\n", theOrder, cid, time.Now().Sub(start).Seconds()*1000)

				if pag {
					err := ipfs.Dht().Provide(ctx, p)
					if err != nil {
						fmt.Printf("failed to provide file after get: %v\n", err.Error())
						return
					}
				}

				if len(neighbours) != 0 {
					for _, n := range neighbours {
						err := DisconnectToPeers(ctx, ipfs, n)
						if err != nil {
							fmt.Printf("failed to disconnect: %v\n", err)
						}
					}
				}
			}

		}(i)
	}
	wg.Wait()
}

// 实现了 daemon upload download 的功能
func main() {

	//read config option
	flag.BoolVar(&(metrics.CMD_CloseBackProvide), "closebackprovide", false, "wether to close background provider")
	flag.BoolVar(&(metrics.CMD_CloseLANDHT), "closelan", false, "whether to close lan dht")
	flag.BoolVar(&(metrics.CMD_CloseDHTRefresh), "closedhtrefresh", false, "whether to close dht refresh")
	flag.BoolVar(&(metrics.CMD_EnableMetrics), "enablemetrics", true, "whether to enable metrics")

	var cmd string
	var filesize int
	var filenumber int
	var parallel int
	var cidfile string
	var provideAfterGet bool
	var neighboursPath string
	var ipfsPath string
	var redun_rate int
	var chunker string // NOTE: added a chunker option
	var concurrentGet int // NOTE: added a option for the number of threads to concurrent get files.


	flag.IntVar(&redun_rate, "redun", 0, "The redundancy of the file when Benchmarking upload, 100 indicates that there is exactly the same file in the node, 0 means there is no existence of same file.(default 0)")
	flag.StringVar(&cmd, "c", "", "operation type\n"+
		"upload: upload files to ipfs, with -s for file size, -n for file number, -p for concurrent upload threads, -cid for specified uploaded file cid stored\n"+
		"downloads: download file following specified cid file with single thread, -pag provide file after get, -np path to the file of neighbours which will be disconnected after each get\n"+
		"daemon: run ipfs daemon\n")
	flag.StringVar(&cidfile, "cid", "cid", "name of cid file for uploading")

	flag.IntVar(&filesize, "s", 256*1024, "file size")
	flag.IntVar(&filenumber, "n", 1, "file number")
	flag.IntVar(&parallel, "p", 1, "concurrent operation number")

	flag.BoolVar(&provideAfterGet, "pag", false, "whether to provide file after get it")

	flag.StringVar(&neighboursPath, "np", "neighbours", "the path of file that records neighbours id, neighbours will be removed after geting file")

	flag.StringVar(&ipfsPath, "ipfs", "./go-ipfs/cmd/ipfs/ipfs", "where go-ipfs exec exists")

	// NOTE: Added two option.
	flag.IntVar(&concurrentGet, "cg", 1, "concurrent get number")
	flag.StringVar(&chunker, "chunker", "size-262144", "customized chunker")

	flag.Parse()

	// NOTE: check the concurrentGet.
	if concurrentGet <= 0 {
		fmt.Printf("Bad parameter for the concurrerntGet. It should be an integer belong to [1, )")
		return
	}

	if metrics.CMD_EnableMetrics {
		metrics.TimersInit()
		defer func() {
			//metrics.Output_addBreakdown()
			metrics.OutputMetrics0()
		}()
	}

	if cmd == "upload" {
		ctx, ipfs, cancel := Ini()

		defer cancel()
		// NOTE: I modified the function for adding a ** chunker ** .
		Upload(filesize, filenumber, parallel, ctx, ipfs, cidfile, redun_rate, chunker)
		return
	}
	if cmd == "downloads" {
		ctx, ipfs, cancel := Ini()
		defer cancel()
		// NOTE: I added a para for setting the threads concurrently getting files.
		DownloadSerial(ctx, ipfs, cidfile, provideAfterGet, neighboursPath, concurrentGet)
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
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)
	bf := bufio.NewReader(file)
	for {
		s, _, err := bf.ReadLine()
		if err != nil {
			fmt.Println(err.Error())
			return neighbours, nil
		}
		neighbours = append(neighbours, string(s))
	}
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
		//fmt.Printf("disconnect from %v\n",addrs)

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
