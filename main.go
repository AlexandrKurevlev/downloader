package main

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

func downloadFile(client *http.Client, url, savePath string) error {
	fmt.Println("Url:", url)
	err := os.MkdirAll(savePath, 0750)
	if err != nil {
		return err
	}

	resp, err := client.Head(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Размер файла
	contentLength := resp.Header.Get("Content-Length")
	fileSize, _ := strconv.ParseInt(contentLength, 10, 64)
	if fileSize == 0 {
		fmt.Println("Размер: неизвестен")
	} else {
		fmt.Println("Размер:", fileSize)
	}

	// Поддержка докачки
	acceptRanges := resp.Header.Get("Accept-Ranges")
	supportsResume := acceptRanges == "bytes"
	if supportsResume {
		fmt.Println("Докачка: поддерживается")
	} else {
		fmt.Println("Докачка: не поддерживается")
	}

	file, err := os.Create(path.Join(savePath, path.Base(url)))
	if err != nil {
		return err
	}
	defer file.Close()

	file.Truncate(fileSize)

	const chunkSize = 10 * 1024 * 1024 // 10 MB

	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	fmt.Println("Размер чанка:", chunkSize, "байт")
	fmt.Println("Количество чанков:", totalChunks)
	for i := 1; i <= totalChunks; i++ {
		start := chunkSize * (i - 1)
		end := int64(math.Min(float64(chunkSize*i-1), float64(fileSize-1)))
		fmt.Printf("Чанк %d/%d: байты %d-%d\n", i, totalChunks, start, end)

		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusPartialContent {
			return fmt.Errorf("сервер вернул %d", resp.StatusCode)
		}

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Использование: downloader <директория> <url1> [url2...]")
		os.Exit(1)
	}

	savePath := os.Args[1]
	urls := os.Args[2:]

	client := &http.Client{Timeout: 30 * time.Second}

	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			err := downloadFile(client, u, savePath)
			if err != nil {
				fmt.Println(err)
			}
		}(url)
	}

	wg.Wait()
}
