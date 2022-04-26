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

## on server:

```
cd httpfs
go run server.go
```

## on client:
upload file:
````
cd httpfs
go run client.go -c upload -n 100 -s 262144 -fn <filenames> -h <server ip>
````

the file 'filename' specified will store all the file's names we just uploaded.

download file:
```
cd httpfs
go run client.go -c download -fn filenames -h (server ip)
```

after each round, run "./ini.sh" to clean memory and the temporary files

# ipfs-none-resolve
## on provider:
````
cd local-node
./main -c upload -n 100 -s 262144 -cid cids
./main -c daemon
````
copy "cids" to client local-node directory

## on client:
````
./main -c downloads -cid cids
````

# ipfs-resolve
## on provider:
````
cd local-node
./main -c upload -n 100 -s 262144 -cid cids
./main -c daemon
````
copy "cids" to client local-node directory

store provider's peer identity in file "neighbours"
## on client:
````
./main -c downloads -cid cids -np neighbours
````
