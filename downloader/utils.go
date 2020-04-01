package downloader

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ErrReadTimeout .
var ErrReadTimeout = errors.New("Read Timeout")

// SizeToReadable .
func SizeToReadable(size float64) string {
	if size < 0 {
		return "-"
	}
	var suffixes = []string{" B", "KB", "MB", "GB", "TB", "PB", "EB"}
	for index := 0; index < len(suffixes); index++ {
		if size < 1024 {
			return fmt.Sprintf("%.2f %s", size, suffixes[index])
		}
		size /= float64(1024)
	}
	return "Too Large"
}

// AddRange .
func AddRange(request *http.Request, begin, end int64) {
	request.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", begin, end))
}

// AddSuffixRange .
func AddSuffixRange(request *http.Request, begin int64) {
	request.Header.Add("Range", fmt.Sprintf("bytes=%d-", begin))
}

// GetFileSize .
func GetFileSize(filename string) int64 {
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

// ExtractFilenameFromURI .
func ExtractFilenameFromURI(uri *url.URL) string {
	paths := strings.Split(uri.Path, "/")
	return paths[len(paths)-1]
}

// CopyWithReadTimeout .
func CopyWithReadTimeout(dst io.Writer, src io.Reader, timeout time.Duration) int64 {
	result := make(chan int)
	var total int64
	writer := &ChanWriter{
		Dst:        dst,
		Limit:      math.MaxInt64,
		ResultChan: result,
	}
	go io.Copy(writer, src)

	for {
		select {
		case n := <-result:
			total += int64(n)
		case <-time.After(timeout):
			return total
		}
	}
}
