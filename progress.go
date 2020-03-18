package main

import (
	"fmt"
	"io"
	"log"
	"time"
)

type ProgressWriter struct {
	Dst               io.Writer
	Current           int64
	Total             int64
	PreviousWriteTime time.Time
	ByteSincePrevious int64
}

func NewProgressWriter(dst io.Writer, current, total int64) *ProgressWriter {
	return &ProgressWriter{
		Dst:     dst,
		Current: current,
		Total:   total,
	}
}

func sizeToReadable(size float64) string {
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

func (p *ProgressWriter) Write(b []byte) (int, error) {
	n, err := p.Dst.Write(b)
	p.Current += int64(n)
	p.ByteSincePrevious += int64(n)

	currentTime := time.Now()
	duration := currentTime.Sub(p.PreviousWriteTime)

	if duration < time.Second {
		return n, err
	}

	currentSize := sizeToReadable(float64(p.Current))
	totalSize := sizeToReadable(float64(p.Total))
	percentage := float64(p.Current) * 100.0 / float64(p.Total)
	speed := float64(p.ByteSincePrevious) / duration.Seconds()
	log.Printf("Write: %s / %s, %.2f%%, %s/s\n", currentSize, totalSize, percentage, sizeToReadable(speed))

	p.PreviousWriteTime = currentTime
	p.ByteSincePrevious = 0
	return n, err
}
