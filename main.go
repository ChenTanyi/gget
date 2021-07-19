package main

import (
	"net/http"
	"os"
	"regexp"

	"github.com/chentanyi/gget/downloader"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	uri              string
	username         *string
	password         *string
	filename         *string
	thread           *int
	hashLen          *string
	start            *string
	downloadContinue *bool
	debug            *bool
)

// ParseArgs .
func ParseArgs() {
	cmd := &cobra.Command{
		Use: "gget <url>",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				cmd.Usage()
				os.Exit(0)
			}
			uri = args[0]
		},
	}
	downloadContinue = cmd.PersistentFlags().BoolP("continue", "c", true, "Continue Download")
	username = cmd.PersistentFlags().StringP("username", "u", "", "Username")
	password = cmd.PersistentFlags().StringP("password", "p", "", "Password")
	filename = cmd.PersistentFlags().StringP("output", "o", "", "Output File")
	thread = cmd.PersistentFlags().IntP("concurrent", "j", 8, "Concurrent Download Thread Number")
	hashLen = cmd.PersistentFlags().StringP("len", "l", "", "Max len to check downloaded file hash rather than do download, only compliable for github.com/chentanyi/fileserver")
	start = cmd.PersistentFlags().StringP("start", "s", "0", "Start position to check hash")
	debug = cmd.PersistentFlags().Bool("debug", false, "Show Debug Log")

	err := cmd.Execute()
	if err != nil {
		cmd.Usage()
		panic(err)
	}
	if uri == "" {
		os.Exit((0))
	}
}

func main() {
	ParseArgs()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	if match, err := regexp.MatchString("https?://", uri); !match || err != nil {
		logrus.Errorf("Unsupport uri: %s", uri)
		panic(err)
	}

	request, _ := http.NewRequest("GET", uri, nil)
	request.SetBasicAuth(*username, *password)
	logrus.Debugf("Request uri: %s", request.URL.String())
	if *hashLen != "" {
		if err := downloader.NewDefaultDownloader().FilterUnmatchedHash(request, *filename, *hashLen, *start); err != nil {
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
