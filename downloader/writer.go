package downloader

import (
	"errors"
	"io"
	"time"

	"github.com/sirupsen/logrus"
)

// ProgressWriter .
type ProgressWriter struct {
	Title   string
	Dst     io.Writer
	Current int64
	Total   int64

	previousWriteTime time.Time
	byteSincePrevious int64
}

// ErrOutOfWriterLimitation .
var ErrOutOfWriterLimitation = errors.New("Out of writer limitation")

// ChanWriter .
type ChanWriter struct {
	Dst        io.Writer
	Written    int64
	Limit      int64
	ResultChan chan<- int
	LimitChan  <-chan int64
}

// OffestWriter .
type OffestWriter struct {
	Dst    io.WriterAt
	Offset int64
}

// NewProgressWriter .
func NewProgressWriter(dst io.Writer, current, total int64, title string) *ProgressWriter {
	return &ProgressWriter{
		Title:   title,
		Dst:     dst,
		Current: current,
		Total:   total,
	}
}

func (p *ProgressWriter) Write(b []byte) (int, error) {
	n, err := p.Dst.Write(b)
	p.Current += int64(n)
	p.byteSincePrevious += int64(n)

	currentTime := time.Now()
	duration := currentTime.Sub(p.previousWriteTime)

	if duration < time.Second {
		return n, err
	}

	currentSize := SizeToReadable(float64(p.Current))
	totalSize := SizeToReadable(float64(p.Total))
	percentage := float64(p.Current) * 100.0 / float64(p.Total)
	speed := float64(p.byteSincePrevious) / duration.Seconds()
	logrus.Infof("%s: %s / %s, %.2f%%, %s/s\n", p.Title, currentSize, totalSize, percentage, SizeToReadable(speed))

	p.previousWriteTime = currentTime
	p.byteSincePrevious = 0
	return n, err
}

func (w *ChanWriter) Write(b []byte) (int, error) {
	select {
	case w.Limit = <-w.LimitChan:
	default:
	}
	n, err := w.Dst.Write(b)
	if n > 0 {
		w.Written += int64(n)
		w.ResultChan <- n
	}
	if w.Written >= w.Limit {
		return 0, ErrOutOfWriterLimitation
	}
	return n, err
}

func (w *OffestWriter) Write(b []byte) (int, error) {
	n, err := w.Dst.WriteAt(b, w.Offset)
	w.Offset += int64(n)
	return n, err
}
