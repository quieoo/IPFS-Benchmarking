package quieoo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb/testutil"

	"github.com/ipfs/go-cid"
	util "github.com/ipfs/go-ipfs-util"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/peer"
)

type Dispatcher struct {
	path   []format.NavigableNode
	selfID peer.ID
	//queryState map[cid.Cid]int //1-empty, 2-pending, 3-finished

	queryState *sync.Map
	cids       []cid.Cid

	left int

	wantBlocksEach int //number of blks each peer want at one time

	currentNodeData *bytes.Reader
	ctx             context.Context
	workctx         context.Context
	cancle          context.CancelFunc

	routing  routing.ContentRouting
	rootNode format.NavigableNode

	//worker  []*peerToDispatch
	worker  *sync.Map
	monitor *DispatchMonitor

	writeNodeLock *sync.Mutex

	collectedblk int
}

//type Visitor func(node format.NavigableNode) error

func NewDisPatcher(ctx context.Context, root format.NavigableNode) *Dispatcher {
	result := &Dispatcher{
		ctx:            ctx,
		path:           []format.NavigableNode{root},
		wantBlocksEach: 10,
		queryState:     new(sync.Map),
		cids:           []cid.Cid{},
		monitor:        NewMonitor(),
		writeNodeLock:  new(sync.Mutex),
		worker:         new(sync.Map),
	}
	result.routing = result.path[0].GetGetter().(format.PeerGetter).GetRouting()
	result.selfID = result.routing.(routing.ProviderManagerRouting).SelfID()
	result.workctx, result.cancle = context.WithCancel(ctx)
	return result
}
/*
func (d *Dispatcher) saveNodeData(node format.Node) error {
	extractedNodeData, err := unixfs.ReadUnixFSNodeData(node)
	if err != nil {
		return err
	}

	d.currentNodeData = bytes.NewReader(extractedNodeData)
	return nil
}*/

func (d *Dispatcher) writeNodeDataBuffer(w io.Writer) (int64, error) {

	n, err := d.currentNodeData.WriteTo(w)
	if err != nil {
		return n, err
	}

	if d.currentNodeData.Len() == 0 {
		d.currentNodeData = nil
		// Signal that the buffer was consumed (for later `Read` calls).
		// This shouldn't return an EOF error as it's just the end of a
		// single node's data, not the entire DAG.
	}

	return n, nil
}

func (d *Dispatcher) blkFind(cids []cid.Cid) {
	d.left += len(cids)
	for _, c := range cids {
		_, has := d.queryState.Load(c)
		if has {
			//other worker has store this load of cids
			return
		}
		d.queryState.Store(c, Empty)
		d.cids = append(d.cids, c)
	}
	//fmt.Printf("blkFind %d\n",len(cids))
	d.worker.Range(func(key, value interface{}) bool {
		value.(*peerToDispatch).absorb2(cids, d.selfID)
		return true
	})
}

func (d *Dispatcher) blkFill(c cid.Cid) bool {
	d.queryState.Store(c, Filled)
	d.left--
	return d.left > 0
}

func (d *Dispatcher) blkRequest(peer peerToDispatch) []cid.Cid {

	var result []cid.Cid
	fetched := 0
	for _, cid := range peer.sequence {
		v, ok := d.queryState.Load(cid)
		if !ok {
			fmt.Println("peerToDispatch ask for non-exists cid")
			return nil
		}
		if v == Empty {
			result = append(result, cid)
			fetched++
			if fetched >= d.wantBlocksEach {
				break
			}
		}
	}
	if fetched < d.wantBlocksEach {
		for _, cid := range peer.sequence {
			v, ok := d.queryState.Load(cid)
			if !ok {
				fmt.Println("peerToDispatch ask for non-exists cid")
				return nil
			}
			if v == Pending {
				result = append(result, cid)
				fetched++
				if fetched >= d.wantBlocksEach {
					break
				}
			}
		}
	}
	return result
}

func (d *Dispatcher) blkPending(cids []cid.Cid) {

	for _, c := range cids {
		d.queryState.Store(c, Pending)
	}
}

func (d *Dispatcher) Dispatch(visitor format.Visitor) error {
	//got all blk cids for those files small than 43.5MB
	peerSenceGetter := d.path[0].GetGetter()
	cids := d.path[0].GetChilds()

	//d.blkFind(cids)

	//got all providers
	peers := d.path[0].GetPeers()
	fmt.Println("All peers:")
	fmt.Println(peers)

	blocks := peerSenceGetter.(format.PeerGetter).GetBlocksFrom(d.ctx, cids, peers[0])
	blks := len(cids)
	for blks > 0 {
		//fmt.Printf("Calling for blk%s\n", c)
		select {
		case b, ok := <-blocks:
			if !ok {
			}
			nd, err := format.Decode(b)
			if err != nil {
				fmt.Println(err.Error())
			}
			navigableNode := format.NewNavigableIPLDNode(nd, peerSenceGetter)

			childs := navigableNode.GetChilds()
			if len(childs) > 0 {
				d.blkFind(childs)

			}
			//do the visit
			err = visitor(navigableNode)
			if err != nil {
				fmt.Println(err.Error())
			}

			blks--
			/*			err = d.saveNodeData(nd)
						if err != nil {
							return
						}
						written, err := d.writeNodeDataBuffer(w)
						if err != nil {
							return
						}
						fmt.Printf("Written %d\n", written)*/
		case <-d.ctx.Done():
		}
	}
	return nil
}

type ProviderRole int32

const (
	Role_FullProvider ProviderRole = 0
	Role_CoWorker     ProviderRole = 1
)

type peerToDispatch struct {
	id peer.ID

	sequence        []cid.Cid
	distances       *sync.Map
	requestEachTime int

	getter     format.NodeGetter
	ctx        context.Context
	dispatcher *Dispatcher
	visit      format.Visitor

	finish chan peer.ID

	stopflag bool

	effective int

	lock *sync.Mutex
	workinglock *sync.Mutex

	wg *sync.WaitGroup
	da *DynamicAdjuster

	working bool

	visitnumber int
	visittime   float64
	channeltime float64

	preparenumber int
	preparetime   float64
	requesttime   float64
}

func (d *Dispatcher) newPeerToDispatch(p peer.ID, blks []cid.Cid, theGetter format.NodeGetter, finishchan chan peer.ID, visitFunc format.Visitor) *peerToDispatch {
	//fmt.Printf("new Worker for peer %s, got target %v\n",p,blks)

	result := &peerToDispatch{
		id:              p,
		distances:       new(sync.Map),
		sequence:        []cid.Cid{},
		requestEachTime: 10,
		getter:          theGetter,
		ctx:             d.ctx,
		dispatcher:      d,
		finish:          finishchan,
		visit:           visitFunc,
		stopflag:        false,
		effective:       0,
		lock:            new(sync.Mutex),
		workinglock: new(sync.Mutex),
		da:              NewDynamicAdjuster(),
		working:         false,
	}
	result.absorb2(blks, d.selfID)
	return result
}

type sortItem struct {
	cpl   int64
	index int
}
type sortList []sortItem

func (s sortList) Len() int {
	return len(s)
}
func (s sortList) Swap(i, j int) {
	buf := s[i]
	s[i] = s[j]
	s[j] = buf
}
func (s sortList) Less(i, j int) bool {
	return s[i].cpl < s[j].cpl
}

func CommonPrefixLength(p peer.ID, cid3 cid.Cid) int {
	pp := p.String()
	s2 := []byte(pp)
	s3 := cid3.Bytes()

	cpl := 0
	for i := 0; i < len(s2) && i < len(s3); i++ {
		if s2[i] != s3[i] {
			break
		}
		cpl++
	}
	return cpl
}

var xorlength = 4

func ShortXorDistance(p peer.ID, c cid.Cid) int64 {
	pb := []byte(p.String())
	cb := c.Bytes()
	xorlength = testutil.Min(len(pb), len(cb))
	pbs := pb[len(pb)-xorlength:]
	cbs := cb[len(cb)-xorlength:]

	result := util.XOR(pbs, cbs)
	//fmt.Println(result)

	var data int64
	for _, r := range result {
		data = data<<8 + int64(r)
	}
	//fmt.Println(data)
	return data
}

func twoXorDistance(p1 peer.ID, p2 peer.ID, c cid.Cid) int64 {
	d1 := ShortXorDistance(p1, c)
	d2 := ShortXorDistance(p2, c)
	return d1 + d2
}

func DistanceAnalyze() {
	//p1,err:=peer.IDFromString("12D3KooWER6eVhHBVwZuWj7r8ZrqgogNms3QCG4cXRdCQTrjQdr5")
	p1, err := peer.IDB58Decode("12D3KooWER6eVhHBVwZuWj7r8ZrqgogNms3QCG4cXRdCQTrjQdr5")
	if err != nil {
		fmt.Printf(err.Error())
		return
	}
	p2, _ := peer.IDB58Decode("12D3KooWEr3DGcqJPTzVuhX7EuuDgsZUx6qmZMP8v32fCRpy3Fvw")
	c1, _ := peer.IDB58Decode("12D3KooWK28oX1L3fBzrU4dK9uufNfF6th4r2qvtQrgr6oE8X2pt")
	c2, _ := peer.IDB58Decode("12D3KooWSDNngUBaZ9CXYvhEqbQVj8dQXfWNeADnHRRfnShmJjUw")

	cids := []cid.Cid{}
	file, err := os.Open("cids")
	defer file.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}

	br := bufio.NewReader(file)
	for {
		t, _, err := br.ReadLine()
		if err != nil {
			break
		}
		c, e := cid.Decode(string(t))
		if e != nil {
			fmt.Printf(e.Error())
			return
		}
		cids = append(cids, c)
	}

	prov1 := peerToDispatch{id: p1, distances: new(sync.Map), sequence: []cid.Cid{}, lock: new(sync.Mutex)}
	prov2 := peerToDispatch{id: p2, distances: new(sync.Map), sequence: []cid.Cid{}, lock: new(sync.Mutex)}
	//p1forc2:=peerToDispatch{id: p1, distances: new(sync.Map),sequence: []cid.Cid{},lock: new(sync.Mutex)}
	//p2forc2:=peerToDispatch{id: p2, distances: new(sync.Map),sequence: []cid.Cid{},lock: new(sync.Mutex)}

	prov1.absorb(cids)
	prov2.absorb(cids)

	fill := make(map[cid.Cid]int)
	for _, c := range cids {
		fill[c] = Empty
	}

	//fetch from p1 and p2
	blkeachtime := 1
	round := 0
	for i := 0; i < len(cids)-blkeachtime; {
		for j := 0; j < blkeachtime; j++ {
			torequest := prov1.sequence[i+j]
			if fill[torequest] != Filled {
				fill[torequest] = Filled
			}
			torequest = prov2.sequence[i+j]
			if fill[torequest] != Filled {
				fill[torequest] = Filled
			}
		}

		i += blkeachtime

		allfound := true
		for _, v := range cids {
			if fill[v] != Filled {
				allfound = false
			}
		}
		if allfound {
			break
		}
		round++
	}
	fmt.Printf("fetch from p1,p2 , finish in round %d\n", round)

	//c1 fetch from c2,p1,p2
	p1forc1 := peerToDispatch{id: p1, distances: new(sync.Map), sequence: []cid.Cid{}, lock: new(sync.Mutex)}
	p2forc1 := peerToDispatch{id: p2, distances: new(sync.Map), sequence: []cid.Cid{}, lock: new(sync.Mutex)}
	c2forc1 := peerToDispatch{id: c2, distances: new(sync.Map), sequence: []cid.Cid{}, lock: new(sync.Mutex)}

	p1forc2 := peerToDispatch{id: p1, distances: new(sync.Map), sequence: []cid.Cid{}, lock: new(sync.Mutex)}
	p2forc2 := peerToDispatch{id: p2, distances: new(sync.Map), sequence: []cid.Cid{}, lock: new(sync.Mutex)}
	c1forc2 := peerToDispatch{id: c1, distances: new(sync.Map), sequence: []cid.Cid{}, lock: new(sync.Mutex)}
	/*
		p1forc1.absorb2(cids,c1)
		p2forc1.absorb2(cids,c1)
		c2forc1.absorb2(cids,c1)

		p1forc2.absorb2(cids,c2)
		p2forc2.absorb2(cids,c2)
		c1forc2.absorb2(cids,c2)
	*/

	p1forc1.absorb(cids)
	p2forc1.absorb(cids)
	c2forc1.absorb(cids)

	p1forc2.absorb(cids)
	p2forc2.absorb(cids)
	c1forc2.absorb(cids)

	fillc1 := make(map[cid.Cid]int)
	fillc2 := make(map[cid.Cid]int)
	for _, c := range cids {
		fillc1[c] = Empty
		fillc2[c] = Empty
	}
	roundc1 := 0
	roundc2 := 0
	for i := 0; i < len(cids); {
		for j := 0; j < blkeachtime; j++ {
			//c1
			//fetch from p1, p2
			torequest := p1forc1.sequence[i+j]
			if fillc1[torequest] != Filled {
				fillc1[torequest] = Filled
			}
			torequest = p2forc1.sequence[i+j]
			if fillc1[torequest] != Filled {
				fillc1[torequest] = Filled
			}
			//fetch from c2
			torequest = c2forc1.sequence[i+j]
			if fillc2[torequest] == Filled {
				if fillc1[torequest] != Filled {
					fillc1[torequest] = Filled
				}
			}

			//c2
			//fetch from p1, p2
			torequest = p1forc2.sequence[i+j]
			if fillc2[torequest] != Filled {
				fillc2[torequest] = Filled
			}
			torequest = p2forc2.sequence[i+j]
			if fillc2[torequest] != Filled {
				fillc2[torequest] = Filled
			}
			//fetch from c1
			torequest = c1forc2.sequence[i+j]
			if fillc1[torequest] == Filled {
				if fillc2[torequest] != Filled {
					fillc2[torequest] = Filled
				}
			}

		}

		i += blkeachtime

		allfound := true
		for _, v := range cids {
			if fillc1[v] != Filled {
				allfound = false
			}
		}
		if !allfound {
			roundc1++
		}

		allfound = true
		for _, v := range cids {
			if fillc2[v] != Filled {
				allfound = false
			}
		}
		if !allfound {
			roundc2++
		}
	}

	fmt.Printf("c1 fetch from p1 p2 c2, take round %d; and c2 take round %d\n", roundc1, roundc2)
}

func (p *peerToDispatch) Sort(cids []cid.Cid) {
	p.sequence = make([]cid.Cid, len(cids))

	sortsquence := make([]sortItem, len(cids))
	for i := 0; i < len(cids); i++ {
		sortsquence[i].index = i
		sortsquence[i].cpl = ShortXorDistance(p.id, cids[i])
		//fmt.Printf("(%d,%d)\n",i,sortsquence[i].cpl)
	}
	sort.Sort(sortList(sortsquence))

	for i := 0; i < len(cids); i++ {
		p.sequence[i] = cids[sortsquence[i].index]
	}
}

func (p *peerToDispatch) absorb2(blks []cid.Cid, self peer.ID) {
	p.lock.Lock()
	defer p.lock.Unlock()
	unadd := make([]cid.Cid, 0)
	for _, c := range blks {
		//p.distances[c]=ShortXorDistance(p.id,c)
		_, has := p.distances.Load(c)
		if !has {
			p.distances.Store(c, twoXorDistance(self, p.id, c))
			unadd = append(unadd, c)
		} else {
			return
		}
	}

	//fmt.Printf("peer %s\n, have %d ,absorb unadd %d\n",p.id,len(p.sequence),len(unadd))

	sortsquence := make([]sortItem, len(unadd))
	for i := 0; i < len(unadd); i++ {
		sortsquence[i].index = i
		//sortsquence[i].cpl = ShortXorDistance(p.id, unadd[i])
		c, _ := p.distances.Load(unadd[i])
		sortsquence[i].cpl = c.(int64)
		//fmt.Printf("(%d,%d)\n",i,sortsquence[i].cpl)
	}
	sort.Sort(sortList(sortsquence))

	if len(p.sequence) == 0 {
		for i := 0; i < len(unadd); i++ {
			p.sequence = append(p.sequence, unadd[sortsquence[i].index])
		}
	} else {
		mergedSequence := make([]cid.Cid, len(p.sequence)+len(unadd))
		p1 := 0
		p2 := 0
		for i := 0; i < len(p.sequence)+len(sortsquence); i++ {
			if p1 == len(p.sequence) {
				mergedSequence[i] = unadd[sortsquence[p2].index]
				p2++
				continue
			}
			if p2 == len(sortsquence) {
				mergedSequence[i] = p.sequence[p1]
				p1++
				continue
			}
			v, _ := p.distances.Load(p.sequence[p1])
			if v.(int64) < sortsquence[p2].cpl {
				mergedSequence[i] = p.sequence[p1] //error
				p1++
			} else {
				mergedSequence[i] = unadd[sortsquence[p2].index]
				p2++
			}
		}
		p.sequence = mergedSequence
	}
}

func (p *peerToDispatch) absorb(blks []cid.Cid) {

	p.lock.Lock()
	defer p.lock.Unlock()
	unadd := make([]cid.Cid, 0)
	for _, c := range blks {
		//p.distances[c]=ShortXorDistance(p.id,c)
		_, has := p.distances.Load(c)
		if !has {
			p.distances.Store(c, ShortXorDistance(p.id, c))
			unadd = append(unadd, c)
		} else {
			return
		}
	}

	//fmt.Printf("peer %s\n, have %d ,absorb unadd %d\n",p.id,len(p.sequence),len(unadd))

	sortsquence := make([]sortItem, len(unadd))
	for i := 0; i < len(unadd); i++ {
		sortsquence[i].index = i
		//sortsquence[i].cpl = ShortXorDistance(p.id, unadd[i])
		c, _ := p.distances.Load(unadd[i])
		sortsquence[i].cpl = c.(int64)
		//fmt.Printf("(%d,%d)\n",i,sortsquence[i].cpl)
	}
	sort.Sort(sortList(sortsquence))

	if len(p.sequence) == 0 {
		for i := 0; i < len(unadd); i++ {
			p.sequence = append(p.sequence, unadd[sortsquence[i].index])
		}
	} else {
		mergedSequence := make([]cid.Cid, len(p.sequence)+len(unadd))
		p1 := 0
		p2 := 0
		for i := 0; i < len(p.sequence)+len(sortsquence); i++ {
			if p1 == len(p.sequence) {
				mergedSequence[i] = unadd[sortsquence[p2].index]
				p2++
				continue
			}
			if p2 == len(sortsquence) {
				mergedSequence[i] = p.sequence[p1]
				p1++
				continue
			}
			v, _ := p.distances.Load(p.sequence[p1])
			if v.(int64) < sortsquence[p2].cpl {
				mergedSequence[i] = p.sequence[p1] //error
				p1++
			} else {
				mergedSequence[i] = unadd[sortsquence[p2].index]
				p2++
			}
		}
		p.sequence = mergedSequence
	}
}
func Random(strings []cid.Cid) []cid.Cid{

	for i := len(strings) - 1; i > 0; i-- {
		num := rand.Intn(i + 1)
		strings[i], strings[num] = strings[num], strings[i]
	}
	return strings
}
func (d *Dispatcher) sequeeze(peer peerToDispatch) []cid.Cid {
	//fmt.Printf("peer %s, sequeeze from %d targets\n",peer.id,len(peer.sequence))

	//peer.sequence=Random(peer.sequence)
	//peer.requestEachTime=10

	var result []cid.Cid
	fetched := 0
	for _, cid := range peer.sequence {
		v, ok := d.queryState.Load(cid)
		if !ok {
			fmt.Println("peerToDispatch ask for non-exists cid")
			return nil
		}
		if v == Empty {
			result = append(result, cid)
			fetched++
			if fetched >= peer.requestEachTime {
				break
			}
		}
	}
	if fetched < peer.requestEachTime {
		for _, cid := range peer.sequence {
			v, ok := d.queryState.Load(cid)
			if !ok {
				fmt.Println("peerToDispatch ask for non-exists cid")
				return nil
			}
			if v == Pending {
				result = append(result, cid)
				fetched++
				if fetched >= peer.requestEachTime {
					break
				}
			}
		}
	}
	return result
}

func (p *peerToDispatch) run() {
	fmt.Printf("Worker %s working\n", p.id)

	p.workinglock.Lock()
	p.working = true
	p.workinglock.Unlock()

	p.getter.(format.PeerGetter).PeerConnect(p.id)
	defer func() {
		fmt.Printf("%s Worker %s done, effective: %d\n", time.Now().String(), p.id, p.effective)
		//p.dispatcher.worker.Delete(p.id)
		p.workinglock.Lock()
		p.working = false
		p.workinglock.Unlock()
		p.dispatcher.monitor.updateEffects(p.id, p.effective)
		//dispatch_finish = true
		//fmt.Printf("%s visit %d, use %f, channel use %f\n",p.id,p.visitnumber,p.visittime/float64(p.visitnumber),p.channeltime/float64(p.visitnumber))
		//fmt.Printf("%s prepare %d, use %f, request use %f \n",p.id,p.preparenumber,p.preparetime/float64(p.preparenumber), p.requesttime/float64(p.preparenumber))

	}()

	limiter := time.NewTicker(p.da.tolerate)
	for {
		//fmt.Printf("peer %s, sequence %v\n",p.id,p.sequence)
		p.preparenumber++
		prestart := time.Now()

		toRequest := p.dispatcher.sequeeze(*p)
		requests := new(sync.Map)
		numOfRequest := len(toRequest)
		totalrequests := numOfRequest
		//fmt.Printf("peer %s, sequeeze out targets %v\n",p.id,toRequest)
		fmt.Printf("%s, ask for %d : %d\n", p.id, totalrequests,p.dispatcher.collectedblk)

		if numOfRequest <= 0 {
			p.finish <- p.id
			return
		}
		p.dispatcher.blkPending(toRequest)
		for _, r := range toRequest {
			requests.Store(r, Pending)
		}
		p.preparetime += time.Now().Sub(prestart).Seconds() * float64(1000)

		restart := time.Now()
		blocks := p.getter.(format.PeerGetter).GetBlocksFrom(p.ctx, toRequest, p.id)
		p.requesttime += time.Now().Sub(restart).Seconds() * float64(1000)

		start := time.Now()
		readytochannel := time.Now()
		limiter.Reset(p.da.tolerate)
		for {
			select {
			case blk, ok := <-blocks:
				p.channeltime += time.Now().Sub(readytochannel).Seconds() * float64(1000)
				p.visitnumber++
				visitstart := time.Now()

				if !ok {
					//fmt.Printf("block channel finish--%s\n",p.id)
					goto NextRound2
				}
				need := false

				//current node wants
				state2, ok2 := requests.Load(blk.Cid())
				if ok2 && state2 == Pending {
					requests.Store(blk.Cid(), Filled)
					numOfRequest--
					//no other worker take this job
					state1, ok1 := p.dispatcher.queryState.Load(blk.Cid())
					if ok1 && state1 != Filled {

						limiter.Reset(p.da.tolerate)

						need = true
						p.dispatcher.queryState.Store(blk.Cid(), Filled)
						nd, err := format.Decode(blk)
						if err != nil {
							fmt.Println(err.Error())
						}
						navigableNode := format.NewNavigableIPLDNode(nd, p.getter)

						//fmt.Printf("visit blk %s from %s\n",blk.Cid(),p.id)

						p.effective++
						//fmt.Printf("%s %s requesting lock\n",time.Now().String(),p.id)
						p.dispatcher.writeNodeLock.Lock()
						//fmt.Printf("%s %s got lock\n",time.Now().String(),p.id)
						err = p.visit(navigableNode)
						p.dispatcher.collectedblk++
						p.dispatcher.writeNodeLock.Unlock()
						if err != nil {
							fmt.Println(err.Error())
						}
						childs := navigableNode.GetChilds()
						if len(childs) > 0 {
							p.dispatcher.blkFind(childs)
						}
					}
				}

				p.visittime += time.Now().Sub(visitstart).Seconds() * float64(1000)
				readytochannel = time.Now()

				if !need {
					p.dispatcher.monitor.updateRedundant()
					continue
				}

				if numOfRequest <= 0 {

					goto NextRound2
				}

			case <-limiter.C:
				fmt.Printf("TIMELimit")
				goto NextRound2
			case <-p.ctx.Done():
				return
			}
		}
	NextRound2:
		p.requestEachTime = p.da.Adjust5(float64(totalrequests-numOfRequest)/float64(totalrequests), time.Now().Sub(start), totalrequests-numOfRequest)

		fmt.Printf("%s, %d blks left\n", p.id, numOfRequest)
	}
}

func (p *peerToDispatch) Stop() {
	p.stopflag = true
}

//first fetch all peers for root node
//fetch all internal node in merkle tree, so as to get all cids
//dispatch cids to all peers
func (d *Dispatcher) Dispatch2(visit format.Visitor) error {

	err := visit(d.path[0])
	if err != nil {
		return err
	}

	peers := d.path[0].GetPeers() //the peer own root node, owns all the blks

	if len(peers) <= 0 {
		//current file need'd remote peers, means load have
		//TO DO
		fmt.Println("TODO: datastore")
		return nil
	}

	peerSenceGetter := d.path[0].GetGetter()

	//fetch all childs
	currentLevelChilds := d.path[0].GetChilds()

	if len(currentLevelChilds) <= 0 {
		//fmt.Println("end of dag")
		return format.EndOfDag
	}
	var nextLevelChilds []cid.Cid
	arriveBottom := false
	//presume all childs are internal node
	for {
		//fetch each level nodes
		blks := len(currentLevelChilds)
		for blks > 0 {
			//fmt.Printf("Calling for blk%s\n", currentLevelChilds[blks-1])
			blocks := peerSenceGetter.(format.PeerGetter).GetBlocksFrom(d.ctx, []cid.Cid{currentLevelChilds[blks-1]}, peers[0])
			select {
			case b, ok := <-blocks:
				if !ok {

				}
				nd, err := format.Decode(b)
				if err != nil {
					fmt.Println(err.Error())
				}
				navigableNode := format.NewNavigableIPLDNode(nd, peerSenceGetter)

				childs := navigableNode.GetChilds()
				if len(childs) == 0 {
					arriveBottom = true
					goto exit
				} else {
					for _, c := range childs {
						nextLevelChilds = append(nextLevelChilds, c)
					}
				}

				//do the visit
				err = visit(navigableNode)
				if err != nil {
					fmt.Println(err.Error())
				}

				blks--
			case <-d.ctx.Done():
			}
		}
	exit:
		if arriveBottom {
			break
		} else {
			currentLevelChilds = nextLevelChilds
			nextLevelChilds = nextLevelChilds[0:0]
		}
	}
	d.blkFind(currentLevelChilds)

	//fmt.Println(currentLevelChilds)

	//sort all cids for each peer
	ptdList := make([]peerToDispatch, len(peers))
	for i, p := range peers {
		//fmt.Println(p.String())
		//pd:=&peerToDispatch{p,[]int{}}
		pd := new(peerToDispatch)
		pd.id = p
		pd.Sort(currentLevelChilds)
		ptdList[i] = *pd
	}
	/*
		for _, p := range ptdList {
			for _, s := range p.sequence {
				fmt.Printf("%s ", s)
			}
			fmt.Printf("\n")
		}*/

	//dispatch for each peer, fetch all blks
	finish := make(chan int)

	for _, p := range ptdList {
		go func() {
			loop := 0
			for {
				toRequest := d.blkRequest(p)
				numOfRequest := len(toRequest)
				if numOfRequest <= 0 {
					//finish thread
					finish <- 1
					return
				}
				d.blkPending(toRequest)
				blocks := peerSenceGetter.(format.PeerGetter).GetBlocksFrom(d.ctx, toRequest, p.id)
				for {
					select {
					case blk, ok := <-blocks:
						if !ok {
							return
						}
						nd, err := format.Decode(blk)
						if err != nil {
							fmt.Println(err.Error())
						}
						navigableNode := format.NewNavigableIPLDNode(nd, peerSenceGetter)
						requested := false
						for _, i := range toRequest {
							if i == blk.Cid() {
								requested = true
							}
						}
						if requested {
							err = visit(navigableNode)
							if err != nil {
								fmt.Println(err.Error())
							}
							d.blkFill(blk.Cid())
							numOfRequest--
							if numOfRequest <= 0 {
								goto NextRound
							}
						}
					case <-d.ctx.Done():
						return
					}
				}

			NextRound:
				if loop > 10 {
					break
				}
			}
		}()
	}

	select {
	case <-finish:
		return nil
	case <-d.ctx.Done():
		fmt.Println("context finished")
		return nil
	}

}

var Empty = 1
var Pending = 2
var Filled = 3

func (p *peerToDispatch) InitRequestBlkNumber(blkNumber uint64) {
	if blkNumber <= 1 {
		p.requestEachTime = 1
	} else {
		if blkNumber > 20 {
			p.requestEachTime = 10
		} else {
			p.requestEachTime = int(blkNumber) / 2
		}
	}
	//p.requestEachTime=5
	p.da.L = p.requestEachTime

}

var dispatcher_close bool

func (d *Dispatcher) Dispatch3(visit format.Visitor) error {
	dispatcher_close = false
	defer func() {
		//d.monitor.collect()
		MyTracker.UpdateVariance(d.monitor.GetEffectsVariance())
	}()
	//fmt.Printf("%s dispatch %s\n",time.Now().String(),d.path[0].GetIPLDNode().Cid())
	err := visit(d.path[0])
	if err != nil {
		return err
	}
	rootNode := d.path[0].GetIPLDNode()
	blkNumber, _ := rootNode.Size()
	blkNumber = blkNumber / 256 / 1024

	childs := d.path[0].GetChilds()
	if len(childs) == 0 {
		return format.EndOfDag
	} else {
		d.blkFind(childs)
	}
	//one thread periodically find new providers
	//d.routing = d.path[0].GetGetter().(format.PeerGetter).GetRouting()

	providers := make(chan peer.ID)
	finish := make(chan peer.ID)
	//defer close(providers)
	//defer close(finish)
	//find provider from WAN network
	fullprovider := []peer.ID{}
	go func() {
		//defer close(providers)
		//wg.Add(1)
		//To boost provider:
		//	call routing.ProviderManager after got rootNode from provider
		defer func() {
			//fmt.Printf("provider-finder down\n")
		}()
		for {
			if dispatcher_close {
				return
			}

			provChan := d.routing.FindProvidersAsync(d.workctx, rootNode.Cid(), 10)
			for {
				select {
				case prov, ok := <-provChan:
					if !ok {
						goto nextfinder
					}
					//fmt.Printf("provider find %s\n", prov.ID)
					fullprovider = append(fullprovider, prov.ID)
					providers <- prov.ID
				}
			}
		nextfinder:
			time.Sleep(1000 * time.Millisecond)
		}

	}()
	//find co-workers from current known providers
	go func() {
		defer close(providers)

		for {
			if dispatcher_close {
				return
			}
			//only  get co-worker info from full providers
			d.worker.Range(func(key, value interface{}) bool {
				provs, err1 := d.routing.(routing.ProviderManagerRouting).FindProviderFrom(d.workctx, rootNode.Cid(), key.(peer.ID))
				if err1 != nil {
					//fmt.Println(err1.Error())
					return false
				}
				for _, prov := range provs {
					//fmt.Printf("co-worker find2 %s\n", prov)
					providers <- prov
				}
				return true
			})
			time.Sleep(1000 * time.Millisecond)
		}
	}()

	for {
		select {
		case prov, ok := <-providers:
			//fmt.Printf("%s PROVIDER:%s\n",time.Now().String(),prov)
			if !ok {
				fmt.Printf("provider channel close")
				return errors.New("provider channel close")
			}
			if prov != d.selfID {
				_, ok1 := d.worker.Load(prov)
				if !ok1 {

					newworker := d.newPeerToDispatch(prov, d.cids, d.path[0].GetGetter(), finish, visit)
					newworker.ctx = d.workctx
					//newworker.getter.(format.PeerGetter).PeerConnect(prov)
					newworker.InitRequestBlkNumber(blkNumber)
					//d.worker = append(d.worker, newworker)
					d.worker.Store(prov, newworker)
				}
				worker, _ := d.worker.Load(prov)
				p:=worker.(*peerToDispatch)
				p.workinglock.Lock()
				isworking:=p.working
				p.workinglock.Unlock()
				if !isworking {
					go worker.(*peerToDispatch).run()
					go func() {
						//only put co-worker info to full providers
						d.routing.(routing.ProviderManagerRouting).ProvideTo(d.workctx, rootNode.Cid(), prov)
					}()
				}
			}

		case _, ok := <-finish:
			if !ok {
				return errors.New("channel receive")
			}

			allends := true
			d.worker.Range(func(key, value interface{}) bool {
				if value.(*peerToDispatch).working {
					allends = false
					return false
				}
				return true
			})
			if allends {
				/*for _, w := range d.worker {
					w.Stop()
				}*/
				//fmt.Printf("%s Dispatcher cancle\n", time.Now().String())
				dispatcher_close = true
				d.cancle()
				return nil
			}
		case <-d.ctx.Done():
			return nil
		}
	}
}
