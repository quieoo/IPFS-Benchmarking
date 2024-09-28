## Prerequisite
go version 1.21+

## Run the Provider

Install
```
go install github.com/ipni/index-provider/cmd/provider@latest
```

Initialize
```
provider init
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
provider daemon
```

## Run the ipfs daemon

use the modified node (based from IPFS kubo which support Network Indexder) so as to collect the output of content 'Provide' and 'Retrieval'. Choose a directory, run: 
```
git clone https://github.com/quieoo/go-ipfs.git
cd go-ipfs
git checkout kubo
cd ..
git clone https://github.com/quieoo/boxo.git
cd boxo
git checkout kubo
cd ../go-ipfs
make build CGO_ENABLED=0
```
The executable file is located at `go-ipfs/cmd/ipfs/ipfs`

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

Change the 'Type' filed to `dht` to avoid using index provider.

Run the ipfs daemon
```
./cmd/ipfs/ipfs daemon
```

## Run the testing script

Make sure the latest version of Node-JS(at the time when the script was written, it was v20)
```
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash
nvm install 20
```
Before run `nvm`, it needs to restart the terminal

Install
```
npm i kubo-rpc-client
```

Run the testing script to update files
