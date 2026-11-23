// LIP V1.0: lip sync keyframes for one voice file.
// 16-byte header: char[8] signature, f32 length in seconds, u32 keyframe count n.
// Then n keyframes of 5 bytes each, unaligned: f32 time in seconds, u8 mouth shape.
package kotor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	lipSignature    = "LIP V1.0"
	LipHeaderSize   = 16
	lipKeyframeSize = 5
	lipShapeCount   = 16
)

// LipCsvHeader is the first line of every converted LIP file.
const LipCsvHeader = "time_s,shape"

type LipKeyframe struct {
	Time  float32
	Shape uint8
}

type Lip struct {
	Length    float32
	Keyframes []LipKeyframe
	Errors    []string
}

func f32(data []byte, offset int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
}

// ParseLip reads a LIP resource. It requires the keyframes to end exactly at the end of the data, times
// to be nondecreasing, shapes to be below 16, and the length to equal the last keyframe's time, so that
// the keyframes alone carry every value of the resource.
func ParseLip(data []byte) *Lip {
	lip := &Lip{}
	if len(data) < LipHeaderSize || string(data[:signatureLength]) != lipSignature {
		lip.Errors = append(lip.Errors, "not LIP V1.0")
		return lip
	}

	lip.Length = f32(data, 8)
	count := int(u32(data, 12))
	if LipHeaderSize+lipKeyframeSize*count != len(data) {
		lip.Errors = append(lip.Errors, fmt.Sprintf("%d keyframes need %d bytes, resource has %d",
			count, LipHeaderSize+lipKeyframeSize*count, len(data)))
		return lip
	}

	for i := range count {
		offset := LipHeaderSize + lipKeyframeSize*i
		keyframe := LipKeyframe{Time: f32(data, offset), Shape: data[offset+4]}

		if keyframe.Shape >= lipShapeCount {
			lip.Errors = append(lip.Errors, fmt.Sprintf("keyframe %d has shape %d", i, keyframe.Shape))
		}
		if i > 0 && keyframe.Time < lip.Keyframes[i-1].Time {
			lip.Errors = append(lip.Errors, fmt.Sprintf("keyframe %d goes back in time", i))
		}

		lip.Keyframes = append(lip.Keyframes, keyframe)
	}

	if count == 0 || lip.Keyframes[count-1].Time != lip.Length {
		lip.Errors = append(lip.Errors, "length differs from the last keyframe time")
	}
	return lip
}

// LipCsv writes the keyframes as CSV, one keyframe per line. Each time is the shortest decimal that
// reads back as the same f32, so the conversion is lossless.
func LipCsv(lip *Lip) []byte {
	var text bytes.Buffer
	fmt.Fprintf(&text, "%s\n", LipCsvHeader)

	for _, keyframe := range lip.Keyframes {
		fmt.Fprintf(&text, "%g,%d\n", keyframe.Time, keyframe.Shape)
	}
	return text.Bytes()
}

func ReportLip(label string, lip *Lip) string {
	return fmt.Sprintf("%s keyframes=%d length=%gs %s", label, len(lip.Keyframes), lip.Length, Verdict(lip.Errors...))
}
