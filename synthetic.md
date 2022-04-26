This is a comprehensive experiment on IPFS and HTTP. 

We try to demonstrate that the advantage of IPFS lies in its scalability, especially on a global network scale, when the network distance between the client and the server are large.


# 1. Generate Synthetic Web Application Traces

Trace Generator is forked from [lookat119/GlobeTraff](https://github.com/lookat119/GlobeTraff), which is an open source from research paper:
>Katsaros K V, Xylomenos G, Polyzos G C. Globetraff: a traffic workload generator for the performance evaluation of future internet architectures[C]//2012 5th International Conference on New Technologies, Mobility and Security (NTMS). IEEE, 2012: 1-5.

Build generator:
```
cd IPFS-Benchmarking/traces/GlobeTraff/
./setup
```
This Generator provides a simple GUI for configuring mixed workload, so run the GUI generator, and generate trace files we need.
```
cd JavaGUI
java -jar dist/JavaGUI.jar
```
Generated trace files will be output to `JavaGUI/data/`, and look like `docs.all` and `workload.all`.

Next, we will make use of the benchmark tools to test the performance of IPFS and HTTP.

We implement a many-to-many architecture, so on each server or client run the instructions we give below.

![architecture](https://github.com/quieoo/IPFS-Benchmarking/blob/main/sync_architecture.png)

# 2. Test HTTP

## 2.1 Run Servers
On Servers: 
```
cd IPFS-Benchmarking/httpfs
./server -c traceUpload -f ../traces/GlobeTraff/JavaGUI/data/docs.all -i 0 -s 3
```
Flag `f` indicates the path of traces files.
Flag `s` indicates the number of total servers, and flag `i` indicates current server index, from 0 to `s-1`. The Entire workload is divided into `s` parts and provided by each server.

## 2.2 Run Clients
On Clients:
```
cd IPFS-Benchmarking/httpfs
./client -c traceDownload -h 101.76.220.43 -f ../traces/GlobeTraff/JavaGUI/data/workload.all -s 3 -randomRequest
```
Flag `h` indicates server ip.
Flag `randomRequest` makes clients randomly reorder their request.
Running the command makes each client to issue requests according to workload file and its own order.

## 2.3 Log
Client will periodically flush its performance into file `PeriodLog`. In one line are time, throughput, request/s, latency, tail latency
