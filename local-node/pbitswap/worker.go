package pbitswap

import (
	"context"
	"fmt"
	"sort"
	"sync"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	util "github.com/ipfs/go-ipfs-util"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/syndtr/goleveldb/leveldb/testutil"
)

type peerToDispatch struct {
	id peer.ID

	sequence        []cid.Cid
	distances       *sync.Map //distance is mixed with self-cid and provider-cid, used to randomly re-order request sequence
	requestEachTime int
	MaxRequest      int

	getter     format.NodeGetter
	ctx        context.Context
	dispatcher *Dispatcher
	visit      format.Visitor

	finish chan peer.ID

	stopflag bool

	effective int

	lock        *sync.Mutex // 改为读写锁以提高并发
	workinglock *sync.Mutex

	wg *sync.WaitGroup
	da *DynamicAdjuster

	working bool

	received_blks int
	desired_blks  int
	request_blks  int

	// visitnumber int
	// visittime   float64
	// channeltime float64

	// preparenumber int
	// preparetime   float64
	// requesttime   float64
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
	//fmt.Printf("peer %s, sequeeze from %d targets\n", peer.id, len(peer.sequence))

	var result []cid.Cid
	fetched := 0
	// totalCids := len(peer.sequence)
	// allowedPending := totalCids / 5 // 不超过1/5的CID可以为Pending
	// pendingCount := 0

	// 首先处理所有状态为 Empty 的块
	for _, cid := range peer.sequence {
		v, ok := d.blkQuery(cid)
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

	// 如果还没有达到需要的块数，再处理 Pending 状态的块，但要限制 Pending 的比例
	if fetched < peer.requestEachTime {
		for _, cid := range peer.sequence {
			v, ok := d.blkQuery(cid)
			if !ok {
				fmt.Println("peerToDispatch ask for non-exists cid")
				return nil
			}
			if v == Pending {
				// 检查当前 Pending 状态的数量是否超过1/5的限制
				// if pendingCount < allowedPending {
				result = append(result, cid)
				// pendingCount++
				fetched++
				if fetched >= peer.requestEachTime {
					break
				}
				// }
			}
		}
	}
	return result
}

// run starts the main loop of a peer worker, which is responsible for
// requesting blocks from a peer, processing received blocks, and
// adjusting the request width based on the received block ratio.
// The main loop listens to the block channel and the threshold channel,
// and uses the received blocks to update the request width in real-time.
// The main loop also checks whether all blocks have been received and
// closes the done channel to signal the completion of all block requests.
// The main loop starts by requesting the first batch of blocks, and
// then enters a loop to listen to the block channel and the threshold
// channel. In each iteration, it checks whether there are new blocks
// received, or whether the threshold has been reached. If there are new
// blocks, it processes them and updates the request width based on the
// received block ratio. If the threshold has been reached, it requests
// the next batch of blocks. The main loop continues until all blocks
// have been received, at which point it closes the done channel and
// returns.
func (p *peerToDispatch) run() {
	logger.Debugf("Worker %s start working", p.id)

	p.workinglock.Lock()
	p.working = true
	p.workinglock.Unlock()

	// 建立连接
	p.getter.(format.PeerGetter).PeerConnect(p.id)

	defer func() {
		logger.Debugf("Worker %s has done, effectivness: %d", p.id, p.effective)
		p.workinglock.Lock()
		p.working = false
		p.workinglock.Unlock()
		p.dispatcher.monitor.updateEffects(p.id, p.effective)
	}()

	// 创建主 routine 接收 block 的通道
	blockCh := make(chan blocks.Block, 100) // 缓冲区大小设置为 100
	doneCh := make(chan struct{})
	thresholdCh := make(chan struct{}, 1) // 阈值信号通道只需要 1 个缓冲
	var closeOnceDone sync.Once           // 用于确保通道只关闭一次
	var wg sync.WaitGroup

	// 启动第一个批次块的获取
	toRequest := p.dispatcher.squeeze(*p)
	if len(toRequest) == 0 {
		close(doneCh)
		return
	}
	p.request_blks += len(toRequest)
	p.dispatcher.blkPending(toRequest)

	wg.Add(1)
	go p.getBlocksFrom(toRequest, blockCh, thresholdCh, doneCh, &wg)

	// 主 routine 开始监听 block 接收和阈值触发信号
	for {
		select {
		case blk := <-blockCh:
			// 处理接收到的 block
			status := p.processBlock(blk)
			if status == 1 {
				p.received_blks++
			} else if status == 0 {
				p.received_blks++
				p.desired_blks++
			}
			// 实时维护冗余块比例并根据比例调整请求宽度
			uniquentRatio := float64(p.desired_blks) / float64(p.received_blks)
			if uniquentRatio < 0.7 {
				p.requestEachTime = int(float64(p.MaxRequest) * uniquentRatio) // 例如：快速缩小请求宽度
			}
			if p.requestEachTime < 1 {
				p.requestEachTime = 1
			}

			logger.Debugf("Worker %s received block %s, status: %d , uniqueRatio: %f, requestEachTime: %d", p.id, blk.Cid(), status, uniquentRatio, p.requestEachTime)

			// 判断是否所有块都已接收
			if p.dispatcher.blkAllFilled() {
				// logger.Debugf("Worker %s received all blocks", p.id)
				// 使用 sync.Once 确保通道只被关闭一次
				closeOnceDone.Do(func() {
					close(doneCh) // 关闭主 routine 的完成信号
				})
				p.finish <- p.id
			}

		case <-thresholdCh:
			toRequest = p.dispatcher.squeeze(*p)
			if len(toRequest) > 0 {
				p.request_blks += len(toRequest)
				p.dispatcher.blkPending(toRequest)
				wg.Add(1)
				go p.getBlocksFrom(toRequest, blockCh, thresholdCh, doneCh, &wg)
			}
		case <-doneCh:
			// 完成所有块请求
			logger.Debugf("Worker %s finished all block requests", p.id)
			close(blockCh) // 关闭 block 通道
			wg.Wait()      // 等待所有 goroutine 完成
			return
		}
	}
}

// 额外的 goroutine 调用 GetBlocksFrom，并在接收到 60% 的块时通知主 routine
func (p *peerToDispatch) getBlocksFrom(toRequest []cid.Cid, blockCh chan<- blocks.Block, thresholdCh chan<- struct{}, doneCh <-chan struct{}, wg *sync.WaitGroup) {
	logger.Debugf("Worker %s start new routine to send %d block requests to peers: %v", p.id, len(toRequest), toRequest)
	defer wg.Done()
	blocks := p.getter.(format.PeerGetter).GetBlocksFrom(p.ctx, toRequest, p.id)
	receivedCount := 0
	totalCount := len(toRequest)
	preloaded := 0
	if totalCount <= 1 {
		preloaded = 1
	}
	for {
		select {
		case blk, ok := <-blocks:
			if !ok {
				// 通道关闭，退出
				return
			}

			select {
			case <-doneCh:
				return
			case blockCh <- blk:
				// logger.Debugf("Worker %s received block %s", p.id, blk.Cid())
				receivedCount++
			}
			// 如果接收到超过 60% 的块，通知主 routine
			if preloaded == 0 && float64(receivedCount)/float64(totalCount) >= 0.6 {
				// logger.Debugf("Worker %s received 60%% blocks", p.id)
				select {
				case <-doneCh:
					return
				case thresholdCh <- struct{}{}:
				}
				preloaded = 1
			}

		case <-p.ctx.Done():
			return
		case <-doneCh:
			// 如果 doneCh 关闭，退出
			return
		}
	}
}

// 处理每一个接收到的 block
// 状态：0-成功，1-冗余，2-失败
func (p *peerToDispatch) processBlock(blk blocks.Block) int {
	// 当前节点是否需要这个块
	state, ok := p.dispatcher.blkQuery(blk.Cid())
	if ok && state != Filled {
		// 更新块的状态
		p.dispatcher.blkFill(blk.Cid())
		nd, err := format.Decode(blk)
		if err != nil {
			fmt.Println(err.Error())
			return 2
		}

		navigableNode := format.NewNavigableIPLDNode(nd, p.getter)
		p.effective++

		// 处理块的子节点
		childs := navigableNode.GetChilds()
		if len(childs) > 0 {
			p.dispatcher.blkFind(childs)
		}
		return 0
	} else if state == Filled {
		p.dispatcher.monitor.updateRedundant()
		return 1
	} else {
		return 2
	}
}

func (p *peerToDispatch) Stop() {
	p.stopflag = true
}

// InitRequestBlkNumber determines the initial block request batch size of each peer worker, according to given total block number
func (p *peerToDispatch) InitRequestBlkNumber(blkNumber uint64) {
	if blkNumber <= 1 {
		p.MaxRequest = 1
	} else {
		if blkNumber > 20 {
			p.MaxRequest = 10
		} else {
			p.MaxRequest = int(blkNumber) / 2
		}
	}
	p.requestEachTime = p.MaxRequest
	//p.requestEachTime=5
	p.da.L = p.requestEachTime

}
