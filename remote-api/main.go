package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/multiformats/go-multiaddr"
	"io/ioutil"
	"os"

	httpapi "github.com/ipfs/go-ipfs-http-client"
)

var StdChars = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

// NewLenChars returns a new random string of the provided length, consisting of the provided byte slice of allowed characters(maximum 256).
func NewLenChars(length int, chars []byte) string {
	if length == 0 {
		return ""
	}
	clen := len(chars)
	if clen < 2 || clen > 256 {
		panic("Wrong charset length for NewLenChars()")
	}
	maxrb := 255 - (256 % clen)
	b := make([]byte, length)
	r := make([]byte, length+(length/4)) // storage for random bytes.
	i := 0
	for {
		if _, err := rand.Read(r); err != nil {
			panic("Error reading random bytes: " + err.Error())
		}
		for _, rb := range r {
			c := int(rb)
			if c > maxrb {
				continue // Skip this number to avoid modulo bias.
			}
			b[i] = chars[c%clen]
			i++
			if i == length {
				return string(b)
			}
		}
	}
}

func getUnixfsNode(path string) (files.Node, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := files.NewSerialFile(path, false, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func main() {
	var filesize int
	var filenumber int
	var http string
	flag.IntVar(&filesize, "s", 256*1024, "file size")
	flag.IntVar(&filenumber, "n", 0, "file nmber")
	flag.StringVar(&http, "p", "127.0.0.1", "ipv4 address of connect ipfs node (node should keep port 5001 open for rest api call)")
	flag.Parse()

	//api, err := httpapi.NewLocalApi()
	muladr, _ := multiaddr.NewMultiaddr("/ip4/" + http + "/tcp/5001")

	api, err := httpapi.NewApi(muladr)
	if err != nil {
		fmt.Printf(err.Error())
		return
	}
	tf := 0
	_, err = os.Stat("./temp")
	if err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir("./temp", 0777)
			if err != nil {
				fmt.Printf("failed to mkdir: %v\n", err.Error())
				return
			}
		} else {
			fmt.Println(err.Error())
			return
		}
	}
	for {
		tf++
		subs := NewLenChars(filesize, StdChars)
		inputpath := fmt.Sprintf("./temp/%d", tf)
		tf++
		//fmt.Println(inputpath)
		err := ioutil.WriteFile(inputpath, []byte(subs), 0666)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}

		f, _ := getUnixfsNode(inputpath)
		cid, err := api.Unixfs().Add(context.Background(), f)
		if err != nil {
			fmt.Printf(err.Error())
		}
		fmt.Printf("%s\n", cid)
		if filenumber != 0 {
			if tf >= filenumber {
				break
			}
		}
	}
}
