package downloader

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// ErrWrongSegmentFormat .
var ErrWrongSegmentFormat = errors.New("Wrong Segment Format")

// Segment .
type Segment struct {
	Begin int64
	End   int64
}

// Segments .
type Segments struct {
	segments []*Segment
}

func (s *Segment) String() string {
	return fmt.Sprintf("%d-%d", s.Begin, s.End)
}

// Length .
func (s *Segment) Length() int64 {
	return s.End - s.Begin
}

// Finish .
func (s *Segment) Finish() bool {
	return s.Begin >= s.End
}

// NewSegments .
func NewSegments(segs []*Segment) *Segments {
	return &Segments{
		segments: segs,
	}
}

// SegmentsReadFromByte .
func SegmentsReadFromByte(b []byte) (*Segments, error) {
	segmentStrs := strings.Split(string(b), ",")
	s := &Segments{}
	s.segments = make([]*Segment, 0, len(segmentStrs))
	// logrus.Debugf("Segments %+v %d", segmentStrs, len(segmentStrs))
	for _, segmentStr := range segmentStrs {
		segmentStr = strings.TrimSpace(segmentStr)
		if segmentStr == "" {
			continue
		}

		segment := strings.Split(segmentStr, "-")
		if len(segment) != 2 {
			return nil, ErrWrongSegmentFormat
		}
		begin, err := strconv.ParseInt(segment[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Parse segment error: segment = %s, err = %v", segmentStr, err)
		}
		end, err := strconv.ParseInt(segment[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Parse segment error: segment = %s, err = %v", segmentStr, err)
		}
		s.segments = append(s.segments, &Segment{Begin: begin, End: end})
	}
	return s, nil
}

// Segments .
func (s *Segments) Segments() []*Segment {
	return s.segments
}

// Remaining .
func (s *Segments) Remaining(total int64) []*Segment {
	if len(s.segments) == 0 {
		return []*Segment{&Segment{Begin: 0, End: total}}
	}
	s.CleanOverlap()
	remaining := make([]*Segment, 0, len(s.segments))
	for i, seg := range s.segments {
		if i == 0 && seg.Begin > 0 {
			remaining = append(remaining, &Segment{Begin: 0, End: seg.Begin})
		}
		if i+1 < len(s.segments) {
			remaining = append(remaining, &Segment{Begin: seg.End, End: s.segments[i+1].Begin})
		}
		if i+1 == len(s.segments) && seg.End < total {
			remaining = append(remaining, &Segment{Begin: seg.End, End: total})
		}
	}
	return remaining
}

// ToByte .
func (s *Segments) ToByte() []byte {
	buffer := bytes.NewBuffer(nil)
	for index, segment := range s.segments {
		if index > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(segment.String())
	}
	return buffer.Bytes()
}

// IsOverlaped .
func (s *Segments) IsOverlaped() bool {
	var end int64 = -1
	for _, seg := range s.segments {
		if seg.Begin < end {
			return false
		}
		end = seg.End
	}
	return true
}

// CleanOverlap .
func (s *Segments) CleanOverlap() {
	if !s.IsOverlaped() {
		newSegments := make([]*Segment, 0, len(s.segments))
		var end int64 = -1
		for _, seg := range s.segments {
			if end < seg.Begin {
				newSegments = append(newSegments, seg)
				end = seg.End
			} else if end < seg.End {
				newSegments[len(newSegments)-1].End = seg.End
				end = seg.End
			}
		}
		s.segments = newSegments
	}
}

func (s *Segments) insert(segment *Segment, index int) {
	length := len(s.segments)
	s.segments = append(s.segments, s.segments[length-1])
	copy(s.segments[index+1:], s.segments[index:length])
	s.segments[index] = segment
}

// Add .
func (s *Segments) Add(segment *Segment) {
	s.CleanOverlap()
	added := false
	for i, seg := range s.segments {
		if seg.End == segment.Begin {
			seg.End = segment.End
			if i+1 < len(s.segments) && seg.End == s.segments[i+1].Begin {
				s.segments[i+1].Begin = seg.Begin
				s.segments = append(s.segments[:i], s.segments[i+1:]...)
			}
			added = true
			break
		}
		if seg.Begin == segment.End {
			seg.Begin = segment.Begin
			if i > 0 && seg.Begin == s.segments[i-1].End {
				s.segments[i-1].End = seg.End
				s.segments = append(s.segments[:i], s.segments[i+1:]...)
			}
			added = true
			break
		}
		if seg.Begin > segment.End {
			s.insert(segment, i)
			added = true
			break
		}
		if seg.End >= segment.End {
			msg := fmt.Sprintf("Unexpected segment %+v, inside %+v", segment, seg)
			logrus.Error(msg)
			panic(errors.New(msg))
		}
	}
	if !added {
		s.segments = append(s.segments, segment)
	}
}

// Sum .
func (s *Segments) Sum() int64 {
	s.CleanOverlap()
	sum := int64(0)
	for _, seg := range s.segments {
		sum += seg.End - seg.Begin
	}
	return sum
}
