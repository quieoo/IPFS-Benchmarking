# go1.15 is required
cd go-ipfs/
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build
cp cmd/ipfs/ipfs ../ipfs

cd ../
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o xipfs