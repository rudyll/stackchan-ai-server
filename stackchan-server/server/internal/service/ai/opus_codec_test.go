package ai

import (
	"math"
	"testing"
)

// Also run against each statically packaged macOS codec before distribution.
func TestOpusCodecRoundTrip(t *testing.T) {
	pcm := make([]int16, serverFrameSamples*3)
	for i := range pcm {
		pcm[i] = int16(8000 * math.Sin(2*math.Pi*440*float64(i)/serverSampleRate))
	}
	frames, err := encodeOpusFrames(pcm)
	if err != nil || len(frames) != 3 {
		t.Fatalf("encode: %d frames, %v", len(frames), err)
	}
	decoded, err := decodeFramesToPCM(frames)
	if err != nil || len(decoded) != deviceFrameSamples*3 {
		t.Fatalf("decode: %d samples, %v", len(decoded), err)
	}
	var energy int64
	for _, sample := range decoded {
		energy += int64(sample) * int64(sample)
	}
	if energy == 0 {
		t.Fatal("decoded test tone is silent")
	}
	if _, err := decodeFramesToPCM(nil); err == nil {
		t.Fatal("empty frame list must be rejected")
	}
}
