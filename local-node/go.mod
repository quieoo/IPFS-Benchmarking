module localIPFSNode

go 1.14

require (
	github.com/ipfs/go-cid v0.1.0 // indirect
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-config v0.12.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/multiformats/go-multiaddr v0.3.1
	metrics v0.0.0
)

replace (
	github.com/ipfs/go-bitswap => ./go-bitswap/
	github.com/ipfs/go-blockservice => ./go-blockservice/
	github.com/ipfs/go-ds-flatfs => ./go-ds-flatfs/
	github.com/ipfs/go-ipfs => ./go-ipfs/
	github.com/ipfs/go-ipfs-provider => ./go-ipfs-provider/
	github.com/ipfs/go-ipld-format => ./go-ipld-format/
	github.com/ipfs/go-merkledag => ./go-merkledag/
	github.com/ipfs/go-unixfs => ./go-unixfs/
	metrics => ./metrics/
	github.com/libp2p/go-libp2p-kad-dht => ./go-libp2p-kad-dht/
)
