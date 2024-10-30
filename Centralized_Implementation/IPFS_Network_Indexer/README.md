## Run the Indexer

### Prerequisite
go version 1.21+
```
wget https://go.dev/dl/go1.23.1.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.23.1.linux-amd64.tar.gz
```
Add the following to the `/etc/profile`
```
export PATH=$PATH:/usr/local/go/bin
export PATH=$(go env GOPATH)/bin:$PATH
```
run `source /etc/profile`

### Install & Run
```bash
## build index-provider from source
git clone https://github.com/quieoo/index-provider.git
cd index-provider
go build

## initialize the index-provider
./provider init
```

The configuration file can be found at `~/.index-provider/config`.
Edit the Delegated Routing which allows IPFS nodes to advertise their contents to indexers alongside DHT. For example, as the above storetheindex damon output: 
```
"DelegatedRouting": {
    "ListenMultiaddr": "/ip4/0.0.0.0/tcp/50617",
    "ReadTimeout": "10m0s",
    "WriteTimeout": "10m0s",
    "CidTtl": "24h0m0s",
    "AdFlushFrequency": "10m0s",
    "ChunkSize": 1000,
    "SnapshotSize": 10000,
    "ProviderID": "",
    "DsPageSize": 5000,
    "Addrs": null
  }
```

Run the index-provider
```
./provider daemon
```

## Provider
### Build ipfs

use the modified node (based from IPFS kubo which support Network Indexder) so as to collect the output of content 'Provide' and 'Retrieval'. Choose a directory, run: 
```
git clone https://github.com/quieoo/go-ipfs.git
git clone https://github.com/quieoo/boxo.git
cd boxo
git checkout kubo
cd ../go-ipfs
git checkout kubo
make build CGO_ENABLED=0
```
The executable file is located at `go-ipfs/cmd/ipfs/ipfs`


### Configure and Run
Initialize the ipfs node
```
./cmd/ipfs/ipfs init
```

Configure the ipfs node so as to connect with the index provider, the configuration file can be found at `~/.ipfs/config` and edit the following section:
```
"Routing": {
    "Methods": {
      "find-peers": {
        "RouterName": "WanDHT"
      },
      "find-providers": {
        "RouterName": "ParallelHelper"
      },
      "get-ipns": {
        "RouterName": "WanDHT"
      },
      "provide": {
        "RouterName": "ParallelHelper"
      },
      "put-ipns": {
        "RouterName": "WanDHT"
      }
    },
    "Routers": {
      "IndexProvider": {
        "Parameters": {
          "Endpoint": "http://127.0.0.1:50617",
          "MaxProvideBatchSize": 10000,
          "MaxProvideConcurrency": 1
        },
        "Type": "http"
      },
      "ParallelHelper": {
        "Parameters": {
          "Routers": [
            {
              "IgnoreErrors": true,
              "RouterName": "IndexProvider",
              "Timeout": "30m"
            },
            {
              "IgnoreErrors": true,
              "RouterName": "WanDHT",
              "Timeout": "30m"
            }
          ]
        },
        "Type": "parallel"
      },
      "WanDHT": {
        "Parameters": {
          "AcceleratedDHTClient": false,
          "Mode": "auto",
          "PublicIPNetwork": true
        },
        "Type": "dht"
      }
    },
    "Type": "custom"
  },
```

Or simply run `configure.sh`, for example:
```
configure.sh ~/.ipfs/config ni
or
configure.sh ~/.ipfs/config dht
```
NOTE: modify the 'Endpoint' field to the address of the Indexer. `jq` is need to be installed.

Run the ipfs daemon
```
./cmd/ipfs/ipfs daemon
```

## Run the testing script

Make sure the latest version of Node-JS(at the time when the script was written, it was v20)
```
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash
source ~/.bashrc
nvm install 20
```

Install dependencies
```
cd IPFS-Benchmarking/Centralized_Impelmentation/IPFS_Network_Indexer
npm i kubo-rpc-client
npm i ipfs-http-client
npm i node-fetch
```
Run the testing script to update files
```
(Provider): node ipni_upload.mjs
(Client):   node ipni_findprovider.mjs
```