latency and throughput comparison between http, ipfs, ipfs_rn(remove neighbours after each requesting)
# http
## on server:

```
cd httpfs
go run server.go

```

## on client:
one example of uploading and download 100 files with size of 256KB 
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



