package pbitswap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"sync"
	"time"
)

var logger = logging.Logger("pbitswap")

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

// NewDisPatcher create a dispatcher for each node represented file fetching
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

var Empty = 1
var Pending = 2
var Filled = 3

// blkFind expends dispatcher queryState according to given cids, and set their states as Empty.
// Then it sends those cids to all peerToDispatch, inform them of new requests
func (d *Dispatcher) blkFind(cids []cid.Cid) {
	d.left += len(cids)
	for _, c := range cids {
		_, has := d.queryState.Load(c)
		if has {
			//other worker has store this cid
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

type ProviderRole int32

const (
	Role_FullProvider ProviderRole = 0
	Role_CoWorker     ProviderRole = 1
)

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
		workinglock:     new(sync.Mutex),
		da:              NewDynamicAdjuster(),
		working:         false,
	}
	result.absorb2(blks, d.selfID)
	return result
}

var dispatcher_close bool

func (d *Dispatcher) Dispatch3(visit format.Visitor) error {
	dispatcher_close = false
	defer func() {
		//d.monitor.collect()
		// MyTracker.UpdateVariance(d.monitor.GetEffectsVariance())
	}()
	//fmt.Printf("%s dispatch %s\n",time.Now().String(),d.path[0].GetIPLDNode().Cid())
	err := visit(d.path[0])
	if err != nil {
		return err
	}
	rootNode := d.path[0].GetIPLDNode()

	//get the number of blocks
	blkNumber, _ := rootNode.Size()
	blkNumber = blkNumber / 256 / 1024

	//find children of root node
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

	// start peerToDispatch process
	for {
		select {
		case prov, ok := <-providers:
			//fmt.Printf("%s PROVIDER:%s\n",time.Now().String(),prov)
			if !ok {
				fmt.Printf("provider channel close")
				return errors.New("provider channel close")
			}
			if prov != d.selfID {
				// if find a new provider, create a peer worker
				_, ok1 := d.worker.Load(prov)
				if !ok1 {
					newworker := d.newPeerToDispatch(prov, d.cids, d.path[0].GetGetter(), finish, visit)
					newworker.ctx = d.workctx
					//newworker.getter.(format.PeerGetter).PeerConnect(prov)
					newworker.InitRequestBlkNumber(blkNumber)
					//d.worker = append(d.worker, newworker)
					d.worker.Store(prov, newworker)
				}

				// run this worker
				worker, _ := d.worker.Load(prov)
				p := worker.(*peerToDispatch)
				p.workinglock.Lock()
				isworking := p.working
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
