GO1.15 is recommanded.

build ipfs node
````
cd go-ipfs
make build
````
build benchmark node
````
cd local-node
go build
````

usage:
````
localIPFSNode runs on a default path of ipfs (~/.ipfs), assuming that a repo exists there already. And if necessary, periodly clean the path during benchmarking.


Usage of ./localIPFSNode:
  -c string
    	operation type
    	upload: upload files to ipfs, with -s for file size, -n for file number, -p for concurrent upload threads, -cid for specified uploaded file cid stored
    	downloads: download file following specified cid file with single thread, -pag provide file after get, -np path to the file of neighbours which will be disconnected after each get
    	daemon: run ipfs daemon
    	traceUpload: upload generated trace files, return ItemID-Cid mapping
    	traceDownload: download according to workload trace file and ItemID-CID mapping
    	
  -cg int
    	concurrent get number (default 1)
  -chunker string
    	customized chunker (default "size-262144")
  -cid string
    	name of cid file for uploading (default "cid")
  -closebackprovide
    	wether to close background provider
  -closedhtrefresh
    	whether to close dht refresh
  -closelan
    	whether to close lan dht
  -enablemetrics
    	whether to enable metrics (default true)
  -f string
    	file indicates the path of doc file of generated trace
  -fastsync
    	Speed up IPFS by skip some overcautious synchronization: 1. skip flat-fs synchronizing files, 2. skip leveldb synchronizing files
  -i int
    	given each server has a part of entire workload, index indicates current server own the i-th part of data. Index starts from 0.
  -ipfs string
    	where go-ipfs exec exists (default "./go-ipfs/cmd/ipfs/ipfs")
  -n int
    	file number (default 1)
  -np string
    	the path of file that records neighbours id, neighbours will be removed after geting file (default "neighbours")
  -p int
    	concurrent operation number (default 1)
  -pag
    	whether to provide file after get it
  -providefirst
    	manually provide file after upload
  -randomRequest
    	random request means that current client will randomly reorder requests from generated workload
  -redun int
    	The redundancy of the file when Benchmarking upload, 100 indicates that there is exactly the same file in the node, 0 means there is no existence of same file.(default 0)
  -s string
    	file size, for example: 256k, 64m, 1024 (default "262144")
  -seelogs string
    	configure the specified log level to 'debug', logs connect with'-', such as 'dht-bitswap-blockservice'
  -servers int
    	servers indicates the total number of servers (default 1)
  -stallafterupload
    	stall after upload

````
