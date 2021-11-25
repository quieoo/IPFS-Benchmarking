# server
run http file server on localhost:8080, "/uploads" for uploading files, "/files" for downloading files
````
go run server.go
or
go build server.go && ./server
````
uploaded files saved in "./files/"
# client
build:
````
go run client.go
of
go build client.go && ./client
````
usage:
````
  -c string
        operation type
        upload: upload file with -s for file size, -n for file number, -fn for specified name of file storing all uploaded file name
        download: download files following specified filename
    
  -fn string
        name of files for uploading (default "filenames")
  -h string
        server address (default "127.0.0.1")
  -n int
        file number (default 1)
  -s int
        file size (default 262144)
````
downloaded files saved in "./downloaded/"
