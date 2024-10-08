package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const filePath = "files"

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}

		//fmt.Printf("FileName=[%s], FormName=[%s]\n", part.FileName(), part.FormName())
		if part.FileName() == "" { // this is FormData
			data, _ := ioutil.ReadAll(part)
			fmt.Printf("FormData=[%s]\n", string(data))
		} else { // This is FileData
			dst, err := os.Create(filePath + "/" + part.FileName())
			if err != nil {
				fmt.Println("failed to create file for uploading: ", err.Error())
				return
			}
			_, err = io.Copy(dst, part)
			if err != nil {
				fmt.Println("failed to copy buffer to file: ", err.Error())
				return
			}
			dst.Close()
		}
	}
}

var STD = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

// NewLenChars returns a new random string of the provided length, consisting of the provided byte slice of allowed characters(maximum 256).
func RanDomContent(length int, chars []byte) string {
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

func main() {

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir(filePath, 0777)
			if err != nil {
				fmt.Printf("failed to mkdir: %v\n", err.Error())
				return
			}
		} else {
			fmt.Println(err.Error())
			return
		}
	}

	var cmd string
	var index int
	var servers int
	var trace_docs string
	var fileNumber int
	var fileSize int
	flag.StringVar(&cmd, "c", "", "operation type\n"+
		"traceUpload: generate trace files\n"+
		"upload: generate file with specified file number and file size\n")
	flag.StringVar(&trace_docs, "f", "", "file indicates the path of doc file of generated trace")
	flag.IntVar(&index, "i", 0, "given each server has a part of entire workload, index indicates current server own the i-th part of data. Index starts from 0.")
	flag.IntVar(&servers, "s", 1, "servers indicates the total number of servers")
	flag.IntVar(&fileNumber, "n", 1, "file number used for upload cmd")
	flag.IntVar(&fileSize, "size", 256, "file size used for upload cmd")
	flag.Parse()

	if cmd == "traceUpload" {
		if index < 0 || index >= servers {
			fmt.Println("Error, the index is out of range")
			return
		}
		tracefile := trace_docs
		if f, err := os.Open(tracefile); err != nil {
			fmt.Printf("failed to open trace file: %s, due to %s\n", tracefile, err.Error())
			return
		} else {
			defer f.Close()
			//parse and generate related file, with trace format:
			// ItemID | Popularity | Size(Bytes) | Application Type
			scanner := bufio.NewScanner(f)
			var sizes []int

			for scanner.Scan() {
				line := scanner.Text()
				codes := strings.Split(line, "\t")
				size, _ := strconv.Atoi(codes[2])
				sizes = append(sizes, size)
			}
			if err := scanner.Err(); err != nil {
				fmt.Printf("Cannot scanner text file: %s, err: [%v]\n", tracefile, err)
				return
			}

			//determine the server provide what files
			fileNumbers := len(sizes)
			for i := 0; i < fileNumbers; i++ {
				if i%100 == 0 {
					fmt.Printf("uploading %d %d \n", i, fileNumbers)
				}
				if i%servers == index {

					name := strconv.Itoa(i)
					size := sizes[i]
					content := RanDomContent(size, STD)
					err := ioutil.WriteFile("./"+filePath+"/"+name, []byte(content), 0666)
					if err != nil {
						fmt.Println("failed to generate file: ", err.Error())
						return
					}
				}
			}

		}
	} else if cmd == "upload" {
		fn, _ := os.Create("filenames")
		fmt.Printf("Uploading files with size %d B\n", fileSize)
		start := time.Now()
		for i := 0; i < fileNumber; i++ {
			if i%100 == 0 {
				fmt.Printf("uploading %d %d \n", i, fileNumber)
			}
			if i%servers == index {
				name := strconv.Itoa(fileSize) + "-" + strconv.Itoa(i)
				size := fileSize
				content := RanDomContent(size, STD)
				err := ioutil.WriteFile("./"+filePath+"/"+name, []byte(content), 0666)
				if err != nil {
					fmt.Println("failed to generate file: ", err.Error())
					return
				}

				_, err = io.WriteString(fn, name+"\n")
				if err != nil {
					fmt.Println("failed to store filename: ", err.Error())
					return
				}
			}
		}
		fmt.Printf("finished upload, latecy: %f ms, throughput: %f MB/s\n", float64(time.Now().Sub(start).Milliseconds())/float64(fileNumber), float64(fileNumber*fileSize)/float64(1024*1024)/float64(time.Now().Sub(start).Seconds()))
		fn.Close()
	}
	http.HandleFunc("/upload", uploadHandler)

	fs := http.FileServer(http.Dir(filePath))
	http.Handle("/files/", http.StripPrefix("/files", fs))

	fmt.Println("http file server listening on 8080")
	http.ListenAndServe(":8080", nil)
}
