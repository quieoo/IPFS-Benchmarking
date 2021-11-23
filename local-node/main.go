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
	icorepath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"io"
	"io/ioutil"
	"metrics"
	"os"
	"path/filepath"
	"strings"
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
func UploadFile(file string, ctx context.Context, ipfs icore.CoreAPI) (icorepath.Resolved, error) {
	somefile, err := getUnixfsNode(file)
	if err != nil {
		return nil, err
	}
	//start:=time.Now()

	cid, err := ipfs.Unixfs().Add(ctx, somefile)
	//quieoo.AddTimer.UpdateSince(start)
	return cid, err
}

func Upload(size, number, cores int, ctx context.Context, ipfs icore.CoreAPI, cids string) {
	f, _ := os.Create(cids)

	fmt.Printf("Uploading files with size %d B\n", size)
	coreNumber := cores
	stallchan := make(chan int)
	sendFunc := func(i int) {
		for j := 0; j < number/coreNumber; j++ {
			var subs string
			subs = NewLenChars(size, StdChars)
			inputpath := fmt.Sprintf("./temp %d", i)
			err := ioutil.WriteFile(inputpath, []byte(subs), 0666)
			start := time.Now()
			cid, err := UploadFile(inputpath, ctx, ipfs)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			finish := time.Now()
			fmt.Printf("%s upload %f \n", cid.Cid(), finish.Sub(start).Seconds()*1000)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s", err)
				os.Exit(1)
			}

			io.WriteString(f, strings.Split(cid.String(), "/")[2]+"\n")
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
				return
			}
		}
	}
}

func DownloadSerial(ctx context.Context, ipfs icore.CoreAPI, cids string, pag bool, np string) {
	//logging.SetLogLevel("dht","debug")

	neighbours, err := LocalNeighbour(np)
	if err != nil || len(neighbours) == 0 {
		fmt.Printf("no neighbours file specified, will not disconnect any neighbours after geting\n")
	}

	file, err := os.Open(cids)
	defer file.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}

	_, err = os.Stat("./output")
	if err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir("./output", 0777)
			if err != nil {
				fmt.Printf("failed to mkdir: %v\n", err.Error())
				return
			}
		} else {
			fmt.Println(err.Error())
			return
		}
	}

	br := bufio.NewReader(file)
	for {
		torequest, _, err := br.ReadLine()
		cid := string(torequest)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		p := icorepath.New(cid)
		start := time.Now()
		rootNode, err := ipfs.Unixfs().Get(ctx, p)
		if err != nil {
			panic(fmt.Errorf("Could not get file with CID: %s", err))
		}
		err = files.WriteTo(rootNode, "./output/"+cid)
		if err != nil {
			panic(fmt.Errorf("Could not write out the fetched CID: %s", err))
		}
		fmt.Printf("get file %s %f\n", cid, time.Now().Sub(start).Seconds()/1000)
		//remove peers
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

}

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
	var filepersecond int
	var cidfile string
	var provideAfterGet bool
	var neighboursPath string

	flag.StringVar(&cmd, "c", "", "operation type\n"+
		"upload: upload files to ipfs, with -s for file size, -n for file number, -p for concurrent upload threads, -cid for specified uploaded file cid stored\n"+
		"downloads: download file following specified cid file with single thread, -pag provide file after get, -np path to the file of neighbours which will be disconnected after each get\n"+
		"default: spawning ipfs node\n")
	flag.StringVar(&cidfile, "cid", "cid", "name of cid file for uploading")

	flag.IntVar(&filesize, "s", 256*1024, "file size")
	flag.IntVar(&filenumber, "n", 1, "file number")
	flag.IntVar(&parallel, "p", 1, "concurrent operation number")
	flag.IntVar(&filepersecond, "fps", 0, "add file per second")

	flag.BoolVar(&provideAfterGet, "pag", false, "whether to provide file after get it")

	flag.StringVar(&neighboursPath, "np", "neighbours", "the path of file that records neighbours id, neighbours will be removed after geting file")

	flag.Parse()

	if metrics.CMD_EnableMetrics {
		metrics.TimersInit()
		defer metrics.OutputMetrics()
	}

	if cmd == "upload" {
		ctx, ipfs, cancel := Ini()

		defer cancel()
		Upload(filesize, filenumber, parallel, ctx, ipfs, cidfile)
		return
	}
	if cmd == "downloads" {
		ctx, ipfs, cancel := Ini()
		defer cancel()
		DownloadSerial(ctx, ipfs, cidfile, provideAfterGet, neighboursPath)
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
