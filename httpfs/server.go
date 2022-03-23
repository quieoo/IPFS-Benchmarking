package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
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

	http.HandleFunc("/upload", uploadHandler)

	fs := http.FileServer(http.Dir(filePath))
	http.Handle("/files/", http.StripPrefix("/files", fs))

	fmt.Println("http file server listening on 8080")
	http.ListenAndServe(":8080", nil)
}
