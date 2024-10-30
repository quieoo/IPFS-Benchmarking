The experiment uses the Bitcoin dataset to emulate a scenario where IPFS is leveraged to offload block data, reducing the storage costs for blockchain nodes while preserving block consistency and decentralization.

## Environment Setup

The experiment is performaned on the `Public-network`, in which 13 nodes are deployed in public network and from different locations:
- Singapore x1, Frankfurt, Germany x1, Virginia, USA x1, Silicon Valley, USA x1, Dubai, UAE x2, Tokyo, Japan x1, Hong Kong, China x1, Seoul, South Korea x1, Jakarta, Indonesia x1, Kuala Lumpur, Malaysia x1, Manila, Philippines x1, Bangkok, Thailand x1, London, UK x1

Among them the Singapore node serves as the full node and the rest are light nodes.

Each instance is equipped with:
- 4vCPU
- 8GB RAM
- 40GB disk
- Ubuntu 22.04

## Overall Performance comparison

Prequisite: 
- A trace that contains blocks from bitcoin main chain: [trace](../tools/bitcoin_blk_one_week)
- The XIPFS nodes have been running for a while and the history peer latency hit ratio reachs ~98% (as detailed in [last experiment](RO-DHT.md))
- Light nodes are configure to remove the Full node from the neighbor list by running `echo <Full node ID> >> remove_neighboours` on the same directory of the `xipfs` binary
- A configure file that contains the IP address of peers and the path to trace file, such as [full_node_config](../tools/bitcoin_config%20fullnode.json) and [light_node_config](../tools/bitcoin_config%20lightnode.json)

Working procedure:
1. Full node generate the block according to the trace
2. Full node update the block to the IPFS and publish the block CID to the DHT
3. Full node broadcast the block CID to the light nodes
4. Light nodes fetch the block CID from the DHT and fetch the block from the IPFS
5. Light nodes validate the block and respond the comfirmation to the full node
6. Full node update the state and continue for the next block


### IPFS
Running command:

```bash
# Full Node:
./xipfs -c fullnode -bc <confgire file path> -enablemetrics -provideeach -closebackprovide
# Light Node:
./xipfs -c lightnode -bc <confgire file path>  -enablemetrics
```

### XIPFS
Running command:
```bash
# Full Node
./xipfs -c fullnode -bc <confgire file path> -enablemetrics -provideeach -closebackprovide -earlyabort -eac 4

# Light Node
./xipfs -c lightnode -bc <confgire file path>  -enablemetrics -PeerRH -B 0.95  -enablepbitswap  -pbticker
```

