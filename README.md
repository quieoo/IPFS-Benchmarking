In this repo, we illustrate the performance benchmarking of the decentralized protocol IPFS, so as to identify the bottleneck of the performance, and to optimize it.

We propose `XIPFS` which incorporates several techniques that improve the performance without sacrificing the decentralization.

By comparing the performance of `XIPFS`, `IPFS` and `CIPFS` (a centralized performance optimized IPFS), we discuss the benefits of `XIPFS`.


# Setup

## Build IPFS & XIPFS
clone the repo with submodule:
````
git clone --recurse-submodules  https://github.com/quieoo/IPFS-Benchmarking.git
cd local-node
./build_static.sh
./ipfs init
````
Before running the script, make sure you have installed the `go1.15`.

## CIPFS
refer to [IPFS Network Indexer](Centralized_Impelmentation/IPFS_Network_Indexer/instruction.md).


# Usage

## IPFS & XIPFS

`XIPFS`  has a number of parameters to configure or enable some features, which can be fonud at [Local Node](local-node/README.md). 

`XIPFS` includes a wrapper of original `IPFS` to support various testing methods and integrate the technique of `XIPFS` as an extension which can be activated by the user. For example:
- Use case:
```
./xipfs -c upload -n 10 -s 4096 -regenerate -enablemetrics -provideeach -closebackprovide
```
`Generate` and `upload` `10` files to the local node, with each file of `4096` bytes. Enable the `metrics` and ouput. `Manually` call Provide() to update the DHT for each file. The CIDs of uploaded files will be stored in `cid` file.

- Use case:

```
./xipfs -c upload -s 16k -n 10 -regenerate -enablemetrics -seelogs seecpl -provideeach -closebackprovide -earlyabort -eac 10
```
`Genrate` and `upload` `10` files to the local node, with each file of `16kB`. The proposed technique of **Adaptive Termination Policy for Publication** in `XIPFS` is enbaled and the checking window size is `10`. The flag `-seelogs seecpl` is used to output the Common Prefix Length (CPL) of current nearest nodes recorded in the checking window during the Provide process. 

- Use case:
```
./xipfs -c upload -n 10 -s 4096 -provideeach -closebackprovide -stallafterupload -regenerate -p 10
```
 Upload files with 10 concurrent threads(`p` flag), stall after the uploading(like run `ipfs daemon`)



- Use case:
```
echo <provider identity> > remove_neighbours
./xipfs -c downloads -cid cid
```
The `<provider identity>` referred as the PeerID of providers. By doing this the client node will remove the provider from its neighbours list after each download, so that the FindProvider() of DHT will be called on each downloading. The second command conrtols the client node to `download` files with given cids (load from the specified `cid` files).


- Use case:
```
./xipfs -c downloads -cid cid -enablemetrics -enablepbitswap
```

Download files with enabled PBitswap (**Provider-Centric Block Exchange**) in `XIPFS`.


- Use case:
```
./ipfs -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.5  -loadsavecache
```

Download files with enabled **Response-Optimized Content Query Strategy** in `XIPFS` (PeerRH), the parameter `B` is used to balance the physical distance and logical distance (higher `B` values means the physical distance is more important). `-loadsavecache` load the historical communication information as the basis of physical distance from file "cache.txt".

## CIPFS
Refer to [IPFS Network Indexer](Centralized_Impelmentation/IPFS_Network_Indexer/instruction.md).

# Experimental Instructions and Setup

Refer to the docs under `Experiments instructions`