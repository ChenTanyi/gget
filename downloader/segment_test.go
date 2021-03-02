package downloader

import (
	"bytes"
	"math/rand"
	"testing"
)

type WriteAtBuffer struct {
	buffer []byte
}

func (buff *WriteAtBuffer) WriteAt(b []byte, offset int64) (int, error) {
	size := copy(buff.buffer[offset:], b)
	if size < len(b) {
		return size, ErrOutOfWriterLimitation
	}
	return size, nil
}

func (buff *WriteAtBuffer) Bytes() []byte {
	return buff.buffer
}

type Range struct {
	Begin int64
	End   int64
}

func TestSegment(t *testing.T) {
	size := 100
	src := make([]byte, size)
	rand.Read(src)
	dst := &WriteAtBuffer{buffer: make([]byte, size)}

	seg := NewSegment(0, int64(size))
	err := seg.Start(1, dst)
	if err != nil {
		t.Error(err)
	}

	_, err = seg.Write(src[:size/2])
	if err != nil {
		t.Error(err)
	}

	_, err = seg.Write(src[size/2:])
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(src, dst.Bytes()) {
		t.Errorf("Copy error, src = %v, dst = %v", src, dst.Bytes())
	}
}

func TestMultiSegments(t *testing.T) {
	size := 1024 * 1024 * 200
	thread := 16
	src := make([]byte, size)
	rand.Read(src)
	dst := &WriteAtBuffer{buffer: make([]byte, size)}

	segs := NewSegments(nil)
	segs.InitSize(int64(size))
	ranges := make([]Range, thread)

	for i := 0; i < thread; i++ {
		seg, err := segs.Start(i+1, dst)
		if err != nil && err != ErrAllSegmentIsFinish {
			t.Error(err)
		}
		if seg != nil {
			ranges[i].Begin = seg.Begin()
			ranges[i].End = seg.End()
		}
	}

	for segs.Remaining() > 0 {
		index := rand.Intn(thread)
		for ranges[index].Begin >= ranges[index].End {
			index = (index + 1) % thread
		}
		sz := rand.Intn(int(ranges[index].End)-int(ranges[index].Begin)) + 1

		sz, err := segs.Write(index+1, src[ranges[index].Begin:int(ranges[index].Begin)+sz])
		ranges[index].Begin += int64(sz)
		if err != nil && err != ErrAllSegmentIsFinish && err != ErrSegmentFinish {
			t.Error(err)
		}

		if err == ErrAllSegmentIsFinish || ranges[index].Begin >= ranges[index].End {
			seg, err := segs.Start(index+1, dst)
			if err != nil && err != ErrAllSegmentIsFinish {
				t.Error(err)
			}
			if seg != nil {
				ranges[index].Begin = seg.Begin()
				ranges[index].End = seg.End()
			}
		}
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
