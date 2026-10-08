package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"
)

func downloadFile(url, savePath string) error {
	err := os.MkdirAll(savePath, 0750)
	if err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("сервер вернул %d", resp.StatusCode)
	}

	file, err := os.Create(path.Join(savePath, path.Base(url)))
	if err != nil {
		return err
	}
	defer file.Close()

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

	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			err := downloadFile(u, savePath)
			if err != nil {
				fmt.Println(err)
			}
		}(url)
	}

	wg.Wait()
}
