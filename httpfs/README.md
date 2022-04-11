# server
run http file server on "localhost:8080", "/uploads" for uploading files, "/files" for downloading files
````
go run server.go
or
go build server.go && ./server
````

````
Usage of ./server:
  -c string
    	operation type
    	traceUpload: generate trace files
    	
  -f string
    	file indicates the path of doc file of generated trace
  -i int
    	given each server has a part of entire workload, index indicates current server own the i-th part of data. Index starts from 0.
  -s int
    	servers indicates the total number of servers (default 1)
````
# client
build:
````
go run client.go
of
go build client.go && ./client
````

````
Usage of ./client:
  -c string
    	operation type
    	upload: upload file with -s for file size, -n for file number, -fn for specified name of file storing all uploaded file name
    	download: download files following specified filename
    	traceDownload: download according to generated trace file
    	
  -cg int
    	concurrent get number (default 1)
  -f string
    	file indicates the path of workload file of generated trace
  -fn string
    	name of files for uploading (default "filenames")
  -h string
    	server addresses, seperated by commas (default "127.0.0.1")
  -n int
    	file number (default 1)
  -randomRequest
    	random request means that current client will randomly reorder requests from generated workload
  -s int
    	servers indicates the total number of servers (default 1)
  -size int
    	file size (default 262144)
````

