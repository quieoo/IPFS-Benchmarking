package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/rcrowley/go-metrics"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const MS = 1000000

func main() {
	var cmd string
	var filesize int
	var filenumber int
	var concurrentGet int
	var filenames string
	var host string
	const downloadfilepath = "downloaded"

	flag.StringVar(&cmd, "c", "", "operation type\n"+
		"upload: upload file with -s for file size, -n for file number, -fn for specified name of file storing all uploaded file name\n"+
		"download: download files following specified filename\n")
	flag.IntVar(&filesize, "s", 256*1024, "file size")
	flag.IntVar(&filenumber, "n", 1, "file number")
	flag.IntVar(&concurrentGet, "cg", 1, "concurrent get number")
	flag.StringVar(&filenames, "fn", "filenames", "name of files for uploading")
	flag.StringVar(&host, "h", "127.0.0.1", "server address")

	flag.Parse()

	uploadTimer := metrics.NewTimer()
	downloadTimer := metrics.NewTimer()

	if cmd == "upload" {
		fn, _ := os.Create(filenames)
		fmt.Printf("Uploading files with size %d B\n", filesize)

		createDirIfNotExist("./temp")

		for i := 0; i < filenumber; i++ {
			//create temporary file
			name := fmt.Sprintf("%d-%d", filesize, i)
			subs := NewLenChars(filesize, StdChars)
			err := ioutil.WriteFile("./temp/"+name, []byte(subs), 0666)
			if err != nil {
				fmt.Println("failed to write temporary file: ", err.Error())
				return
			}

			uploadstart := time.Now()
			//upload file
			bodyBuffer := &bytes.Buffer{}
			bodyWriter := multipart.NewWriter(bodyBuffer)

			fileWriter, _ := bodyWriter.CreateFormFile("files", name)

			file, _ := os.Open("./temp/" + name)

			_, err = io.Copy(fileWriter, file)
			if err != nil {
				fmt.Println("failed to copy file to buffer: ", err.Error())
				return
			}

			contentType := bodyWriter.FormDataContentType()
			bodyWriter.Close()

			resp, _ := http.Post("http://"+host+":8080/upload", contentType, bodyBuffer)

			resp.Body.Close()
			file.Close()
			//resp_body, _ := ioutil.ReadAll(resp.Body)
			uploadTimer.Update(time.Now().Sub(uploadstart))
			//name -> filename
			_, err = io.WriteString(fn, name+"\n")
			if err != nil {
				fmt.Println("failed to store filename: ", err.Error())
				return
			}
		}

		//cleaning temporary files
		_, err := os.Stat("./temp")
		if !(err != nil && os.IsNotExist(err)) {
			err := os.RemoveAll("./temp")
			if err != nil {
				fmt.Println("failed to remove temp directory: ", err.Error())
				return
			}
		}

		//output metrics
		fmt.Printf(standardOutput("http-upload", uploadTimer))
	} else if cmd == "download" {
		file, err := os.Open(filenames)
		defer file.Close()
		if err != nil {
			fmt.Println("failed to open filenames to read file name: ", err.Error())
			return
		}
		//br := bufio.NewReader(file)

		createDirIfNotExist(downloadfilepath)

		// 把cid读到多个切片中
		inputReader := bufio.NewReader(file)

		allFileName := make([][]string, concurrentGet)
		for i := range allFileName {
			allFileName[i] = make([]string, 0)
		}

		tmpCnt := 0
		for {
			aLine, readErr := inputReader.ReadString('\n')
			aLine = strings.TrimSuffix(aLine, "\n")
			if readErr == io.EOF {
				break
			}
			allFileName[tmpCnt%concurrentGet] = append(allFileName[tmpCnt%concurrentGet], aLine)
			tmpCnt++
		}

		// 创建多个协程，分别去get文件
		var wg sync.WaitGroup
		wg.Add(concurrentGet)
		for i := 0; i < concurrentGet; i++ {
			go func(theOrder int) {
				defer wg.Done()
				for j := 0; j < len(allFileName[theOrder]); j++ {
					toRequest := allFileName[theOrder][j]

					downloadstrat := time.Now()
					url := "http://" + host + ":8080/files/" + toRequest
					res, err := http.Get(url)
					if err != nil {
						fmt.Println("failed to request file: ", err.Error())
						return
					}

					f, err := os.Create(downloadfilepath + "/" + toRequest)
					if err != nil {
						fmt.Println("failed to create local file: ", err.Error())
						return
					}
					_, err = io.Copy(f, res.Body)
					if err != nil {
						fmt.Println("failed to copy response to local file: ", err.Error())
						return
					}
					downloadTimer.Update(time.Now().Sub(downloadstrat))

					if err != nil {
						fmt.Printf("thread %d downloading stall caused by %s", theOrder, err.Error())
						fmt.Printf(standardOutput("http-download", downloadTimer))
						return
					} else if j == len(allFileName[theOrder])-1 {
						fmt.Printf("thread %d downloading stall caused by havingfilesystem:https://docs.google.com/persistent/docs/documents/1HT8zjCAQULYnfpc_EahYXJ0MRoEIW53cR_zucqMB7wU/image/PLACEHOLDER_1e1273deb9e66546_0 downloaded all needed files.\n", theOrder)
						fmt.Printf(standardOutput("http-download", downloadTimer))
						return
					}
				}

			}(i)
		}
		wg.Wait()

	} else if cmd == "tracUpload" {
		traceFile := ""
	}

}

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

func createDirIfNotExist(path string) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir(path, 0777)
			if err != nil {
				fmt.Printf("failed to mkdir: %v\n", err.Error())
				return
			}
		} else {
			fmt.Println(err.Error())
			return
		}
	}
}

func standardOutput(function string, t metrics.Timer) string {
	return fmt.Sprintf("%s: %d files, average latency: %f ms, 0.99P latency: %f ms\n", function, t.Count(), t.Mean()/MS, t.Percentile(float64(t.Count())*0.99)/MS)
}
