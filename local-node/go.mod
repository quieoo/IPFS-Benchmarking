module localIPFSNode

go 1.14

require (
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-config v0.12.0
	github.com/ipfs/go-ipfs-files v0.0.9
	github.com/ipfs/interface-go-ipfs-core v0.5.2
	github.com/libp2p/go-libp2p-core v0.11.0
	github.com/libp2p/go-libp2p-peerstore v0.2.7
	github.com/multiformats/go-multiaddr v0.4.1
	github.com/multiformats/go-multihash v0.1.0
	metrics v0.0.0-00010101000000-000000000000
)

replace (
	github.com/ipfs/go-ipfs => ./go-ipfs/
	metrics => ./metrics
)
