package main

import (
	"encoding/json"
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

const chunkSize = 10 * 1024 * 1024 // 10 MB
const maxRetries = 3
const retryDelay = 2 * time.Second

type DownloadState struct {
	URL              string `json:"url"`
	TotalSize        int64  `json:"total_size"`
	ChunkSize        int    `json:"chunk_size"`
	TotalChunks      int    `json:"total_chunks"`
	DownloadedChunks []bool `json:"downloaded_chunks"`
}

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

	file, err := os.OpenFile(path.Join(savePath, path.Base(url)), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Truncate(fileSize)

	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	fmt.Println("Размер чанка:", chunkSize, "байт")
	fmt.Println("Количество чанков:", totalChunks)

	state := &DownloadState{
		URL:              url,
		TotalSize:        fileSize,
		ChunkSize:        chunkSize,
		TotalChunks:      totalChunks,
		DownloadedChunks: make([]bool, totalChunks),
	}
	progressFileName := path.Join(savePath, path.Base(url)+".progress")
	_, err = os.Stat(progressFileName)
	if err != nil {
		if os.IsNotExist(err) {
			data, _ := json.MarshalIndent(state, "", " ")
			os.WriteFile(path.Join(savePath, path.Base(url)+".progress"), data, 0644)
		} else {
			return err
		}
	} else {
		data, _ := os.ReadFile(progressFileName)
		json.Unmarshal(data, &state)
	}
	for i := 1; i <= totalChunks; i++ {
		if state.DownloadedChunks[i-1] {
			fmt.Printf("Чанк %d уже загружен, пропускаем\n", i)
			continue
		}

		for attempt := 1; attempt <= maxRetries; attempt++ {
			start := int64(chunkSize * (i - 1))
			end := int64(math.Min(float64(chunkSize*i-1), float64(fileSize-1)))
			fmt.Printf("Чанк %d/%d: байты %d-%d\n", i, totalChunks, start, end)

			err = downloadChunk(client, start, end, state, file)
			if err == nil {
				state.DownloadedChunks[i-1] = true
				data, _ := json.MarshalIndent(state, "", " ")
				os.WriteFile(progressFileName, data, 0644)

				break
			}

			if attempt < maxRetries {
				fmt.Printf("Ошибка %v, повтор через %v...\n", err, retryDelay)
				time.Sleep(retryDelay)
			} else {
				fmt.Printf("Ошибка: %v", err)
				break
			}
		}
	}

	return nil
}

func downloadChunk(client *http.Client, start, end int64, state *DownloadState, file *os.File) error {
	req, _ := http.NewRequest("GET", state.URL, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("сервер вернул %d", resp.StatusCode)
	}

	file.Seek(start, io.SeekStart)
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
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
