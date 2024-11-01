module localIPFSNode

go 1.15

require (
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-config v0.12.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipld-format v0.4.0
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/go-unixfs v0.2.4
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/libp2p/go-libp2p-core v0.15.1
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/multiformats/go-multihash v0.0.15
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	metrics v0.0.0
)

replace (
	github.com/ipfs/go-bitswap => ./go-bitswap/
	github.com/ipfs/go-blockservice => ./go-blockservice/
	github.com/ipfs/go-ds-flatfs => ./go-ds-flatfs/
	github.com/ipfs/go-ds-leveldb => ./go-ds-leveldb/
	github.com/ipfs/go-ipfs => ./go-ipfs/
	github.com/ipfs/go-ipfs-blockstore => ./go-ipfs-blockstore/
	github.com/ipfs/go-ipfs-chunker => ./go-ipfs-chunker/
	github.com/ipfs/go-ipfs-exchange-interface => ./go-ipfs-exchange-interface/
	github.com/ipfs/go-ipfs-exchange-offline => ./go-ipfs-exchange-offline/
	github.com/ipfs/go-ipfs-provider => ./go-ipfs-provider/
	github.com/ipfs/go-ipld-format => ./go-ipld-format/
	github.com/ipfs/go-merkledag => ./go-merkledag/
	github.com/ipfs/go-unixfs => ./go-unixfs/
	github.com/ipfs/interface-go-ipfs-core => ./interface-go-ipfs-core/
	github.com/libp2p/go-libp2p => ./go-libp2p/
	github.com/libp2p/go-libp2p-core => ./go-libp2p-core/
	github.com/libp2p/go-libp2p-kad-dht => ./go-libp2p-kad-dht/
	github.com/libp2p/go-libp2p-routing-helpers => ./go-libp2p-routing-helpers/
	metrics => ./metrics/
	pbitswap => ./pbitswap/
)
