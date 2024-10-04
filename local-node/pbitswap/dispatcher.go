package pbitswap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
)

var logger = logging.Logger("pbitswap")

type Dispatcher struct {
	path           []format.NavigableNode
	selfID         peer.ID
	queryState     *sync.Map
	cids           []cid.Cid
	left           int
	wantBlocksEach int

	currentNodeData *bytes.Reader
	ctx             context.Context
	workctx         context.Context
	cancle          context.CancelFunc

	routing  routing.ContentRouting
	rootNode format.NavigableNode

	worker  *sync.Map
	monitor *DispatchMonitor

	writeNodeLock *sync.Mutex
	collectedblk  int
}

// NewDisPatcher creates a dispatcher for file fetching
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

const (
	Empty   = 1
	Pending = 2
	Filled  = 3
)

var closeOnce sync.Once // 确保通道只关闭一次

// blkFind updates the dispatcher with new CIDs and informs workers
func (d *Dispatcher) blkFind(cids []cid.Cid) {
	d.left += len(cids)
	for _, c := range cids {
		_, has := d.queryState.Load(c)
		if has {
			return // Skip if another worker has already stored this CID
		}
		d.queryState.Store(c, Empty)
		d.cids = append(d.cids, c)
	}

	// Parallelize block dispatching using goroutines
	d.worker.Range(func(key, value interface{}) bool {
		go value.(*peerToDispatch).absorb2(cids, d.selfID)
		return true
	})
}

// blkFill marks a CID as filled and decrements remaining blocks
func (d *Dispatcher) blkFill(c cid.Cid) bool {
	d.queryState.Store(c, Filled)
	d.left--
	return d.left > 0
}

// blkRequest generates a list of CIDs to request from a peer
func (d *Dispatcher) blkRequest(peer *peerToDispatch) []cid.Cid {
	var result []cid.Cid
	fetched := 0
	for _, cid := range peer.sequence {
		v, ok := d.queryState.Load(cid)
		if !ok {
			fmt.Println("peerToDispatch requested non-existent CID")
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
	// If more blocks are needed, check for pending blocks
	if fetched < d.wantBlocksEach {
		for _, cid := range peer.sequence {
			v, ok := d.queryState.Load(cid)
			if !ok {
				fmt.Println("peerToDispatch requested non-existent CID")
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

// blkPending marks a list of CIDs as pending
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

var dispatcher_close bool

// Dispatch3 runs the dispatcher loop, continuously fetching and assigning blocks to peers
func (d *Dispatcher) Dispatch3(visit format.Visitor) error {
	dispatcher_close = false

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

	providers := make(chan peer.ID, 100)
	finish := make(chan peer.ID)

	go d.findProviders(rootNode, providers)
	go d.findCoWorkers(providers)

	// Peer dispatch process
	for {
		select {
		case prov, ok := <-providers:
			if !ok {
				return errors.New("provider channel closed")
			}
			// fmt.Printf("dispatcher got provider %s\n", prov)
			if prov != d.selfID {
				// Create a new worker for this provider if not already created
				if _, ok := d.worker.Load(prov); !ok {
					newworker := d.newPeerToDispatch(prov, d.cids, d.path[0].GetGetter(), finish, visit)
					newworker.InitRequestBlkNumber(blkNumber)
					d.worker.Store(prov, newworker)
				}

				worker, _ := d.worker.Load(prov)
				p := worker.(*peerToDispatch)
				if !p.working {
					go p.run()
					go d.routing.(routing.ProviderManagerRouting).ProvideTo(d.workctx, rootNode.Cid(), prov)
				}
			}

		case _, ok := <-finish:
			if !ok {
				return errors.New("channel receive failed")
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
				dispatcher_close = true
				d.cancle()
				return nil
			}

		case <-d.ctx.Done():
			return nil
		}
	}
}

// findProviders periodically searches for new providers
func (d *Dispatcher) findProviders(rootNode format.Node, providers chan peer.ID) {
	defer closeOnce.Do(func() { close(providers) }) // 使用 sync.Once 确保通道只关闭一次

	for {
		if dispatcher_close {
			return
		}

		provChan := d.routing.FindProvidersAsync(d.workctx, rootNode.Cid(), 10)
		for {
			select {
			case prov, ok := <-provChan:
				// fmt.Printf("provider find %s\n", prov.ID)
				if !ok {
					goto nextfinder
				}
				// 移除默认分支，确保通道写入被阻塞直到通道有空间
				providers <- prov.ID

				// select {
				// case providers <- prov.ID:
				// default:
				// }
			}
		}
	nextfinder:
		time.Sleep(1000 * time.Millisecond)
	}
}

// findCoWorkers searches for co-workers from known providers
func (d *Dispatcher) findCoWorkers(providers chan peer.ID) {
	for {
		if dispatcher_close {
			return
		}
		d.worker.Range(func(key, value interface{}) bool {
			provs, err := d.routing.(routing.ProviderManagerRouting).FindProviderFrom(d.workctx, d.path[0].GetIPLDNode().Cid(), key.(peer.ID))
			if err == nil {
				for _, prov := range provs {
					// select {
					// case providers <- prov:
					// default:
					// }
					providers <- prov
				}
			}
			return true
		})
		time.Sleep(1000 * time.Millisecond)
	}
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
		workinglock:     new(sync.Mutex),
		da:              NewDynamicAdjuster(),
		working:         false,
	}
	result.absorb2(blks, d.selfID)
	return result
}
