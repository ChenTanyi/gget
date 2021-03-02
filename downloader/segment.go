package downloader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	ErrWrongSegmentFormat = errors.New("Wrong Segment Format")
	ErrInvalidJobId       = errors.New("Invalid Job Id")
	ErrInvalidSize        = errors.New("Invalid Size")
	ErrSegmentStarted     = errors.New("Segment Already Started")
	ErrSegmentNotActive   = errors.New("Segment Not Active")
	ErrSegmentFinish      = errors.New("Segment Finish")
	ErrAllSegmentIsFinish = errors.New("All Segment Is Finish")
	ErrSendLimit          = errors.New("Send Limit Error")
)

// Segment .
type Segment struct {
	jobid    int
	begin    int64
	end      int64
	position int64
	dst      io.WriterAt
}

// Segments .
type Segments struct {
	segments []*Segment
}

// NewSegment .
func NewSegment(begin, end int64) *Segment {
	return &Segment{begin: begin, position: begin, end: end}
}

// SegmentFromString .
func SegmentFromString(s string) (*Segment, error) {
	points := strings.Split(strings.TrimSpace(s), "-")
	if len(points) != 3 {
		return nil, ErrWrongSegmentFormat
	}
	begin, err := strconv.ParseInt(points[0], 0, 64)
	if err != nil {
		return nil, fmt.Errorf("Parse segment error: segment = %s, err = %v", s, err)
	}
	position, err := strconv.ParseInt(points[1], 0, 64)
	if err != nil {
		return nil, fmt.Errorf("Parse segment error: segment = %s, err = %v", s, err)
	}
	end, err := strconv.ParseInt(points[2], 0, 64)
	if err != nil {
		return nil, fmt.Errorf("Parse segment error: segment = %s, err = %v", s, err)
	}
	return &Segment{
		begin:    begin,
		position: position,
		end:      end,
	}, nil
}

// Write .
func (s *Segment) Write(b []byte) (size int, err error) {
	if s.Finish() {
		return 0, ErrSegmentFinish
	}
	if !s.Active() {
		logrus.Debugf("segment[inactive] %s add %d", s, size)
		return 0, ErrSegmentNotActive
	}

	size = len(b)
	if size > int(s.Remaining()) {
		logrus.Debugf("segment %s add %d too longer", s, size)
		size = int(s.Remaining())
	}
	size, err = s.dst.WriteAt(b[:size], s.position)
	s.position += int64(size)
	return size, err
}

func (s *Segment) String() string {
	return fmt.Sprintf("%d-%d-%d", s.begin, s.position, s.end)
}

// Readable .
func (s *Segment) Readable() string {
	return fmt.Sprintf("%d: %s - %s(%s)", s.jobid, SizeToReadable(float64(s.begin)),
		SizeToReadable(float64(s.position)), SizeToReadable(float64(s.end)))
}

// Begin .
func (s *Segment) Begin() int64 {
	return s.begin
}

// Current .
func (s *Segment) Current() int64 {
	return s.position
}

// End .
func (s *Segment) End() int64 {
	return s.end
}

// Length .
func (s *Segment) Length() int64 {
	return s.end - s.begin
}

// Remaining .
func (s *Segment) Remaining() int64 {
	return s.end - s.position
}

// Written .
func (s *Segment) Written() int64 {
	return s.position - s.begin
}

// Finish .
func (s *Segment) Finish() bool {
	return s.position >= s.end
}

// Active .
func (s *Segment) Active() bool {
	return !s.Finish() && s.jobid != 0
}

// Start .
func (s *Segment) Start(jobid int, dst io.WriterAt) error {
	if s.JobId() > 0 {
		logrus.Debugf("segment %s already start", s.Readable())
		return ErrSegmentStarted
	}
	if jobid <= 0 {
		logrus.Debugf("segment %s start invalid job id %d", s.Readable(), jobid)
		return ErrInvalidJobId
	}
	s.jobid = jobid
	s.dst = dst
	logrus.Infof("segment %s start", s.Readable())
	return nil
}

// JobId .
func (s *Segment) JobId() int {
	return s.jobid
}

// Split .
func (s *Segment) Split() *Segment {
	mid := s.position + (s.end-s.position)/2
	seg := NewSegment(mid, s.end)
	s.end = mid
	return seg
}

// Cross .
func (s *Segment) Cross(seg *Segment) bool {
	return s.begin >= seg.begin && s.begin < seg.end || s.end <= seg.end && s.end > seg.begin
}

// Connected .
func (s *Segment) Connected(seg *Segment) bool {
	return s.begin == seg.end || s.end == seg.begin
}

// Contain .
func (s *Segment) Contain(seg *Segment) bool {
	return s.begin <= seg.begin && seg.end <= s.end
}

// Merge .
func (s *Segment) Merge(seg *Segment) {
	if s.Finish() && seg.Finish() {
		if s.Connected(seg) || s.Cross(seg) || s.Contain(seg) || seg.Contain(s) {
			if s.begin > seg.begin {
				s.begin = seg.begin
			}
			if s.end < seg.end {
				s.end = seg.end
			}
			s.position = s.end
		} else {
			panic(fmt.Errorf("Connot merge uncross segment, %s + %s", s.Readable(), seg.Readable()))
		}
	} else {
		panic(fmt.Errorf("Cannot merge unfinish segment, %s + %s", s.Readable(), seg.Readable()))
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
	for _, segmentStr := range segmentStrs {
		segmentStr = strings.TrimSpace(segmentStr)
		if segmentStr == "" {
			continue
		}

		seg, err := SegmentFromString(segmentStr)
		if err != nil {
			return nil, err
		}

		s.segments = append(s.segments, seg)
	}
	return s, nil
}

// Segments .
func (s *Segments) Segments() []*Segment {
	return s.segments
}

// CleanOverlap .
func (s *Segments) CleanOverlap() {
	newSegments := make([]*Segment, 0, len(s.segments))
	sort.Slice(s.segments, func(i, j int) bool {
		if s.segments[i].Begin() == s.segments[j].Begin() {
			return s.segments[i].End() > s.segments[j].End()
		}
		return s.segments[i].Begin() < s.segments[j].Begin()
	})

	finish := NewSegment(0, 0)
	for _, seg := range s.segments {
		if seg.Finish() {
			if seg.Begin() <= finish.End() {
				if seg.End() > finish.End() {
					finish.end = seg.End()
					finish.position = seg.End()
				}
			} else {
				if finish.End() > finish.Begin() {
					newSegments = append(newSegments, finish)
				}
				finish = seg
			}
			continue
		}
		newSegments = append(newSegments, seg)
	}
	if finish.End() > finish.Begin() {
		newSegments = append(newSegments, finish)
	}
	s.segments = newSegments
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

func (s *Segments) String() string {
	return string(s.ToByte())
}

// Write .
func (s *Segments) Write(jobid int, b []byte) (int, error) {
	if jobid <= 0 {
		return 0, ErrInvalidJobId
	}
	for _, seg := range s.segments {
		if seg.JobId() == jobid && !seg.Finish() {
			return seg.Write(b)
		}
	}
	logrus.Debugf("Can't' write to job %d with size %d, segments: %s", jobid, len(b), s.Readable())
	return 0, ErrAllSegmentIsFinish
}

// Remove .
func (s *Segments) Remove(begin, end int64) {
	s.segments = append(s.segments, NewSegment(begin, end))
}

// Start .
func (s *Segments) Start(jobId int, dst io.WriterAt) (*Segment, error) {
	if jobId <= 0 {
		return nil, ErrInvalidJobId
	}
	maxSize := int64(0)
	index := -1
	for i, seg := range s.segments {
		if !seg.Finish() {
			if seg.JobId() == jobId {
				return nil, nil
			}
			if seg.JobId() == 0 {
				seg.Start(jobId, dst)
				return seg, nil
			}
			if seg.Length() > maxSize {
				maxSize = seg.Remaining()
				index = i
			}
		}
	}
	if maxSize >= 2*MinimalSegment {
		seg := s.segments[index].Split()
		seg.Start(jobId, dst)
		s.segments = append(s.segments, seg)
		return seg, nil
	}
	return nil, ErrAllSegmentIsFinish
}

// InitSize .
func (s *Segments) InitSize(length int64) {
	if len(s.segments) == 0 {
		s.segments = append(s.segments, NewSegment(0, length))
	}
}

// Remaining .
func (s *Segments) Remaining() int64 {
	sum := int64(0)
	for _, seg := range s.segments {
		sum += seg.Remaining()
	}
	return sum
}
