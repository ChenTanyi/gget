package downloader

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/chentanyi/go-utils/filehash"
	"github.com/chentanyi/go-utils/interrupt-hook"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
)

var (
	MinimalSegment int64 = 256 * 1024
	ReadTimeout          = 20 * time.Second
)

// Downloader .
type Downloader struct {
	Client *http.Client
}

// Job .
type Job struct {
	Index      int
	Segment    *Segment
	preEnd     int64
	ResultChan chan int
	LimitChan  chan int64
}

type result struct {
	index int
	size  int64
}

// NewDefaultDownloader .
func NewDefaultDownloader() *Downloader {
	d := &Downloader{}

	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	d.Client = &http.Client{
		Jar: jar,
	}
	return d
}

// NewDownloader .
func NewDownloader(client *http.Client) *Downloader {
	d := &Downloader{
		Client: client,
	}

	return d
}

// NewJob .
func NewJob(Segment *Segment, index, thread int) *Job {
	return &Job{
		Index:      index,
		Segment:    Segment,
		ResultChan: make(chan int, thread),
		LimitChan:  make(chan int64, thread),
	}
}

// Download .
func (d *Downloader) Download(uri string) error {
	request, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return err
	}

	return d.DownloadFile(request, 0, "")
}

// DownloadFile .
func (d *Downloader) DownloadFile(request *http.Request, threadCount int, filename string) (err error) {
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		err = fmt.Errorf("%+v", r)
	// 	}
	// }()

	if filename == "" {
		filename = ExtractFilenameFromURI(request.URL)
	}
	if threadCount < 1 {
		threadCount = 16
	}
	if threadCount == 1 {
		return d.SingleThreadDownload(request, filename)
	}

	canContinue, contentLength, err := d.DetectContinueDownload(request)
	if err != nil {
		panic(err)
	}
	if !canContinue {
		return d.SingleThreadDownload(request, filename)
	}

	stateFilename := filename + ".state"
	stateFile, err := os.OpenFile(stateFilename, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer stateFile.Close()
	state, err := ioutil.ReadAll(stateFile)
	if err != nil {
		panic(err)
	}
	segments, err := SegmentsReadFromByte(state)
	if err != nil {
		panic(err)
	}

	saveSegments := func() {
		b := segments.ToByte()
		fmt.Printf("Segments: %s\n", string(b))
		stateFile.Truncate(0)
		stateFile.WriteAt(b, 0)
	}
	defer saveSegments()
	interrupt.Add("saveSegments", saveSegments)
	defer interrupt.Remove("saveSegments")

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	segments.InitSize(contentLength)
	jobs := make([]*Job, threadCount)
	resultChan := make(chan result, threadCount)

	logrus.Debugf("Read Segments %+v %d", segments.Segments(), len(segments.Segments()))

	for i := 0; i < threadCount; i++ {
		d.CreateNewJob(segments, jobs, i, threadCount)
		if jobs[i] != nil {
			go d.StartJob(request, jobs[i], file, resultChan)
		}
	}

	remaining := segments.Remaining()
	for remaining > 0 {
		jobsCount := make([]bool, threadCount)
		timer := time.NewTicker(time.Second)
		timerCount := 0
		for { // per read timeout
		LoopPerSecond:
			for {
				select {
				case res := <-resultChan:
					jobsCount[res.index] = true
					err := segments.Add(res.index+1, res.size)
					if err != nil {
						panic(err)
					}
					if jobs[res.index] != nil && jobs[res.index].Segment.Finish() {
						d.CreateNewJob(segments, jobs, res.index, threadCount)
						if jobs[res.index] != nil {
							go d.StartJob(request, jobs[res.index], file, resultChan)
						}
					}
				case <-timer.C:
					break LoopPerSecond
				}
			}

			current := segments.Remaining()
			logrus.Infof("Download %s: %s / %s, speed %s/s", filename, SizeToReadable(float64(contentLength-current)),
				SizeToReadable(float64(contentLength)), SizeToReadable(float64(remaining-current)))
			// logrus.Debugf("Current Segments: %s", segments.Readable())
			remaining = current
			if remaining <= 0 {
				break
			}

			timerCount++
			for i, job := range jobs {
				if job == nil {
					d.CreateNewJob(segments, jobs, i, threadCount)
					jobsCount[i] = false
				}
				job = jobs[i]
				if job != nil {
					if timerCount == int(ReadTimeout/time.Second)+1 {
						if !jobsCount[i] && !job.Segment.Finish() {
							go d.StartJob(request, job, file, resultChan)
						}
					}
					seg, err := segments.Start(i+1, job.preEnd, job.LimitChan)
					job.preEnd = job.Segment.End()
					if seg != nil || err != nil {
						if err == ErrSendLimit {
							logrus.Warnf("Cannot send limit to job %d", i+1)
						} else {
							panic(fmt.Errorf("Unexpected segment during start, job = %+v, err = %v", job, err))
						}
					}
				}
			}
			if timerCount == int(ReadTimeout/time.Second)+1 {
				break
			}
		}
	}

	logrus.Infof("Finish download %s, %s", filename, SizeToReadable(float64(contentLength)))
	return nil
}

// CreateNewJob .
func (d *Downloader) CreateNewJob(segments *Segments, jobs []*Job, index, threadCount int) {
	seg, err := segments.Start(index+1, 0, nil)
	if seg == nil {
		if err != ErrAllSegmentIsFinish {
			if err == nil {
				err = fmt.Errorf("Job already exists when start, id: %d", index+1)
			}
			panic(err)
		}
		jobs[index] = nil
	} else {
		jobs[index] = NewJob(seg, index, threadCount)
	}
}

// SingleThreadDownload .
func (d *Downloader) SingleThreadDownload(request *http.Request, filename string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%+v", r)
		}
	}()

	logrus.Debugf("Single thread download: %s", request.URL)

	if filename == "" {
		filename = ExtractFilenameFromURI(request.URL)
		logrus.Debugf("Get filename %s", filename)
	}
	filesize := GetFileSize(filename)
	SetSuffixRange(request, filesize)

	response, err := d.Client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	logrus.Debugf("Open file %s", filename)
	var file *os.File
	if response.StatusCode == 206 {
		file, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}
	} else if 200 <= response.StatusCode && response.StatusCode < 300 {
		logrus.Warnf("Cannot continue download, uri = %s", request.URL)
		file, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		filesize = 0
		if err != nil {
			panic(err)
		}
	} else {
		panic(fmt.Errorf("Request error, code = %d, status = %s", response.StatusCode, response.Status))
	}

	writer := &ProgressWriter{
		Title:   fmt.Sprintf("Write to %s", filename),
		Dst:     file,
		Current: filesize,
		Total:   filesize + response.ContentLength,
	}
	copySize := CopyWithReadTimeout(writer, response.Body, ReadTimeout)
	if copySize < response.ContentLength {
		panic(ErrReadTimeout)
	}
	return nil
}

// StartJob .
func (d *Downloader) StartJob(req *http.Request, job *Job, dst io.WriterAt, resultChan chan<- result) {
	request := req.Clone(context.Background())
	SetRange(request, job.Segment.Current(), job.Segment.End()-1)
	var writer io.Writer = &OffestWriter{
		Dst:    dst,
		Offset: job.Segment.Current(),
	}
	writer = &ChanWriter{
		Dst:        writer,
		Written:    job.Segment.Current(),
		Limit:      job.Segment.End(),
		LimitChan:  job.LimitChan,
		ResultChan: job.ResultChan,
	}

	response, err := d.Client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 206 {
		return
	}
	go io.Copy(writer, response.Body)

	for {
		select {
		case res := <-job.ResultChan:
			resultChan <- result{job.Index, int64(res)}
		case <-time.After(ReadTimeout):
			return
		}
	}
}

// DetectContinueDownload .
func (d *Downloader) DetectContinueDownload(req *http.Request) (bool, int64, error) {
	request := req.Clone(context.Background())
	request.Method = "HEAD"
	SetSuffixRange(request, 1)

	response, err := d.Client.Do(request)
	if err != nil {
		return false, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode == 206 {
		return true, response.ContentLength + 1, nil
	} else if 200 <= response.StatusCode && response.StatusCode < 300 {
		return false, response.ContentLength, nil
	}
	return false, 0, fmt.Errorf("Request error: code = %d, status = %s", response.StatusCode, response.Status)
}

// FilterUnmatchedHash .
func (d *Downloader) FilterUnmatchedHash(request *http.Request, filename, maxLenStr, startStr string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%+v", r)
		}
	}()

	maxLen, err := SizeToInt(maxLenStr)
	if err != nil {
		return err
	}
	if maxLen <= 0 || (maxLen&(maxLen-1) != 0) {
		return fmt.Errorf("Unexpected segment length %d", maxLen)
	}

	if filename == "" {
		filename = ExtractFilenameFromURI(request.URL)
	}
	logrus.Debugf("Max segment: %d, for file: %s", maxLen, filename)

	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	stateFile, err := os.OpenFile(filename+".state", os.O_RDWR, 0755)
	if err != nil {
		panic(err)
	}
	defer stateFile.Close()
	state, err := ioutil.ReadAll(stateFile)
	if err != nil {
		panic(err)
	}
	segments, err := SegmentsReadFromByte(state)
	if err != nil {
		panic(err)
	}

	saveSegments := func() {
		b := segments.ToByte()
		fmt.Printf("Segments: %s\n", string(b))
		stateFile.Truncate(0)
		stateFile.WriteAt(b, 0)
	}
	defer saveSegments()
	interrupt.Add("saveSegments", saveSegments)
	defer interrupt.Remove("saveSegments")

	start, err := SizeToInt(startStr)
	if err != nil || start < 0 {
		start = 0
	}

	canContinue, size, err := d.DetectContinueDownload(request)
	if !canContinue || err != nil {
		panic(fmt.Errorf("Can't get part of content, %v", err))
	}

	segments.InitSize(size)

	for begin, end := start, start+maxLen; begin < size; begin, end = end, end+maxLen {
		if end > size {
			end = size
		}
		d.FilterUnmatchedHashSegments(request, file, begin, end, segments)
	}

	return nil
}

// FilterUnmatchedHashSegments .
func (d *Downloader) FilterUnmatchedHashSegments(req *http.Request, src io.ReaderAt, begin, end int64, segments *Segments) {
	hash1, err := filehash.HashFile(src, begin, end)
	if err != nil {
		panic(err)
	}

	SetRange(req, begin, end-1)
	if req.URL.Query().Get("hash") != "sha1" {
		query := req.URL.Query()
		query.Set("hash", "sha1")
		req.URL.RawQuery = query.Encode()
	}

	response, err := d.Client.Do(req)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	if 200 <= response.StatusCode && response.StatusCode < 300 {
		hash2, err := ioutil.ReadAll(response.Body)
		if err != nil {
			panic(err)
		}

		equals := (hex.EncodeToString(hash1) == strings.TrimSpace(string(hash2)))
		equalsStr := "!="
		if equals {
			equalsStr = "=="
		}

		logrus.Debugf("Calc hash: %s - %s, %x %s %s", SizeToReadable(float64(begin)), SizeToReadable(float64(end)),
			hash1, equalsStr, string(hash2))
		if !equals {
			if end-begin <= 2*MinimalSegment {
				segments.Remove(begin, end)
			} else {
				mid := begin + (end-begin)/2
				d.FilterUnmatchedHashSegments(req, src, begin, mid, segments)
				d.FilterUnmatchedHashSegments(req, src, mid, end, segments)
			}
		}
	} else {
		panic(fmt.Errorf("Request error, code = %d, status = %s", response.StatusCode, response.Status))
	}
}
