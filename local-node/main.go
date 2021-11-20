package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/ipfs/go-cid"
	"metrics"

	mh "github.com/multiformats/go-multihash"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	libp2p "github.com/ipfs/go-ipfs/core/node/libp2p"
	icore "github.com/ipfs/interface-go-ipfs-core"
	icorepath "github.com/ipfs/interface-go-ipfs-core/path"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/libp2p/go-libp2p-core/peer"
)

/// ------ Setting up the IPFS Repo

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
	repoPath := "~/.ipfs"
	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}

	// Create the repo with the config
	//err = fsrepo.Init(repoPath, cfg)
	err = fsrepo.Init("~/.ipfs", cfg)
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

//

func connectToPeers(ctx context.Context, ipfs icore.CoreAPI, peers []string) error {
	var wg sync.WaitGroup
	peerInfos := make(map[peer.ID]*peerstore.PeerInfo, len(peers))
	for _, addrStr := range peers {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return err
		}
		pii, err := peerstore.InfoFromP2pAddr(addr)
		if err != nil {
			return err
		}
		pi, ok := peerInfos[pii.ID]
		if !ok {
			pi = &peerstore.PeerInfo{ID: pii.ID}
			peerInfos[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}

	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peerstore.PeerInfo) {
			defer wg.Done()
			err := ipfs.Swarm().Connect(ctx, *peerInfo)
			if err != nil {
				log.Printf("failed to connect to %s: %s", peerInfo.ID, err)
			}
		}(peerInfo)
	}
	wg.Wait()
	return nil
}

func getUnixfsFile(path string) (files.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	st, err := file.Stat()
	if err != nil {
		return nil, err
	}

	f, err := files.NewReaderPathFile(path, file, st)
	if err != nil {
		return nil, err
	}

	return f, nil
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

func addgettest(ctx context.Context, ipfs icore.CoreAPI) {
	fmt.Println("\n-- Adding and getting back files & directories --")

	inputBasePath := "./example-folder/"
	inputPathFile := inputBasePath + "ipfs.paper.draft3.pdf"
	inputPathDirectory := inputBasePath + "test-dir"

	someFile, err := getUnixfsNode(inputPathFile)
	if err != nil {
		panic(fmt.Errorf("Could not get File: %s", err))
	}

	cidFile, err := ipfs.Unixfs().Add(ctx, someFile)
	if err != nil {
		panic(fmt.Errorf("Could not add File: %s", err))
	}

	fmt.Printf("Added file to IPFS with CID %s\n", cidFile.String())

	someDirectory, err := getUnixfsNode(inputPathDirectory)
	if err != nil {
		panic(fmt.Errorf("Could not get File: %s", err))
	}

	cidDirectory, err := ipfs.Unixfs().Add(ctx, someDirectory)
	if err != nil {
		panic(fmt.Errorf("Could not add Directory: %s", err))
	}

	fmt.Printf("Added directory to IPFS with CID %s\n", cidDirectory.String())

	/// --- Part III: Getting the file and directory you added back

	outputBasePath := "./example-folder/"
	outputPathFile := outputBasePath + strings.Split(cidFile.String(), "/")[2]
	outputPathDirectory := outputBasePath + strings.Split(cidDirectory.String(), "/")[2]

	rootNodeFile, err := ipfs.Unixfs().Get(ctx, cidFile)
	if err != nil {
		panic(fmt.Errorf("Could not get file with CID: %s", err))
	}

	err = files.WriteTo(rootNodeFile, outputPathFile)
	if err != nil {
		panic(fmt.Errorf("Could not write out the fetched CID: %s", err))
	}

	fmt.Printf("Got file back from IPFS (IPFS path: %s) and wrote it to %s\n", cidFile.String(), outputPathFile)

	rootNodeDirectory, err := ipfs.Unixfs().Get(ctx, cidDirectory)
	if err != nil {
		panic(fmt.Errorf("Could not get file with CID: %s", err))
	}

	err = files.WriteTo(rootNodeDirectory, outputPathDirectory)
	if err != nil {
		panic(fmt.Errorf("Could not write out the fetched CID: %s", err))
	}

	fmt.Printf("Got directory back from IPFS (IPFS path: %s) and wrote it to %s\n", cidDirectory.String(), outputPathDirectory)

	/// --- Part IV: Getting a file from the IPFS Network

	fmt.Println("\n-- Going to connect to a few nodes in the Network as bootstrappers --")

	bootstrapNodes := []string{
		// IPFS Bootstrapper nodes.
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",

		// IPFS Cluster Pinning nodes
		"/ip4/138.201.67.219/tcp/4001/p2p/QmUd6zHcbkbcs7SMxwLs48qZVX3vpcM8errYS7xEczwRMA",
		"/ip4/138.201.67.219/udp/4001/quic/p2p/QmUd6zHcbkbcs7SMxwLs48qZVX3vpcM8errYS7xEczwRMA",
		"/ip4/138.201.67.220/tcp/4001/p2p/QmNSYxZAiJHeLdkBg38roksAR9So7Y5eojks1yjEcUtZ7i",
		"/ip4/138.201.67.220/udp/4001/quic/p2p/QmNSYxZAiJHeLdkBg38roksAR9So7Y5eojks1yjEcUtZ7i",
		"/ip4/138.201.68.74/tcp/4001/p2p/QmdnXwLrC8p1ueiq2Qya8joNvk3TVVDAut7PrikmZwubtR",
		"/ip4/138.201.68.74/udp/4001/quic/p2p/QmdnXwLrC8p1ueiq2Qya8joNvk3TVVDAut7PrikmZwubtR",
		"/ip4/94.130.135.167/tcp/4001/p2p/QmUEMvxS2e7iDrereVYc5SWPauXPyNwxcy9BXZrC1QTcHE",
		"/ip4/94.130.135.167/udp/4001/quic/p2p/QmUEMvxS2e7iDrereVYc5SWPauXPyNwxcy9BXZrC1QTcHE",

		// You can add more nodes here, for example, another IPFS node you might have running locally, mine was:
		// "/ip4/127.0.0.1/tcp/4010/p2p/QmZp2fhDLxjYue2RiUvLwT9MWdnbDxam32qYFnGmxZDh5L",
		// "/ip4/127.0.0.1/udp/4010/quic/p2p/QmZp2fhDLxjYue2RiUvLwT9MWdnbDxam32qYFnGmxZDh5L",
	}

	go connectToPeers(ctx, ipfs, bootstrapNodes)

	exampleCIDStr := "QmUaoioqU7bxezBQZkUcgcSyokatMY71sxsALxQmRRrHrj"

	fmt.Printf("Fetching a file from the network with CID %s\n", exampleCIDStr)
	outputPath := outputBasePath + exampleCIDStr
	testCID := icorepath.New(exampleCIDStr)

	rootNode, err := ipfs.Unixfs().Get(ctx, testCID)
	if err != nil {
		panic(fmt.Errorf("Could not get file with CID: %s", err))
	}

	err = files.WriteTo(rootNode, outputPath)
	if err != nil {
		panic(fmt.Errorf("Could not write out the fetched CID: %s", err))
	}

	fmt.Printf("Wrote the file to %s\n", outputPath)

	fmt.Println("\nAll done! You just finalized your first tutorial on how to use go-ipfs as a library")

}

func syncMapTest(){
	m:=new(sync.Map)
	m.Store(1,"zhang")
	m.Store(2,"liu")
	m.Store(3,"liang")
	m.Store(4,"li")

	m.Range(func(key, value interface{}) bool {
		if key==2 && value=="liu"{
			m.Store(key,"sun")
			return false
		}
		return true
	})

	m.Range(func(key, value interface{}) bool {
		fmt.Printf("%d,%s\n",key,value)
		return true
	})
}

func testtick(){
	limiter := time.Tick(time.Second)
	fmt.Printf("Init %s\n",time.Now().String())
	for{
		select {
		case <-limiter:
			fmt.Printf("Tick %s\n",time.Now().String())
		}
	}
}

func reqlogphase(){
	f,_:=ioutil.ReadFile("reqlog")
	out, err := os.Create("reqlog-phase")
	if err!=nil{
		fmt.Println(err.Error())
		os.Exit(0)
	}
	s:=string(f)
	ss:=strings.Split(s," ")


	for _,k:=range ss{
		io.WriteString(out,k+"\n")
	}
	fmt.Println("finish")
	out.Close()
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

func GenFile(size int, fn string){
	subs := NewLenChars(size, StdChars)
	//fmt.Println(inputpath)
	err := ioutil.WriteFile(fn, []byte(subs), 0666)
	if err!=nil{
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("Finish generate file with size %d B\n",size)
}

func ConcatFile(f1,f2,f3 string){
	s1,err:=ioutil.ReadFile(f1)
	if err!=nil{
		fmt.Println(err.Error())
		return
	}
	s2,err:=ioutil.ReadFile(f2)
	if err!=nil{
		fmt.Println(err.Error())
		return
	}
	s1=append(s1,s2...)
	err=ioutil.WriteFile(f3,s1,0666)
	if err!=nil{
		fmt.Println(err.Error())
		return
	}
}
func UploadFile(file string, ctx context.Context,ipfs icore.CoreAPI) (icorepath.Resolved,error){
	somefile,err:=getUnixfsNode(file)
	if err!=nil{
		return nil,err
	}
	//start:=time.Now()

	cid,err:=ipfs.Unixfs().Add(ctx,somefile)
	//quieoo.AddTimer.UpdateSince(start)
	return cid,err
}

var (
	HttpClient = &http.Client{
		Timeout: 3 * time.Second,
	}
)
func UploadRemote(size, filePerSecond int){

	tf:=0
	success:=0
	CMD1:="curl -X POST -F file=@"
	CMD2:=" \"http://127.0.0.1:5001/api/v0/add?quiet=false\" "
	//CMD2:=" \"http://121.40.71.68:5001/api/v0/add?quiet=false\" "

	for{
			if filePerSecond==0{
				subs := NewLenChars(size, StdChars)
				inputpath := fmt.Sprintf("./temp/%d", tf)
				tf++
				//fmt.Println(inputpath)
				err := ioutil.WriteFile(inputpath, []byte(subs), 0666)
				if err != nil {
					fmt.Printf("%s\n", err.Error())
					os.Exit(1)
				}

				cmd := CMD1 + inputpath + CMD2
				fmt.Printf("%s\n", cmd)

				c:=exec.Command("/bin/bash","-c",cmd)
				_,err = c.Output()
				if err != nil {
					log.Println(err)
				}
			}else{
				for i := 0; i < filePerSecond; i++ {
					go func() {
						subs := NewLenChars(size, StdChars)
						inputpath := fmt.Sprintf("./temp/%d", tf)
						tf++
						//fmt.Println(inputpath)
						err := ioutil.WriteFile(inputpath, []byte(subs), 0666)
						if err != nil {
							fmt.Printf("%s\n", err.Error())
							os.Exit(1)
						}

						cmd := CMD1 + inputpath + CMD2
						fmt.Printf("%s\n", cmd)

						c:=exec.Command("/bin/bash","-c",cmd)
						_,err = c.Output()
						if err != nil {
							log.Println(err)
						}
						success++
						fmt.Printf("%d:%d\n",success,tf)
						//resp := string(bytes)
						//log.Println(resp)
					}()
				}
				time.Sleep(time.Second)
		}

	}
}


func Upload(size, number, cores int, ctx context.Context, ipfs icore.CoreAPI) {
	f, _ := os.Create("cids")

	fmt.Printf("Uploading files with size %d B\n", size)
	//s := NewLenChars(size, StdChars)

	coreNumber := cores
	stallchan := make(chan int)
	totalAddTime:=0.0
	totalProvideTime:=0.0
	times:=0.0
	var firstcid cid.Cid
	first:=true
	sendFunc := func(i int) {
		for j := 0; j < number/coreNumber; j++ {
			var subs string
			subs = NewLenChars(size, StdChars)
			inputpath := fmt.Sprintf("./temp %d", i)
			//fmt.Println(inputpath)
			err := ioutil.WriteFile(inputpath, []byte(subs), 0666)
			start:=time.Now()
			cid,err:=UploadFile(inputpath,ctx,ipfs)
			if err!=nil{
				fmt.Println(err.Error())
				return
			}
			if first{
				firstcid=cid.Cid()
				first=false
			}
			//fmt.Printf("added file %s\n",cid.Cid())


			provide:=time.Now()
			/*if !quieoo.CMD_CloseAddProvide{
				ipfs.Dht().Provide(ctx,cid)
			}*/
			finish:=time.Now()
			fmt.Printf("%s upload:provide time, (%f,%f)\n",cid.Cid(),provide.Sub(start).Seconds()*1000,finish.Sub(provide).Seconds()*1000)

			totalAddTime+=provide.Sub(start).Seconds()

			totalProvideTime+=finish.Sub(provide).Seconds()
			times++


			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s", err)
				os.Exit(1)
			}

			io.WriteString(f, strings.Split(cid.String(),"/")[2]+"\n")
			//fmt.Println(cid)
			if j%10 == 0 {
				fmt.Printf("upload %d/%d\n", j, number)
			}
		}
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
				f.Close()
				//p:=icorepath.New(firstcid.String())
				//s:=time.Now()
				//ipfs.Dht().Provide(ctx,p)
				//fmt.Printf("provide %s\n",firstcid)
				//fmt.Printf("average addtime: %f ms,average provide time %f ms\n",totalAddTime*1000/times, time.Now().Sub(s).Seconds()*1000)
				//quieoo.MyTracker.Collect2()
				return
			}
		}
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
		if con.ID().String()!=remove{
			continue
		}
		ci := peer.AddrInfo{
			Addrs: []multiaddr.Multiaddr{con.Address()},
			ID: con.ID(),
		}

		addrs,err:=peer.AddrInfoToP2pAddrs(&ci)
		if err!=nil{
			fmt.Println(err.Error())
		}
		//fmt.Printf("disconnect from %v\n",addrs)

		for _,addr:=range addrs{
			err = ipfs.Swarm().Disconnect(ctx, addr)
			if err != nil {
				return err
			}
		}
		break
	}
	return nil
}

func LocalNeighbour()([]string,error){
	var neighbours []string

	filename:="neighbours"
	file,err:=os.Open(filename)
	if err!=nil{
		return nil,fmt.Errorf("failed to open neighbour file: %v\n",err)
	}
	defer file.Close()
	bf:=bufio.NewReader(file)
	for{
		s,_,err:=bf.ReadLine()
		if err!=nil{
			fmt.Println(err.Error())
			return neighbours,nil
		}
		neighbours=append(neighbours,string(s))
	}
}
func DownloadSerial(ctx context.Context, ipfs icore.CoreAPI,cids string) {
	//logging.SetLogLevel("dht","debug")
	neighbours,err:=LocalNeighbour()
	if err!=nil{
		fmt.Printf("falied to get neighbours: %v\n",err)
	}
	fmt.Println("local neighbours")
	for _,n:=range neighbours {
		fmt.Println(n)
	}


	file, err := os.Open(cids)
	defer file.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	br := bufio.NewReader(file)
	var totalTime time.Duration = 0
	var usetimes []time.Duration
	var times = 0
	for {
		torequest, _, err := br.ReadLine()
		if err != nil {
			fmt.Println(err.Error())
			//fmt.Printf("Througput %f MB/s\n",float64(size)*float64(number)/float64(1024*1024)/time.Now().Sub(ostart).Seconds())
			//fmt.Printf("Request Efficiency %f MB/s\n", float64(size)*float64(number)/float64(1024*1024)/totalTime.Seconds())
			//fmt.Printf("request time %f s\n", totalTime.Seconds())
			fmt.Printf("average latency %f ms\n", totalTime.Seconds()*1000/float64(times))
			sort.Slice(usetimes, func(i, j int) bool {
				return usetimes[i] < usetimes[j]
			})
			pl99 := usetimes[times-times/100-1].Seconds()
			fmt.Printf("99 percentile latency %f ms\n", pl99*1000)

			//quieoo.MyTracker.PrintAll()
			//quieoo.MyTracker.Collect()
			//quieoo.MyTracker.CollectRedundant()
			//quieoo.MyTracker.CollectVariance()
			return
		}

		//derr:=DisconnectToPeers(ctx,ipfs,embed78)


		times++
		//fmt.Printf("%s getting file%s\n",start.String(),string(torequest))
		//err = sh.Get(string(torequest), "output")
		p := icorepath.New(string(torequest))
		start := time.Now()
		rootNode, err := ipfs.Unixfs().Get(ctx, p)

		if err != nil {
			panic(fmt.Errorf("Could not get file with CID: %s", err))
		}
		err = files.WriteTo(rootNode, "./output/"+string(torequest))
		if err != nil {
			panic(fmt.Errorf("Could not write out the fetched CID: %s", err))
		}

		metrics.MyTracker.Finish(string(torequest),time.Now())


		usetime := time.Now().Sub(start)
		usetimes = append(usetimes, usetime)
		totalTime += usetime
		provide:=time.Now()
		//ipfs.Dht().Provide(ctx,p)
		fmt.Printf("provide file %s\n",torequest)
		finish:=time.Now()
		fmt.Printf("%s upload:provide time, (%f,%f)\n",torequest,provide.Sub(start).Seconds()*1000,finish.Sub(provide).Seconds()*1000)


		//remove peers
		if neighbours!=nil{
			for _,n:=range neighbours{
				err := DisconnectToPeers(ctx, ipfs, n)
				if err != nil {
					fmt.Printf("failed to disconnect: %v\n",err)
				}
			}
		}
	}

}

func trackerMocking(){
	mhash,_:=mh.FromB58String("QmcWC9p4t7gnx82tmEaWVNotQe3HNLuJvYHK1EPtfVfQRU")
	target:=cid.NewCidV0(mhash)

	metrics.MyTracker.WantBlocks(target,time.Now())
	time.Sleep(500*time.Millisecond)
	metrics.MyTracker.FindProvider(target,time.Now())

	p0,_:=peer.Decode("12D3KooWCqdWNU6CqpdsHodYZBJKjM8M8UmwpPWf7hwJuzvHwJjo")

	resolver,err:=metrics.MyTracker.GetResolverMH(mhash)
	if err!=nil{
		fmt.Println(err.Error())
		return
	}
	resolver.Seed([]peer.ID{p0},time.Now())


	time.Sleep(500*time.Millisecond)
	resolver.Send(p0,time.Now())

	time.Sleep(500*time.Millisecond)
	closers,_:=peer.Decode("12D3KooWH6h7ghs5znUmTaB2VkQGpv5UYEs2bfEByzR4dfgve1XX")
	resolver.GotCloser(p0,[]peer.ID{closers},time.Now())
	resolver.GotProvider(p0,time.Now())
	metrics.MyTracker.FoundProvider(resolver.GetTarget(),time.Now())

	resolver,_=metrics.MyTracker.GetResolverMH(mhash)

	time.Sleep(500*time.Millisecond)
	metrics.MyTracker.Connected(target,time.Now())

	time.Sleep(500*time.Millisecond)
	metrics.MyTracker.Finish("QmcWC9p4t7gnx82tmEaWVNotQe3HNLuJvYHK1EPtfVfQRU",time.Now())

	metrics.MyTracker.PrintAll()
}

type person struct {
	name string
	age int
}
func(p *person)update(n string,a int){
	p.name=n
	p.age=a
}

type personController struct {
	persons []person
}

func (pc *personController)AddTest()  {
	p:=person{name: "d",age: 10}
	pc.persons=append(pc.persons,p)
}

func (pc *personController)Result(){
	for _,p:=range pc.persons{
		fmt.Printf("%s %d\n",p.name,p.age)
	}
}



func SliceTest(){
	var pc personController
	pc.AddTest()
	pc.Result()

	//This Way dont change the item
	for _,p:=range pc.persons{
		if p.name=="d"{
			p.update("e",3)
		}
	}
	pc.Result()
	for i,_:=range pc.persons{
		if pc.persons[i].name=="d"{
			tmp:=pc.persons[i]
			tmp.update("e",3)
			pc.persons[i]=tmp
		}
	}

	pc.Result()

	pc.persons=append(pc.persons,person{name: "f",age: 5})
	pc.Result()

}

const BlockSize=256*1024
func main() {
	//run
	/// --- Part I: Getting a IPFS node running
	//read config option
	flag.BoolVar(&(metrics.CMD_CloseBackProvide),"closebackprovide",false,"wether to close background provider")
	flag.BoolVar(&(metrics.CMD_CloseLANDHT),"closelan",false,"whether to close lan dht")
	flag.BoolVar(&(metrics.CMD_CloseDHTRefresh),"closedhtrefresh",false,"whether to close dht refresh")
	flag.BoolVar(&(metrics.CMD_CloseAddProvide),"closeaddprovide",false,"wthether to close provider when upload file")
	flag.BoolVar(&(metrics.CMD_EnableMetrics),"enablemetrics",true,"whether to enable metrics")


	var cmd string
	var filesize int
	var filenumber int
	var parallel int
	var filename1 string
	var filename2 string
	var filename3 string
	var filepersecond int
	var cidsFile string

	flag.StringVar(&cmd,"c","","operation type")
	flag.StringVar(&filename1,"f1","","name of file 1, output for gen, source for concat, file to add")
	flag.StringVar(&filename2,"f2","","name of file 2, source for concat")
	flag.StringVar(&filename3,"f3","","name of file 1, output for concat")
	flag.StringVar(&cidsFile,"cid","cid","the cids file to download")


	flag.IntVar(&filesize,"s",256*1024,"file size")
	flag.IntVar(&filenumber,"n",1,"file number")
	flag.IntVar(&parallel,"p",1,"concurrent operation number")
	flag.IntVar(&filepersecond,"fps",0,"add file per second")
	flag.Parse()

	if metrics.CMD_EnableMetrics{
		metrics.TimersInit()
		defer metrics.OutputMetrics()
	}

	if cmd=="upload"{
		ctx, ipfs, cancel := Ini()
		defer cancel()
		if filename1!=""{
			UploadFile(filename1,ctx,ipfs)
		}else{
			Upload(filesize,filenumber,parallel,ctx,ipfs)
		}
		return
	}
	if cmd=="downloads"{
		ctx, ipfs, cancel := Ini()
		defer cancel()
		DownloadSerial(ctx,ipfs,cidsFile)
		return
	}
	if cmd=="genfile"{
		GenFile(filesize,filename1)
		return
	}
	if cmd=="concat"{
		ConcatFile(filename1,filename2,filename3)
		return
	}

	if cmd=="uploadremote"{
		//ctx, ipfs, cancel := Ini()
		//defer cancel()
		UploadRemote(filesize,filepersecond)
		return
	}

	_, _, cancel := Ini()
	defer cancel()
	stall:=make(chan int,1)
	select{
	case s:=<-stall:
		fmt.Println(s)
	}
}
