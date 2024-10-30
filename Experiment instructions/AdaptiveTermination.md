The experiments used to test the performance of the proposed `Adaptive Termination Policy for Publication` protocol in XIPFS.

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


## Overall Performance of Uploading file
Repeatedly send file upload requests to a specific Provider node while controlling the request frequency (Requests Per Second, RPS), and simultaneously test the file upload latency. The file size is set to 4KB. From the results of `Add Breakdown`, it can be concluded that the `Provider()` operation dominates the file upload latency. Therefore, the upload latency reflects the performance of the `Provider()` operation.

For both IPFS and XIPFS, each node can independently upload files and add new content to the global DHT table. The increase in the number of nodes does not affect performance. For the Network Indexer, an additional Indexer needs to be maintained, and all Providers send their publication information to the Indexer. In the experiment, 12 Provider nodes are assigned to 1 Indexer, which is a conservative ratio compared to the total number of nodes and Indexers in the public network.

### `IPFS` & `XIPFS`

```bash
# IPFS: 
./xipfs -c uploadqps -s 4k -n 30 -qps 1 -regenerate -enablemetrics -provideeach -closebackprovide

# XIPFS:
./xipfs -c uploadqps -s 4k -n 30 -qps 1 -regenerate -enablemetrics -provideeach -closebackprovide -earlyabort -eac 4
```

Modify `qps` to control the number of file upload requests sent per second, and modify `n` to control the total number of files. For XIPFS, use `-earlyabort -eac 4` to enable the early abort strategy and set the check window size to 4.

### `Network Indexer`

As shown in the [doc](../Centralized_Impelmentation/IPFS_Network_Indexer/instruction.md), configure the Network Indexer. Afterward, run the following commands:

```bash
# Run Indexer:
provider daemon

# Run Provider nodes in the background:
nohup ./kubo daemon >> kubolog 2>&1 &

# Run the file upload script:
node up.mjs <qps> <file_size> <total_number>
```
## XIPFS Analysis
The `eac` value controls the size of the check window in the `Adaptive Termination Policy`. When the search results for nodes closer to the target CID in the check window do not show any improvement over recent rounds, the search is terminated. Therefore, `eac` directly affects the final selection of the Index depositor during Publication, which in turn impacts the latency of Publication and FindProvider during the file upload and download process.

To test the impact of `eac` on file upload performance on the Provider, execute the following command and modify the `eac` value:
```bash
./xipfs -c uploadqps -s 4k -n 30 -qps 1 -regenerate -enablemetrics -provideeach -closebackprovide -earlyabort -eac 4
```
To test the file download performance under different `eac` values on clients, execute the following command:

```bash
# Copy the cid file from the Provider

# Store the Provider ID in 'remove_neighbours' to ensure each download invokes DHT
echo 12D3KooWSR5tXrZEKZjSz5UXZH26b5N91JAFPExqYeY18RGRYUwS >> remove_neighbours

# Download the file and disable background Provide() to avoid interfering with other nodes' DHT downloads
./xipfs -c downloads -cid cid -enablemetrics -closebackprovide
```
