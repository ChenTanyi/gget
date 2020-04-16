package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/chentanyi/gget/downloader"
	"github.com/sirupsen/logrus"
)

var (
	uri              string
	username         *string
	password         *string
	filename         *string
	thread           *int
	queryLen         *string
	start            *string
	downloadContinue *bool
)

// ParseArgs .
func ParseArgs() {
	downloadContinue = flag.Bool("c", true, "Continue download")
	username = flag.String("u", "", "User name")
	password = flag.String("p", "", "Password")
	filename = flag.String("o", "", "Output File")
	thread = flag.Int("x", 8, "thread number")
	queryLen = flag.String("l", "", "Max len to query hash")
	start = flag.String("s", "0", "start position to query hash")
	help := flag.Bool("h", false, "Show help")

	flag.Parse()
	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if len(flag.Args()) < 1 {
		panic(fmt.Errorf("URL is nessacery: %s [options] url", os.Args[0]))
	} else {
		uri = flag.Arg(0)
	}
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	ParseArgs()

	request, _ := http.NewRequest("GET", uri, nil)
	request.SetBasicAuth(*username, *password)
	logrus.Debugf("Request uri: %s", request.URL.String())
	if *queryLen != "" {
		if err := downloader.NewDefaultDownloader().FilterUnmatchedHash(request, *filename, *queryLen, *start); err != nil {
			logrus.Errorf("Filter hash error: %v", err)
		}
	} else {
		for {
			if err := downloader.NewDefaultDownloader().DownloadFile(request, *thread, *filename); err != nil {
				logrus.Errorf("Download error: %v, continue", err)
			} else {
				return
			}
		}
	}
}
