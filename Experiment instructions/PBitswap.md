The experiments used to test the performance of the proposed `PBitswap` protocol in XIPFS.

## Environment Setup

The experiments are performed on 3 different kinds of network conditions, each with up to 13 nodes:

- `Private-network`: All nodes are located in one cluster, with average communication latency of 0.3 ms (through local network and private IPs). 
- `Public-network`: All nodes are seperated by regions, with average communication latency of 200 ms. Nodes in different region: 
  - Singapore x1, Frankfurt, Germany x1, Virginia, USA x1, Silicon Valley, USA x1, Dubai, UAE x2, Tokyo, Japan x1, Hong Kong, China x1, Seoul, South Korea x1, Jakarta, Indonesia x1, Kuala Lumpur, Malaysia x1, Manila, Philippines x1, Bangkok, Thailand x1, London, UK x1
- `Unreliable-network`: Dispite the `Public-network`, each node is configured with a certain probability of lossing packets.
  - randomly choosing from 0-5%. The script can be found in [file](../tools/packet_loss_setup.sh).

Each instance is equipped with:
- 4vCPU
- 8GB RAM
- 40GB disk
- Ubuntu 22.04

## Overall Perfromance of downloading files

Uploading files on the provider (with number increasing from 2 to 12) and downloading on the client node. Repete the experiment on `Private-network`, `Public-network` and `Unreliable-network` conditions, respectively.

1. Randomly generate the files to upload: 
```
./xipfs -c upload -regenerate -s <file size> -n <file number> -p <concurrent thread number>
```
Generated files will be stored in "temp" directory 

2. Copy the files generated to each provider
3. Run the command on each provider to add files to IPFS network:
```
nohup ./xipfs -c upload -provideeach -closebackprovide -stallafterupload -enablemetrics -s 16m -n <file size> -p <concurrent thread> >> log.txt 2>&1 &
```
The IPFS daemon will be running in the background.
Wait till all files are announced on the network, the CIDs of uploaded files will be stored in "cid.txt".

4. Copy "cid.txt" to the client node and run the following command to start downloading with original Bitswap transfer protocol:
```
./xipfs -c downloads -cid cid -enablemetrics 
```
Or, run the following command to start downloading with PBitswap enabled:
```
./xipfs -c downloads -cid cid -enablemetrics -enablepbitswap
```

If you want to avoid searching for Providers through DHT, you can store the address and information of each Provider in a file "add_neighbours" (as illustrated in [file](../tools/add_neighbours)) and place it in the current directory. 

## Single-Provider Scenario

When there are few providers in the network or only few of them are found, the performance of PBitswap will degenerate to Bitswap. Therefore, the `co-worker` technology is proposed, which allows client nodes to discover other client nodes and use them as temporary available providers (different nodes download files in different orders in PBitswap, so other client nodes may already have the required data blocks).

In this scenario, the file is uploaded to only one provider while the client nodes (ranging from 2 to 12) download it from the provider. The experiment is performed on `Public-network` condition.

1. On the provider node, randomly generate the files and add to the IPFS network:
```
nohup ./xipfs -c upload -provideeach -regenerate -closebackprovide -stallafterupload -enablemetrics -s <file size> -n <file number> -p <concurrent thread number> >> log.txt 2>&1 &
```

2. Copy the "cid.txt" to the client node and run the following command to start downloading:

```
./xipfs -c downloads -cid cid -enablemetrics  -sad
or
./xipfs -c downloads -cid cid -enablemetrics  -enablepbitswap -discoworker -sad
or
./xipfs -c downloads -cid cid -enablemetrics  -enablepbitswap -sad
```

The flag `-sad` controls the client nodes to stall after downloading files so that they can also be discovered by other clients through DHT, but this process is long due to latency of `Announce` procedure in IPFS. 

The first command download file as original IPFS and the second command download file with PBitswap but the co-worker is disabled. The third command download file with PBitswap and the co-worker is enabled.

## Overhead

During experiments, the CPU and Memory overhead of client nodes and providers are measured by running the [script](../tools/overhead_monitor.sh).