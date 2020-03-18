package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	uri              string
	username         *string
	password         *string
	downloadContinue *bool
)

func init() {
	downloadContinue = flag.Bool("c", true, "Continue download")
	username = flag.String("u", "", "User name")
	password = flag.String("p", "", "Password")
	help := flag.Bool("h", false, "Show help")

	flag.Parse()
	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if len(flag.Args()) < 1 {
		log.Fatalf("URL is nessacery: %s [options] url\n", os.Args[0])
	} else {
		uri = flag.Arg(0)
	}
}

func getFileSize(filename string) int64 {
	fstat, err := os.Stat(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		} else {
			return 0
		}
	}
	return fstat.Size()
}

func main() {
	client := &http.Client{}
	log.SetFlags(log.Ltime)

	request, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		panic(err)
	}

	formatURL, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}

	paths := strings.Split(formatURL.Path, "/")
	filename := paths[len(paths)-1]
	filesize := int64(0)

	request.SetBasicAuth(*username, *password)
	if *downloadContinue {
		filesize = getFileSize(filename)
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", filesize))
	}

	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	var f *os.File
	if response.StatusCode == 206 {
		f, err = os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}
	} else if 200 <= response.StatusCode && response.StatusCode < 300 {
		if *downloadContinue {
			log.Printf("Warning: can't continue download with url: %s\n", uri)
		}
		f, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}
	} else {
		log.Fatalf("Request error: %d %s\n", response.StatusCode, response.Status)
	}

	writer := &ProgressWriter{
		Dst:     f,
		Current: filesize,
		Total:   filesize + response.ContentLength,
	}

	if _, err := io.Copy(writer, response.Body); err != nil {
		panic(err)
	}
}
