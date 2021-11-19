module localIPFSNode

go 1.14

require (
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-config v0.16.0
	github.com/ipfs/go-ipfs-files v0.0.9
	github.com/ipfs/interface-go-ipfs-core v0.5.1
	github.com/libp2p/go-libp2p-core v0.9.0
	github.com/libp2p/go-libp2p-peerstore v0.2.8
	github.com/multiformats/go-multiaddr v0.4.0
	github.com/multiformats/go-multihash v0.0.15
)

replace github.com/ipfs/go-ipfs => ./go-ipfs/
