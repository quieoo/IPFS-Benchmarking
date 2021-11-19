package quieoo

import (
	"bufio"
	"context"
	"fmt"
	icorepath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	iface "github.com/ipfs/interface-go-ipfs-core"
)

//used to track the change of one file's provider (the top 20 closest peer to the cid)
type ProviderTracker struct {
	interval time.Duration

}

var ProvT *ProviderTracker

func NewProviderTracker(intv time.Duration)*ProviderTracker{
	return &ProviderTracker{
		interval:intv,
	}
}

func (pt *ProviderTracker)TakeIn(s *Spin){

	var file *os.File
	var err error
	file,err=os.OpenFile("Muti_ProviderLog",os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err!=nil{
		fmt.Println(err.Error())
		return
	}
	defer file.Close()
	fmt.Printf("%d %f %f %f\n",s.Avaluable(),s.First(),s.Average(), s.AverageHop())
	_,err=io.WriteString(file,fmt.Sprintf("%d %f %f %f\n",s.Avaluable(),s.First(),s.Average(), s.AverageHop()))
	if err!=nil{
		fmt.Println(err.Error())
	}
}

func(pt *ProviderTracker)Monitor(ipfs iface.CoreAPI, ctx context.Context){
	file, err := os.Open("cids")
	newfile,err1:=os.Create("ProviderLog")

	if err1!=nil{
		fmt.Println(err1.Error())
	}
	newfile.Close()

	defer func() {
		file.Close()
	}()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	br := bufio.NewReader(file)
	for {
		torequest, _, err := br.ReadLine()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		p := icorepath.New(string(torequest))

		for i:=0;;i++{
			fmt.Printf("routine %d\n",i)
			out,err:=ipfs.Dht().FindProviders(ctx,p)

			if err!=nil{
				fmt.Println(err.Error())
				return
			}
			for{
				select {
				case _,ok:=<-out:
					if !ok{
						fmt.Println("channel !ok")
						goto nextroutine
					}
					//fmt.Printf("found provider %s\n",prov.ID)
				}
			}
		nextroutine:
			DisConnectAllPeers(ctx,ipfs)
			time.Sleep(pt.interval)
		}
	}
}

func(pt *ProviderTracker) Muti_Monitor(ctx context.Context, ipfs iface.CoreAPI){
	newfile,err1:=os.Create("Muti_ProviderLog")

	if err1!=nil{
		fmt.Println(err1.Error())
		return
	}
	newfile.Close()

	for {
		file, err := os.Open("muti_cids")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s", err)
			os.Exit(1)
		}
		br := bufio.NewReader(file)

		mplog,err:=os.OpenFile("Muti_ProviderLog",os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err!=nil{
			fmt.Println(err.Error())
		}
		io.WriteString(mplog,fmt.Sprintf("%s\n",time.Now().String()))

		for {
			torequest, _, err := br.ReadLine()
			if err != nil {
				//fmt.Println(err.Error())
				break
			}

			p := icorepath.New(string(torequest))
			fmt.Printf("FindProv for %s\n", p.String())

			out, err := ipfs.Dht().FindProviders(ctx, p)

			if err != nil {
				fmt.Println(err.Error())
				return
			}
			for {
				select {
				case _, ok := <-out:
					if !ok {
						fmt.Println("channel !ok")
						goto nextroutine2
					}
					//fmt.Printf("found provider %s\n",prov.ID)
				}
			}
		nextroutine2:
			if CMD_CloseDHTRefresh {
				DisConnectAllPeers(ctx, ipfs)
			}
			//time.Sleep(pt.interval)
		}
		file.Close()
	}
}

func DisConnectAllPeers(ctx context.Context,ipfs iface.CoreAPI){
	peerInfos, err := ipfs.Swarm().Peers(ctx)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	for _, con := range peerInfos {
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
				fmt.Println(err.Error())
				return
			}
		}
	}
}


type Spin struct{
	//m map[peer.ID]float64
	latency []float64
	hop []int
	StartTime time.Time
}
func NewSpin()*Spin{
	return &Spin{
		latency: []float64{},
		StartTime: time.Now(),
		hop:[]int{},
	}
}

func(sp *Spin)Update(h int){
	sp.latency=append(sp.latency,time.Now().Sub(sp.StartTime).Seconds())
	sp.hop=append(sp.hop,h)
}
func(sp *Spin)Avaluable()int{
	return len(sp.latency)
}
func(sp *Spin)First()float64{
	min:=10000000.0
	for _,v:=range sp.latency{
		if v<=min{
			min=v
		}
	}
	return min
}
func(sp *Spin)Average()float64{
	total:=0.0
	number:=0.0

	for _,v:=range sp.latency{
		number++
		total+=v
	}
	return total/number
}
func (sp *Spin)AverageHop()float64{
	total:=0.0
	number:=0.0
	for _,v:=range sp.hop{
		if v!=-1{
			number++
			total+=float64(v)
		}
	}
	return total/number
}


var CMD_CloseBackProvide=true
var CMD_CloseLANDHT=false
var CMD_CloseDHTRefresh=false
var CMD_CloseAddProvide=false
var CMD_ImmeProvide=true

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

var StdChars = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

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

func ProviderSearch(ipfs iface.CoreAPI, ctx context.Context) {
	subs := NewLenChars(1024, StdChars)
	inputpath := fmt.Sprintf("./temp")
	//fmt.Println(inputpath)
	err := ioutil.WriteFile(inputpath, []byte(subs), 0666)

	someFile, err := getUnixfsNode(inputpath)
	if err != nil {
		panic(fmt.Errorf("Could not get File: %s", err))
	}

	//cid, err := sh.Add(strings.NewReader(subs))
	cid, _ := ipfs.Unixfs().Add(ctx, someFile)
	fmt.Printf("Provide:%s \n",cid)
	start:=time.Now()
	ipfs.Dht().Provide(ctx,cid)
	fmt.Printf("take %f s\n",time.Now().Sub(start).Seconds())

	for i:=0;i<10;i++{
		fmt.Println("routine"+string(i))
		out,err:=ipfs.Dht().FindProviders(ctx,cid)
		if err!=nil{
			fmt.Println(err.Error())
			return
		}
		for{
			select {
			case prov,ok:=<-out:
				if !ok{
					fmt.Println("channel !ok")
					goto nextroutine
				}
				fmt.Printf("%s\n",prov.ID)
			}
		}


	nextroutine:
		time.Sleep(time.Minute)
	}
}
func ProvidersSearch(ipfs iface.CoreAPI,ctx context.Context) {
	//logging.SetLogLevel("dht","debug")
	ProvT=NewProviderTracker(time.Minute)
	//ProvT.Monitor(ipfs,ctx)
	ProvT.Muti_Monitor(ctx,ipfs)
}