package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ipfs/kubo/client/rpc"
	"github.com/multiformats/go-multihash"
)

func main() {
	// Connect to IPFS node
	client, err := rpc.NewClient("http://127.0.0.1:5001")
	if err != nil {
		log.Fatal(err)
	}

	// CID to search for providers
	cidStr := "QmbgqimM8dgdkQCSpwfFWa5W7jfmKAeJdFVsimyNipst73"
	cid, err := multihash.FromB58String(cidStr)
	if err != nil {
		log.Fatal(err)
	}

	// Call FindProviders
	ctx := context.Background()
	providers, err := client.Routing.FindProviders(ctx, cid)
	if err != nil {
		log.Fatal(err)
	}

	// Print providers
	for _, p := range providers {
		fmt.Println("Found provider:", p.ID)
	}
}
