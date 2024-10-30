This experiment is used to compare the performance of `Response-Optimized Content Query Strategy` with the original IPFS DHT and a `Network Indexer` based on centralized indexing.

## Environment Setup

The experiments are performed on 2 different kinds of network conditions, each with up to 13 nodes:

- `Public-network`: All nodes are seperated by regions, with average communication latency of 200 ms. Nodes in different region: 
  - Singapore x1, Frankfurt, Germany x1, Virginia, USA x1, Silicon Valley, USA x1, Dubai, UAE x2, Tokyo, Japan x1, Hong Kong, China x1, Seoul, South Korea x1, Jakarta, Indonesia x1, Kuala Lumpur, Malaysia x1, Manila, Philippines x1, Bangkok, Thailand x1, London, UK x1
- `Unreliable-network`: Dispite the `Public-network`, each node is configured with a certain probability of lossing packets.
  - randomly choosing from 0-5%. The script can be found in [file](../tools/packet_loss_setup.sh).

NOTE: In IPFS, nodes must join the global DHT network via public IPs. Although it is possible to form a private DHT network via private IP nodes, the size of the network is extremely limited and is not commonly used, so we only consider the public environment when testing the performance of Publication.

Each instance is equipped with:
- 4vCPU
- 8GB RAM
- 40GB disk
- Ubuntu 22.04

## Overall Performance of FindProviders Operation 

With the cid files containing the uploaded and announced files, send requests to the client node to find providers, while different qps for requests are used. Measure the time taken for each request.

### `IPFS` & `XIPFS`

Assume that the Provider has announced files to the DHT, and a `cid.txt` stores the cid of the announced files. Run the following command to find providers:
```bash
# IPFS:
./xipfs -c findproviderqps -cid cid.txt -enablemetrics -closebackprovide -qps 1
```
The `Response-Optimized Content Query Strategy` in XIPFS works with a didacateted cache for recently queried peers and their communication latency. When running `XIPFS` with command `-PeerRH`, the peers commmunication layer will be saved aotumatically and before shutting down the node, the cache will be saved to disk. Once `XIPFS` daemon is started, all touching peers will be remembered and once the cache hit its capacity, the least recently used peer will be removed.

After running the `XIPFS` for a while, the cache has captured the most recent peers and their communication latency. Run the following command to find providers with help of history information:
```bash
# XIPFS
./xipfs -c findproviderqps -cid cid -enablemetrics -closebackprovide -PeerRH -B 0.95 -qps 1
```
The parameter `-B` determines the impact of the communication latency on the priority of choosing peers for querying. `-B 0` means no impact and `-B 1` means the communication latency dominates.

Change `-qps` in this experiment to test the performance of IPFS and XIPFS under different loads.

### `Network Indexer`
As shown in the [doc](../Centralized_Impelmentation/IPFS_Network_Indexer/instruction.md), configure the Network Indexer, and run the index-provider and deposit routing information into the indexer as in [AT-Announce](AT-Announce.md).
After that, run the script [findproviders](../Centralized_Impelmentation/IPFS_Network_Indexer/ipni_findprovider.mjs) to find providers:
```bash
node ipni_findprovider.mjs <qps> <cid_file>
```

## Sensitivity Analysis

### Effects of `B` value

Choosing of different clients for FindProviders, such as Silicon Valley, Dubai and Jakarta which has different geographical distance from the provider.

Run the search command with different `-B` values:
```
./xipfs -c findproviderqps -cid cid -enablemetrics -closebackprovide -PeerRH -B <b> -qps <qps>
```

### Sensitivity of `eac`

The use of `Adaptive Termination Policy for Publication` affect the index-depositor chosen for storing routing information. It is necessary to test if it is possible to cooperate the tow proposed strategies.

Upload files and announce them to the DHT with differnet `eac` values:
```
./xipfs -c uploadqps -s 4k -n 10 -qps 1 -regenerate -enablemetrics -provideeach -closebackprovide -stallafterupload -earlyabort -eac <*>
```

Run FindProvider with `-B`:
```
./xipfs -c findproviderqps -cid cid -enablemetrics -closebackprovide -PeerRH -B <b> -qps <qps>
```

### Memory Overhead

The cache used to store recent peers and their communication latency is stored in memory. 
Measure that in the global DHT the cache size versus the hit ratio for querying peers.

Keep running the upload and Provide while keep track of the cached entries and hit ratio:
```
./xipfs -c uploadqps -s 4k -n 160 -qps 16 -regenerate -enablemetrics -provideeach -closebackprovide -PeerRH
```
The `-PeerRH` flag is used to enable the PeerRH cache.
