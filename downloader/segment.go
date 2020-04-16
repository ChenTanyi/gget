package downloader

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
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

// Readable .
func (s *Segment) Readable() string {
	return fmt.Sprintf("%s - %s", SizeToReadable(float64(s.Begin)), SizeToReadable(float64(s.End)))
}

// Length .
func (s *Segment) Length() int64 {
	return s.End - s.Begin
}

// Finish .
func (s *Segment) Finish() bool {
	return s.Begin >= s.End
}

// Cross .
func (s *Segment) Cross(seg *Segment) bool {
	return s.Begin >= seg.Begin && s.Begin < seg.End || s.End <= seg.End && s.End > seg.Begin
}

// Connected .
func (s *Segment) Connected(seg *Segment) bool {
	return s.Begin == seg.End || s.End == seg.Begin
}

// Merge .
func (s *Segment) Merge(seg *Segment) {
	if seg.Begin < s.Begin {
		s.Begin = seg.Begin
	}
	if seg.End > s.End {
		s.End = seg.End
	}
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
	s.CleanOverlap()
	buffer := bytes.NewBuffer(nil)
	for index, segment := range s.segments {
		if index > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(segment.String())
	}
	return buffer.Bytes()
}

// Readable .
func (s *Segments) Readable() string {
	buffer := bytes.NewBuffer(nil)
	for index, segment := range s.segments {
		if index > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(segment.Readable())
	}
	return buffer.String()
}

// IsOverlaped .
func (s *Segments) IsOverlaped() bool {
	var end int64 = -1
	for _, seg := range s.segments {
		if seg.Begin <= end {
			return false
		}
		end = seg.End
	}
	return true
}

// CleanOverlap .
func (s *Segments) CleanOverlap() {
	s.Sort()
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

// Sort .
func (s *Segments) Sort() {
	sort.Slice(s.segments, func(i, j int) bool {
		if s.segments[i].Begin != s.segments[j].Begin {
			return s.segments[i].Begin < s.segments[i].Begin
		}
		return s.segments[i].End < s.segments[j].End
	})
}

func (s *Segments) insert(segment *Segment, index int) {
	length := len(s.segments)
	s.segments = append(s.segments, s.segments[length-1])
	copy(s.segments[index+1:], s.segments[index:length])
	s.segments[index] = segment
}

// Add .
func (s *Segments) Add(segment *Segment) {
	added := false
	for _, seg := range s.segments {
		if seg.Connected(segment) || seg.Cross(segment) {
			seg.Merge(segment)
			added = true
			break
		}
	}
	if !added {
		s.segments = append(s.segments, segment)
	}
	s.CleanOverlap()
}

// Remove .
func (s *Segments) Remove(segment *Segment) {
	newSegments := make([]*Segment, 0, len(s.segments))
	for _, seg := range s.segments {
		if seg.End <= segment.Begin || seg.Begin >= segment.End {
			newSegments = append(newSegments, seg)
		} else if seg.Begin < segment.Begin {
			if seg.End <= segment.End {
				seg.End = segment.Begin
				newSegments = append(newSegments, seg)
			} else {
				newSegments = append(newSegments, &Segment{Begin: seg.Begin, End: segment.Begin}, &Segment{Begin: segment.End, End: seg.End})
			}
		} else if seg.End > segment.End {
			seg.Begin = segment.End
			newSegments = append(newSegments, seg)
		}
	}
	s.segments = newSegments
	s.CleanOverlap()
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
