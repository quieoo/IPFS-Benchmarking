latency and throughput comparison between http, ipfs, ipfs_rn(remove neighbours after each requesting)
>one example of uploading and download 100 files with size of 256KB
# http
## on server:

```
cd httpfs
go run server.go
```

## on client:
upload file:
````
cd httpfs
go run client.go -c upload -n 100 -s 262144 -fn filenames -h (server ip)
````
download file:
```
cd httpfs
go run client.go -c download -fn filenames -h (server ip)
```
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
