````
cd go-ipfs
make build
````


usage:
````
Usage of ./localIPFSNode:
  -c string
        operation type
        upload: upload files to ipfs, with -s for file size, -n for file number, -p for concurrent upload threads, -cid for specified uploaded file cid stored
        downloads: download file following specified cid file with single thread, -pag provide file after get, -np path to the file of neighbours which will be disconnected after each get
        daemon: run ipfs daemon
    
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
  -fps int
        add file per second
  -n int
        file number (default 1)
  -np string
        the path of file that records neighbours id, neighbours will be removed after geting file (default "neighbours")
  -p int
        concurrent operation number (default 1)
  -pag
        whether to provide file after get it
  -s int
        file size (default 262144)
````
