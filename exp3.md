# 实验2. ：测试突发请求的影响

## http

### on server:

```sh
cd httpfs
./server
```

### on client:

我们使用一台机器用来远程控制多台服务器进行并发的get文件。

```sh
# 1. 首先向server上上传文件
cd httpfs
go run client.go -c upload -n 100 -s 16777216 -fn filenames -h (server ip)

# 2. 使用scp命令给全部的clients的httpfs路径下，上传filenames文件
scp filenames benchmark@${clientIP}:~/IPFS-Benchmark/httpfs/
# ... 给全部的client上传

# 3. 修改httpget.sh 中的clientIP 和 serverIP，然后全部client同时get
cd exp3
./httpget.sh
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
./main -c daemon
```

### on client2:

```sh
./main -c downloads -cid cids -pag
./main -c daemon
```

We got 3 providers: the "provider", "client1", and "client2"

### on clients:

我们使用一台机器用来远程控制多台服务器进行并发的get文件。

```sh
# 1. 使用scp命令给全部的clients的local-node路径下，上传cids文件
scp cids benchmark@${clientIP}:~/IPFS-Benchmark/local-node/
# ... 给全部的client上传

# 2. 修改ipfsget.sh 中的clientIP 和 serverIP，然后全部client同时get
cd exp3
./ipfsget.sh
```