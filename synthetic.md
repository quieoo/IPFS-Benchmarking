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
# 2. Test HTTP

## 2.1 Run Server



