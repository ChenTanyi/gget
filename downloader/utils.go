package downloader

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
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

// SizeToInt .
func SizeToInt(readable string) (int64, error) {
	if readable == "" {
		return 0, nil
	}
	base := int64(1)
	switch readable[len(readable)-1] {
	case 'E', 'e':
		base *= 1024
		fallthrough
	case 'P', 'p':
		base *= 1024
		fallthrough
	case 'T', 't':
		base *= 1024
		fallthrough
	case 'G', 'g':
		base *= 1024
		fallthrough
	case 'M', 'm':
		base *= 1024
		fallthrough
	case 'K', 'k':
		base *= 1024
		fallthrough
	case 'B', 'b':
		readable = readable[:len(readable)-1]
	default:
	}
	size, err := strconv.ParseFloat(readable, 64)
	if err != nil {
		return 0, err
	}
	leftSize := float64(0)
	if size-float64(int64(size)) > 1e-6 {
		leftSize = (size - float64(int64(size))) * float64(base)
	}
	return int64(size)*base + int64(leftSize), nil
}

// SetRange .
func SetRange(request *http.Request, begin, end int64) {
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", begin, end))
}

// SetSuffixRange .
func SetSuffixRange(request *http.Request, begin int64) {
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-", begin))
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
func CopyWithReadTimeout(dst io.Writer, src io.Reader, timeout time.Duration) (int64, error) {
	writer := NewChanWriter(8)
	go io.Copy(writer, src)

	total := int64(0)
	for {
		select {
		case b := <-writer.Chan():
			n, err := dst.Write(b)
			total += int64(n)
			if err != nil {
				return total, err
			}
		case <-time.After(timeout):
			return total, ErrReadTimeout
		}
	}
}
