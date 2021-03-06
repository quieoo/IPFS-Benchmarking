package pbitswap

import (
	"context"
	"fmt"
	"github.com/ipfs/go-cid"
	util "github.com/ipfs/go-ipfs-util"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/syndtr/goleveldb/leveldb/testutil"
	"sort"
	"sync"
	"time"
)

type peerToDispatch struct {
	id peer.ID

	sequence        []cid.Cid
	distances       *sync.Map //distance is mixed with self-cid and provider-cid, used to randomly re-order request sequence
	requestEachTime int

	getter     format.NodeGetter
	ctx        context.Context
	dispatcher *Dispatcher
	visit      format.Visitor

	finish chan peer.ID

	stopflag bool

	effective int

	lock        *sync.Mutex
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

// absord2 reorder blks based on distance definition and update peerToDispatch.sequence
func (p *peerToDispatch) absorb2(blks []cid.Cid, self peer.ID) {
	p.lock.Lock()
	defer p.lock.Unlock()

	//update and keep distance
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

	//re-order request sequence according to distance
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
		// in case when a work calls absorb2 more than one time, so it needs to merge new sequence into original sequence
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

// squeeze return peer.requestEachTime cids for requesting
func (d *Dispatcher) squeeze(peer peerToDispatch) []cid.Cid {
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

// peer worker main loop:
func (p *peerToDispatch) run() {
	logger.Debugf("Worker %s start working", p.id)

	p.workinglock.Lock()
	p.working = true
	p.workinglock.Unlock()

	// build a bitswap connection with target peer
	p.getter.(format.PeerGetter).PeerConnect(p.id)

	defer func() {
		logger.Debugf("Worker %s has done, effectivness: %d", p.id, p.effective)
		//p.dispatcher.worker.Delete(p.id)
		p.workinglock.Lock()
		p.working = false
		p.workinglock.Unlock()
		p.dispatcher.monitor.updateEffects(p.id, p.effective)
		//dispatch_finish = true
		//fmt.Printf("%s visit %d, use %f, channel use %f\n",p.id,p.visitnumber,p.visittime/float64(p.visitnumber),p.channeltime/float64(p.visitnumber))
		//fmt.Printf("%s prepare %d, use %f, request use %f \n",p.id,p.preparenumber,p.preparetime/float64(p.preparenumber), p.requesttime/float64(p.preparenumber))
	}()

	limiter := time.NewTicker(p.da.tolerate) //time-out threshold

	for {
		//fmt.Printf("peer %s, sequence %v\n",p.id,p.sequence)
		p.preparenumber++
		prestart := time.Now()

		toRequest := p.dispatcher.squeeze(*p)
		requests := new(sync.Map)
		numOfRequest := len(toRequest)
		totalrequests := numOfRequest
		//fmt.Printf("peer %s, sequeeze out targets %v\n",p.id,toRequest)
		logger.Debugf("Worker %s, request %d blks, already totally receive %d", p.id, totalrequests, p.dispatcher.collectedblk)

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
		blocks := p.getter.(format.PeerGetter).GetBlocksFrom(p.ctx, toRequest, p.id) //send requests
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
						err = p.visit(navigableNode) //visit block
						p.dispatcher.collectedblk++
						p.dispatcher.writeNodeLock.Unlock()
						if err != nil {
							fmt.Println(err.Error())
						}
						childs := navigableNode.GetChilds() //update its children, if exist
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
				logger.Debugf("Worker %s, Current Batch Request has exceeded the timeout threshold %d", p.id, limiter.C)
				goto NextRound2
			case <-p.ctx.Done():
				return
			}
		}
	NextRound2:
		// adjust requestEachTime dynamically after requesting
		p.requestEachTime = p.da.Adjust5(float64(totalrequests-numOfRequest)/float64(totalrequests), time.Now().Sub(start), totalrequests-numOfRequest)
		//p.requestEachTime = 10
		logger.Debugf("Worker %s, finish current round, left blk: %d", p.id, numOfRequest)
	}
}

func (p *peerToDispatch) Stop() {
	p.stopflag = true
}

// InitRequestBlkNumber determines the initial block request batch size of each peer worker, according to given total block number
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
