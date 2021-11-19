module github.com/ipfs/go-ipfs/examples/go-ipfs-as-a-library

go 1.14

require (
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-config v0.9.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/libp2p/go-libp2p-peerstore v0.2.6
	github.com/multiformats/go-multiaddr v0.3.1
)

replace (
		github.com/ipfs/go-ipfs => ./../../..
		github.com/ipfs/go-bitswap => /home/quieoo/desktop/IPFS/go-bitswap
    	github.com/ipfs/go-blockservice => /home/quieoo/desktop/IPFS/go-blockservice
    	github.com/ipfs/go-datastore => /home/quieoo/desktop/IPFS/go-datastore
    	github.com/ipfs/go-ds-flatfs => /home/quieoo/desktop/IPFS/go-ds-flatfs
    	github.com/ipfs/go-ds-leveldb => /home/quieoo/desktop/IPFS/go-ds-leveldb
    	github.com/ipfs/go-ds-measure => /home/quieoo/desktop/IPFS/go-ds-measure
    	github.com/ipfs/go-ipfs-blockstore => /home/quieoo/desktop/IPFS/go-ipfs-blockstore
    	github.com/ipfs/go-ipfs-cmds => /home/quieoo/desktop/IPFS/go-ipfs-cmds
    	github.com/ipfs/go-ipfs-exchange-interface => /home/quieoo/desktop/IPFS/go-ipfs-exchange-interface
    	github.com/ipfs/go-ipfs-exchange-offline => /home/quieoo/desktop/IPFS/go-ipfs-exchange-offline
    	github.com/ipfs/go-ipfs-pinner => /home/quieoo/desktop/IPFS/go-ipfs-pinner
    	github.com/ipfs/go-ipfs-provider => /home/quieoo/desktop/IPFS/go-ipfs-provider
    	github.com/ipfs/go-ipld-format => /home/quieoo/desktop/IPFS/go-ipld-format
    	github.com/ipfs/go-merkledag => /home/quieoo/desktop/IPFS/go-merkledag
    	github.com/ipfs/go-unixfs => /home/quieoo/desktop/IPFS/go-unixfs
    	github.com/libp2p/go-libp2p => /home/quieoo/desktop/IPFS/go-libp2p
    	github.com/libp2p/go-libp2p-kad-dht => /home/quieoo/desktop/IPFS/go-libp2p-kad-dht
    	github.com/libp2p/go-libp2p-kbucket => /home/quieoo/desktop/IPFS/go-libp2p-kbucket
    	github.com/libp2p/go-libp2p-routing-helpers => /home/quieoo/desktop/IPFS/go-libp2p-routing-helpers
    	github.com/libp2p/go-libp2p-swarm => /home/quieoo/desktop/IPFS/go-libp2p-swarm
		github.com/libp2p/go-libp2p-core => /home/quieoo/desktop/IPFS/go-libp2p-core
)