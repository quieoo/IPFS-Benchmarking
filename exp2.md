# 实验2：测试provider数量的影响

## http

### on server:

upload file:

```
cd httpfs
./server
```

### on client:

upload file:

```sh
cd httpfs
go run client.go -c upload -n 100 -s 16777216 -fn filenames -h (server ip)
```

download file:

```sh
cd httpfs
go run client.go -c download -fn filenames -h (server ip)
```

## ipfs

### on provider:

```sh
cd local-node
./main -c upload -n 100 -s 16777216 -cid cids
./main -c daemon
```

copy "cids" to client1 and client2 local-node directory

### on client1:

```sh
./main -c downloads -cid cids -pag
```

The client1 became a provider. We got 2 providers, which are provider and client1, now.

### on client2:

```sh
./main -c downloads -cid cids -pag
```
