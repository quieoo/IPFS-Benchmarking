package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Response struct {
	Data []Block `json:"data"`
}

type Block struct {
	Size        int   `json:"size"`
	Transaction int   `json:"transaction_count"`
	Timestamp   int64 `json:"time"`
}

const (
	BaseURL      = "https://api.blockchair.com/bitcoin/blocks"
	MaxBlockSize = 100
)

func main() {
	timePeriod := 24 * time.Hour // Define the desired time period
	endTime := time.Now()
	startTime := endTime.Add(-timePeriod)

	url := fmt.Sprintf("%s?limit=%d&q=time(>%d,%d)", BaseURL, MaxBlockSize, startTime.Unix(), endTime.Unix())

	fetchBlocks(url)
}

func fetchBlocks(url string) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("Failed to fetch data:", err)
		return
	}
	fmt.Println(url)
	defer resp.Body.Close()

	var data Response
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		fmt.Println("Failed to decode response data:", err)
		return
	}

	totalSize := 0
	totalTransactions := 0

	for _, block := range data.Data {
		totalSize += block.Size
		totalTransactions += block.Transaction
	}

	fmt.Printf("Total Size in the Last 24 Hours: %d\n", totalSize)
	fmt.Printf("Total Number of Transactions in the Last 24 Hours: %d\n", totalTransactions)
}
