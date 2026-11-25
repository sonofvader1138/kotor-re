// This file converts IMA ADPCM WAV files (format 0x11), Xbox ADPCM WAV files (format 0x69) and 8-bit PCM
// WAV files to 16-bit PCM WAV. Other formats are copied unchanged. As a check, it compares the predictor in each block header with the last
// decoded sample of the previous block.

package kotor

import (
	"encoding/binary"
	"fmt"
	"math"
)

// WAVE format tags from the fmt chunk.
// Xbox ADPCM is IMA ADPCM with 36-byte blocks per channel, but a block yields one sample fewer: the sample
// in its header and the first 63 of its 64 nibbles. The last nibble is decoded and dropped.
const (
	formatPcm       = 1
	formatImaAdpcm  = 0x11
	formatXboxAdpcm = 0x69
)

const (
	pcmHeaderSize     = 44
	pcmFmtChunkSize   = 16
	pcmBitsPerSample  = 16
	pcmBytesPerSample = pcmBitsPerSample / 8
)

var step = []int{
	7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21, 23, 25, 28, 31, 34, 37, 41, 45,
	50, 55, 60, 66, 73, 80, 88, 97, 107, 118, 130, 143, 157, 173, 190, 209, 230,
	253, 279, 307, 337, 371, 408, 449, 494, 544, 598, 658, 724, 796, 876, 963,
	1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024, 3327,
	3660, 4026, 4428, 4871, 5358, 5894, 6484, 7132, 7845, 8630, 9493, 10442,
	11487, 12635, 13899, 15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794,
	32767,
}
var stepIndexChange = []int{-1, -1, -1, -1, 2, 4, 6, 8}
var maxStepIndex = len(step) - 1

// Each block starts with one 4-byte header per channel: i16 first sample, u8 step index, u8 reserved.
// Then each channel in turn stores a 4-byte group of 8 samples, 4 bits each.
const (
	blockHeaderSize = 4
	groupSize       = 4
	groupSamples    = 8
)

// chunks maps each chunk id to its data. A later chunk with the same id replaces an earlier one.
func chunks(file []byte) map[string][]byte {
	found := map[string][]byte{}
	pos := RiffHeaderSize
	for pos+ChunkHeaderSize <= len(file) {
		chunkSize := int(binary.LittleEndian.Uint32(file[pos+4:]))
		start := pos + ChunkHeaderSize
		found[string(file[pos:pos+4])] = Slice(file, start, start+chunkSize)
		pos = start + chunkSize + chunkSize&1
	}
	return found
}

func pcmWav(pcm []int16, channels, rate, size int) []byte {
	out := make([]byte, pcmHeaderSize+size)
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(pcmHeaderSize-ChunkHeaderSize+size))
	copy(out[8:], "WAVE")

	copy(out[12:], "fmt ")
	binary.LittleEndian.PutUint32(out[16:], pcmFmtChunkSize)
	binary.LittleEndian.PutUint16(out[20:], formatPcm)
	binary.LittleEndian.PutUint16(out[22:], uint16(channels))
	binary.LittleEndian.PutUint32(out[24:], uint32(rate))
	binary.LittleEndian.PutUint32(out[28:], uint32(rate*channels*pcmBytesPerSample))
	binary.LittleEndian.PutUint16(out[32:], uint16(channels*pcmBytesPerSample))
	binary.LittleEndian.PutUint16(out[34:], pcmBitsPerSample)

	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(size))

	for i := 0; i < size/pcmBytesPerSample; i++ {
		binary.LittleEndian.PutUint16(out[pcmHeaderSize+pcmBytesPerSample*i:], uint16(pcm[i]))
	}
	return out
}

// 8-bit PCM samples are unsigned, with silence at 128.
const (
	pcm8BitsPerSample = 8
	pcm8Silence       = 128
)

// widenPcm8 converts 8-bit unsigned PCM samples to 16-bit signed samples by moving silence to 0 and
// shifting each sample into the high byte. The conversion loses nothing.
func widenPcm8(data []byte, channels, rate int) AdpcmDecoded {
	pcm := make([]int16, len(data))
	for i, sample := range data {
		pcm[i] = int16(int(sample)-pcm8Silence) << 8
	}
	info := fmt.Sprintf("widened 8-bit PCM samples=%d", len(data)/channels)
	return AdpcmDecoded{pcmWav(pcm, channels, rate, len(pcm)*pcmBytesPerSample), info, ""}
}

// Xbox ADPCM blocks hold 36 bytes per channel and yield 64 samples per channel. The fmt chunk of an Xbox
// ADPCM WAV file is 20 bytes long: the 16 bytes of every fmt chunk, then a u16 extra size of 2 and the u16
// samples per block.
const (
	xboxBlockBytes      = 36
	xboxBlockSamples    = 64
	xboxBitsPerSample   = 4
	xboxFmtChunkSize    = 20
	xboxFmtExtraSize    = 2
	xboxWavHeaderLength = RiffHeaderSize + ChunkHeaderSize + xboxFmtChunkSize + ChunkHeaderSize
)

// XboxAdpcmWav wraps raw Xbox ADPCM blocks, such as the audio packets of an XMV movie, in a WAV header of
// format 0x69, so that DecodeAdpcm can decode them.
func XboxAdpcmWav(blocks []byte, channels, rate int) []byte {
	out := make([]byte, xboxWavHeaderLength, xboxWavHeaderLength+len(blocks))
	blockAlign := xboxBlockBytes * channels
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(xboxWavHeaderLength-ChunkHeaderSize+len(blocks)))
	copy(out[8:], "WAVE")
	copy(out[12:], "fmt ")
	binary.LittleEndian.PutUint32(out[16:], xboxFmtChunkSize)
	binary.LittleEndian.PutUint16(out[20:], formatXboxAdpcm)
	binary.LittleEndian.PutUint16(out[22:], uint16(channels))
	binary.LittleEndian.PutUint32(out[24:], uint32(rate))
	binary.LittleEndian.PutUint32(out[28:], uint32(rate*blockAlign/xboxBlockSamples))
	binary.LittleEndian.PutUint16(out[32:], uint16(blockAlign))
	binary.LittleEndian.PutUint16(out[34:], xboxBitsPerSample)
	binary.LittleEndian.PutUint16(out[36:], xboxFmtExtraSize)
	binary.LittleEndian.PutUint16(out[38:], xboxBlockSamples)
	copy(out[40:], "data")
	binary.LittleEndian.PutUint32(out[44:], uint32(len(blocks)))
	return append(out, blocks...)
}

// AdpcmDecoded holds the converted file, or nil Out when Err explains why there is none.
type AdpcmDecoded struct {
	Out  []byte
	Info string
	Err  string
}

func DecodeAdpcm(file []byte) AdpcmDecoded {
	found := chunks(file)
	format, data := found["fmt "], found["data"]
	tag := int(binary.LittleEndian.Uint16(format[0:]))
	channels := int(binary.LittleEndian.Uint16(format[2:]))
	rate := int(binary.LittleEndian.Uint32(format[4:]))
	blockAlign := int(binary.LittleEndian.Uint16(format[12:]))
	if tag == formatPcm && binary.LittleEndian.Uint16(format[14:]) == pcm8BitsPerSample {
		return widenPcm8(data, channels, rate)
	}
	if tag != formatImaAdpcm && tag != formatXboxAdpcm {
		return AdpcmDecoded{file, fmt.Sprintf("copied (fmt=%d)", tag), ""}
	}

	samplesPerBlock := int(binary.LittleEndian.Uint16(format[18:]))
	groupBytes := blockAlign - blockHeaderSize*channels
	headerSamples := 1
	if tag == formatXboxAdpcm {
		headerSamples = 0
	}
	if float64(samplesPerBlock) != float64(groupBytes*(groupSamples/groupSize))/float64(channels)+float64(headerSamples) {
		return AdpcmDecoded{nil, "", fmt.Sprintf("samples/block %d vs blockalign %d", samplesPerBlock, blockAlign)}
	}

	blocks := len(data) / blockAlign
	total := blocks * samplesPerBlock
	sampleCount := total
	if fact, ok := found["fact"]; ok {
		factSamples := int(binary.LittleEndian.Uint32(fact))
		if factSamples != 0 && factSamples <= total {
			sampleCount = factSamples
		}
	}

	// The dropped last nibble of an Xbox block is written one sample past the block, where the next block's
	// header sample overwrites it, so the buffer holds one sample more.
	pcm := make([]int16, (total+1)*channels)
	errSum, errCount, maxErr, clips := 0, 0, 0, 0
	last := make([]int, channels)
	hasLast := false
	pred := make([]int, channels)
	index := make([]int, channels)

	for block := range blocks {
		blockStart := block * blockAlign
		firstSample := block * samplesPerBlock

		for channel := range channels {
			channelHeader := blockStart + blockHeaderSize*channel
			pred[channel] = int(int16(binary.LittleEndian.Uint16(data[channelHeader:])))
			index[channel] = int(data[channelHeader+2])
			if index[channel] > maxStepIndex {
				return AdpcmDecoded{nil, "", fmt.Sprintf("bad step index %d block %d", index[channel], block)}
			}

			if hasLast {
				gap := pred[channel] - last[channel]
				if gap < 0 {
					gap = -gap
				}
				errSum += gap
				errCount++
				maxErr = max(maxErr, gap)
			}

			pcm[firstSample*channels+channel] = int16(pred[channel])
		}

		// Within a group, the low nibble of each byte comes first.
		pos := blockStart + blockHeaderSize*channels
		for sample := 1; pos < blockStart+blockAlign; sample += groupSamples {
			for channel := range channels {
				for j := range groupSamples {
					b := int(data[pos+j>>1])
					nibble := b & 15
					if j&1 != 0 {
						nibble = b >> 4
					}
					stepSize := step[index[channel]]

					diff := stepSize >> 3
					if nibble&1 != 0 {
						diff += stepSize >> 2
					}
					if nibble&2 != 0 {
						diff += stepSize >> 1
					}
					if nibble&4 != 0 {
						diff += stepSize
					}
					if nibble&8 != 0 {
						diff = -diff
					}

					value := pred[channel] + diff
					if value > math.MaxInt16 {
						value = math.MaxInt16
						clips++
					} else if value < math.MinInt16 {
						value = math.MinInt16
						clips++
					}

					pred[channel] = value
					index[channel] = min(maxStepIndex, max(0, index[channel]+stepIndexChange[nibble&7]))
					pcm[(firstSample+sample+j)*channels+channel] = int16(value)
				}
				pos += groupSize
			}
		}

		copy(last, pred)
		hasLast = true
	}

	partial := len(data) - blocks*blockAlign
	err := ""
	if partial != 0 {
		err = "partial block"
	}
	info := fmt.Sprintf("fmt=%d blocks=%d partial=%d samples=%d boundary_err avg=%.2f max=%d clips=%d",
		tag, blocks, partial, sampleCount, float64(errSum)/float64(errCount), maxErr, clips)
	return AdpcmDecoded{pcmWav(pcm, channels, rate, sampleCount*channels*pcmBytesPerSample), info, err}
}
