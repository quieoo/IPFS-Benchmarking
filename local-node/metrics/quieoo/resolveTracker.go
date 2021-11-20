package metrics

import (
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	kb "github.com/libp2p/go-libp2p-kbucket"
	"github.com/multiformats/go-multihash"
)
const NULL_TIME=0


type Node struct {
	id peer.ID

	sendRequest  time.Time
	gotProvider  time.Time
	gotCloser time.Time
	earlistFound time.Time

	cpl int

	child  []Node
	father peer.ID

}
func NewNode()*Node{
	initime:=time.Unix(NULL_TIME,NULL_TIME)
	return &Node{sendRequest: initime,gotCloser: initime,gotProvider: initime,earlistFound: initime}
}

type ResolveTracker struct {
	target cid.Cid
	mh     multihash.Multihash

	seedPeers []Node
	//nodes map[peer.ID]Node
	//nodes map[peer.ID]Node

	nodes []Node

	providerprovider peer.ID
}

func (rt *ResolveTracker) Init(t cid.Cid) {
	rt.target = t
	rt.mh = t.Hash()
}

func CPL(p peer.ID, mh multihash.Multihash) int {
	//dht.mylogger.Debugf("QueryPeer distance: %d\n", kb.CommonPrefixLen(kb.ConvertKey(string(key)), kb.ConvertPeerID(p)))
	return kb.CommonPrefixLen(kb.ConvertPeerID(p), kb.ConvertKey(string(mh)))
}

func (rt *ResolveTracker) Seed(seeds []peer.ID, t time.Time) {
	for _, i := range seeds {
		node:=NewNode()
		node.id=i
		node.cpl=CPL(i,rt.target.Hash())
		node.earlistFound=t
		node.father=""
		//node := Node{id: i, cpl: CPL(i, rt.target.Hash()), earlistFound: t, father: ""}
		rt.seedPeers = append(rt.seedPeers, *node)
		rt.nodes=append(rt.nodes,*node)
	}
}

func (rt *ResolveTracker) GetTarget() cid.Cid {
	return rt.target
}

func (rt *ResolveTracker) Send(peer peer.ID, t time.Time) {
	for i,_:=range rt.nodes{
		if rt.nodes[i].id==peer{
			k:=rt.nodes[i]
			k.sendRequest=t
			rt.nodes[i]=k
			return
		}
	}
}

func (rt *ResolveTracker) GotCloser(peer peer.ID, closers []peer.ID, t time.Time) {

	for i,_:=range rt.nodes{
		if rt.nodes[i].id==peer{
			rt.nodes[i].gotCloser=t
			for _,c:=range closers{

				//if the corresponding closers already exists, we don't do anything
				//mark earliest found time
				exists:=false
				for _,v:=range rt.nodes{
					if v.id==c{
						exists=true
					}
				}
				if exists{
					continue
				}

				//nd := Node{id: c, earlistFound: t, father: peer, cpl: CPL(c, rt.target.Hash())}
				nd:=NewNode()
				nd.id=c
				nd.earlistFound=t
				nd.father=peer
				nd.cpl=CPL(c,rt.target.Hash())

				rt.nodes=append(rt.nodes,*nd)

				k:=rt.nodes[i]
				k.child=append(k.child,*nd)
				rt.nodes[i]=k
			}
			return
		}
	}
}

func (rt *ResolveTracker) GotProvider(p peer.ID, t time.Time) {
	for i,_:=range rt.nodes{
		if rt.nodes[i].id==p{
			k:=rt.nodes[i]
			k.gotProvider=t
			rt.nodes[i]=k

			rt.providerprovider=p
			//fmt.Printf("Provider-Provider cpl:%d\n",rt.nodes[i].cpl)
			return
		}
	}

}
func(rt *ResolveTracker) ProviderCPL() int{
	for _,v:=range rt.nodes{
		if v.id==rt.providerprovider{
			return v.cpl
		}
	}
	return -1
}

//collect the info of all trying to connect nodes
func (rt *ResolveTracker) Collect2(){
	AllFound:=len(rt.nodes)
	fmt.Printf("%d\n",AllFound)

	Requests:=0
	initime:=time.Unix(NULL_TIME,NULL_TIME)
	for _,r:=range rt.nodes{
		if r.sendRequest!=initime{
			Requests++
		}
	}
	fmt.Printf("send requests to %d nodes\n",Requests)
}

func (rt *ResolveTracker)requestLatency()[]float64{
	initime:=time.Unix(NULL_TIME,NULL_TIME)
	result:=[]float64{}
	for _,r:=range rt.nodes{
		if r.sendRequest!=initime && r.gotCloser!=initime{
			result=append(result,r.gotCloser.Sub(r.sendRequest).Seconds())
		}
	}
	return result
}

func (rt *ResolveTracker)Hops(p peer.ID)int{
	for i,_:=range rt.nodes{
		if rt.nodes[i].id==p{
			hop:=0
			cur:=i
			for rt.nodes[cur].father!=""{
				hop++
				for j,_:=range rt.nodes{
					if rt.nodes[j].id==rt.nodes[cur].father{
						cur=j
						break
					}
				}
			}
			return hop
		}
	}
	return -1
}

//collect the overall Communication Overhead and Schedule Overhead for finding provider
func (rt *ResolveTracker) Collect() (time.Duration, time.Duration) {

	for i,_:=range rt.nodes{
		if rt.nodes[i].id == rt.providerprovider{
			var CO time.Duration
			var SO time.Duration

			CO += rt.nodes[i].gotProvider.Sub(rt.nodes[i].sendRequest)
			SO += rt.nodes[i].sendRequest.Sub(rt.nodes[i].earlistFound)

			index:=i

			for rt.nodes[index].father!=""{

				for fa,_:=range rt.nodes{
					if rt.nodes[fa].id==rt.nodes[index].father{
						CO+=rt.nodes[index].earlistFound.Sub(rt.nodes[fa].sendRequest)
						if rt.nodes[fa].father!=""{
							SO+=rt.nodes[fa].sendRequest.Sub(rt.nodes[fa].earlistFound)
						}

						index=fa
						break
					}
				}
			}
			return CO,SO
		}
	}

	fmt.Println("ERROR: no providerprovider")
	return 0,0
}

func (rt *ResolveTracker) TestCollection() {
	fmt.Println("=======FOUND-PROVIDER========================")
	for i,_:=range rt.nodes{
		fmt.Printf("comparing (%s,%s)\n",rt.nodes[i].id,rt.providerprovider)
		if rt.nodes[i].id==rt.providerprovider{
			fmt.Printf("%s got provider\n", rt.nodes[i].gotProvider.String())

			index:=i
			for rt.nodes[index].father!=""{
				fmt.Printf("from peer %s\n", rt.nodes[index].id)
				fmt.Printf("  %s query peer\n  %s find peer\n", rt.nodes[index].sendRequest.String(), rt.nodes[index].earlistFound.String())
				for fa,_:=range rt.nodes{
					if rt.nodes[index].father==rt.nodes[fa].id{
						index=fa
						break
					}
				}
			}
			return
		}
	}
	fmt.Printf("ERROR no providerprovider\n")
}

func (rt *ResolveTracker)State(){
	fmt.Println("State:")
	for i,_:=range rt.nodes{
		fmt.Printf("%s,%s,%s,%s\n",rt.nodes[i].id,rt.nodes[i].earlistFound,rt.nodes[i].sendRequest,rt.nodes[i].gotProvider)
	}
}

