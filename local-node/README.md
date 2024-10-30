Make sure GO1.15 is installed.

build ipfs node
````
cd go-ipfs
CGO_ENABLED=0 make build
````
build benchmark node
````
cd local-node
CGO_ENABLED=0 go build -o xipfs
````

Initialize IPFS repo
````
./init.sh
````


# xipfs Usage Guide

## Overview

xipfs uses various flags to control the execution of different IPFS-related operations, such as uploading and downloading files, running the IPFS daemon, and more. Below is a breakdown of the supported operations and options.

## Basic Commands

The `-c` flag specifies the command or operation type. Available options are:

1. **upload**: Upload files to IPFS with options for file size (`-s`), number of files (`-n`), concurrent upload threads (`-p`), and the CID file for storing uploaded file CIDs (`-cid`).
   - Example:
     ```bash
     ./xipfs -c upload -s 256k -n 10 -p 5
     ```

2. **downloads**: Download files using a specified CID file. You can provide files after downloading using `-pag` and optionally disconnect neighbors specified in a file using `-rmn`.
   - Example:
     ```bash
     ./xipfs -c downloads -cid cidfile -pag
     ```

3. **daemon**: Run the IPFS daemon.
   - Example:
     ```bash
     ./xipfs -c daemon
     ```

4. **traceUpload**: Upload trace files and return an ItemID-CID mapping.
   - Example:
     ```bash
     ./xipfs -c traceUpload -f tracefile
     ```

5. **traceDownload**: Download files based on a workload trace file and ItemID-CID mapping.
   - Example:
     ```bash
     ./xipfs -c traceDownload -f tracefile
     ```

6. **fullnode**: Run the full node with a specified bitcoin configuration file.
   - Example:
     ```bash
     ./xipfs -c fullnode -bc bitcoin_config
     ```

7. **lightnode**: Run the light node with a specified bitcoin configuration file.
   - Example:
     ```bash
     ./xipfs -c lightnode -bc bitcoin_config
     ```

## Common Command-Line Options

### General Flags
- `-s`: File size (e.g., `256k`, `64m`), default is `262144` (256k).
- `-n`: Number of files, default is `1`.
- `-p`: Number of concurrent operations, default is `1`.
- `-cid`: Name of the CID file for uploading, default is `cid`.
- `-qps`: Queries per second, default is `1`.

### IPFS Configuration
- `-ipfs`: Path to the IPFS executable, default is `./go-ipfs/cmd/ipfs/ipfs`.
- `-redun`: Redundancy level during upload. `100` means the file already exists, `0` means no redundancy. Default is `0`.

### Download/Upload Specific Options
- `-pag`: Whether to provide files after downloading (boolean).
- `-rmn`: Path to a file listing neighbor nodes to disconnect after getting the file.
- `-cg`: Number of concurrent file retrieval threads, default is `1`.
- `-chunker`: Customized chunker option, default is `size-262144`.

### Trace Testing Options
- `-f`: Path to the trace file.
- `-i`: Index indicating the part of the workload handled by the current server, default is `0`.
- `-servers`: Total number of servers, default is `1`.
- `-randomRequest`: Randomize requests in the workload (boolean).

### Performance Optimization Flags
- `-pw`: Number of provider workers to speed up IPFS, default is `8`.
- `-fastsync`: Speed up IPFS by skipping some synchronization (boolean).
- `-earlyabort`: Enable early abort during `findCloserPeers` (boolean).

### Advanced Options
- `-blocksizelimit`: Set the block size limit, default is `1024*1024` (1MB).
- `-enablepbitswap`: Enable `pbitswap` (boolean).
- `-spn`: Search provider number, default is `1`.

### Logging and Debugging
- `-seelogs`: Configure logs for debugging. Use `-` to separate multiple log components, e.g., `dht-bitswap-blockservice`.

