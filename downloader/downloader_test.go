package downloader

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

type handler struct {
	sync.Mutex
	modTime time.Time
	src     io.ReadSeeker
}

type ResponseWriter struct {
	w http.ResponseWriter
}

func (r *ResponseWriter) Header() http.Header {
	return r.w.Header()
}

func (r *ResponseWriter) Write(b []byte) (int, error) {
	logrus.Debugf("Wrtie %v", b)
	return r.w.Write(b)
}

func (r *ResponseWriter) WriteHeader(statusCode int) {
	r.w.WriteHeader(statusCode)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Lock()
	defer h.Unlock()

	logrus.Debugf("Receive range %s", r.Header.Get("Range"))
	h.modTime = h.modTime.Add(time.Millisecond)
	http.ServeContent(&ResponseWriter{w: w}, r, "test", h.modTime, h.src)
}

func TestMultiThreadDownload(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	rand.Seed(time.Now().Unix())
	size := int64(1024 * 1024)
	thread := 16
	src := make([]byte, size)
	// rand.Read(src)
	dst := &WriteAtBuffer{buffer: make([]byte, size)}

	count := 0
	for {
		for i := 0; i < 256; i++ {
			for j := 0; j < 256; j++ {
				if int64(count*256*256+i*256+j) >= size {
					break
				}
				src[count*256*256+i*256+j] = byte((count + i + j) % 256)
			}
		}
		count++
		if int64(count*256*256) >= size {
			break
		}
	}

	f, err := os.OpenFile("test", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Error(err)
	}
	defer os.Remove("test")
	defer f.Close()

	f.Truncate(0)
	f.Seek(0, 0)
	_, err = f.Write(src)
	if err != nil {
		t.Error(err)
	}

	server := httptest.NewServer(&handler{src: f, modTime: time.Now().Add(-20000 * time.Hour)})
	defer server.Close()

	request, _ := http.NewRequest("GET", server.URL, nil)
	segs := NewSegments(nil)
	segs.InitSize(int64(size))

	err = NewDefaultDownloader().MultiThreadDownload(request, segs, dst, "test", size, thread)
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(src, dst.Bytes()) {
		errorCount := 0
		for i, b := range src {
			if b != dst.Bytes()[i] {
				t.Logf("Not Match in %d, src: %d, dst %d", i, b, dst.Bytes()[i])
				errorCount++
			}
			if errorCount > 100 {
				break
			}
		}
		t.Error("Copy error")
	}
}
