module pbitswap

go 1.15

require (
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-ipfs-util v0.0.2
	github.com/ipfs/go-ipld-format v0.4.0
	github.com/ipfs/go-log v1.0.5
	github.com/libp2p/go-libp2p-core v0.15.1
	github.com/libp2p/go-libp2p-kad-dht v0.15.0 // indirect
	github.com/syndtr/goleveldb v1.0.0
)

replace (
	github.com/ipfs/go-ipld-format => ./../go-ipld-format/
	github.com/libp2p/go-libp2p-core => ./../go-libp2p-core/
	metrics => ./../metrics/
)
