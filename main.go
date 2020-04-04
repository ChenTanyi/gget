package main

import (
	"flag"
	"log"
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
	downloadContinue *bool
)

// ParseArgs .
func ParseArgs() {
	downloadContinue = flag.Bool("c", true, "Continue download")
	username = flag.String("u", "", "User name")
	password = flag.String("p", "", "Password")
	filename = flag.String("o", "", "Output File")
	thread = flag.Int("x", 8, "thread number")
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

func main() {
	log.SetFlags(log.Ltime)
	logrus.SetLevel(logrus.InfoLevel)
	ParseArgs()

	request, _ := http.NewRequest("GET", uri, nil)
	request.SetBasicAuth(*username, *password)
	logrus.Debugf("Request uri: %s", request.URL.String())
	for {
		if err := downloader.NewDefaultDownloader().DownloadFile(request, *thread, *filename); err != nil {
			log.Printf("Download error: %v, continue", err)
		} else {
			return
		}
	}
}
