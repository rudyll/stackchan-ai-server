/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"encoding/binary"
	"fmt"

	opus "gopkg.in/hraban/opus.v2"
)

const (
	deviceSampleRate   = 16000
	serverSampleRate   = 24000
	frameChannels      = 1
	frameDurationMs    = 60
	deviceFrameSamples = deviceSampleRate * frameDurationMs / 1000 // 960
	serverFrameSamples = serverSampleRate * frameDurationMs / 1000 // 1440
)

// decodeFramesToPCM decodes a sequence of OPUS frames (16kHz mono) to concatenated int16 PCM.
func decodeFramesToPCM(frames [][]byte) ([]int16, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames")
	}
	dec, err := opus.NewDecoder(deviceSampleRate, frameChannels)
	if err != nil {
		return nil, err
	}
	pcmBuf := make([]int16, deviceFrameSamples*4)
	var all []int16
	for _, frame := range frames {
		n, err := dec.Decode(frame, pcmBuf)
		if err != nil {
			return nil, fmt.Errorf("opus decode: %w", err)
		}
		all = append(all, pcmBuf[:n]...)
	}
	return all, nil
}

// encodeOpusFrames encodes 24kHz mono int16 PCM into 60ms OPUS frames for the device.
func encodeOpusFrames(pcm []int16) ([][]byte, error) {
	enc, err := opus.NewEncoder(serverSampleRate, frameChannels, opus.AppVoIP)
	if err != nil {
		return nil, err
	}
	outBuf := make([]byte, 4096)
	var frames [][]byte
	for i := 0; i+serverFrameSamples <= len(pcm); i += serverFrameSamples {
		n, err := enc.Encode(pcm[i:i+serverFrameSamples], outBuf)
		if err != nil {
			return nil, fmt.Errorf("opus encode at %d: %w", i, err)
		}
		frame := make([]byte, n)
		copy(frame, outBuf[:n])
		frames = append(frames, frame)
	}
	return frames, nil
}

// newOpusDecoder creates a reusable 16kHz mono decoder for streaming device input.
func newOpusDecoder() (*opus.Decoder, error) {
	return opus.NewDecoder(deviceSampleRate, frameChannels)
}

// decodeOpusFrame decodes one OPUS frame, preserving state across consecutive calls on the same decoder.
func decodeOpusFrame(dec *opus.Decoder, frame []byte) ([]int16, error) {
	pcmBuf := make([]int16, deviceFrameSamples*4)
	n, err := dec.Decode(frame, pcmBuf)
	if err != nil {
		return nil, err
	}
	return pcmBuf[:n], nil
}

// opusStreamEncoder buffers 24kHz PCM and emits complete 60ms OPUS frames on Encode calls.
type opusStreamEncoder struct {
	enc *opus.Encoder
	buf []int16
}

func newOpusStreamEncoder() (*opusStreamEncoder, error) {
	enc, err := opus.NewEncoder(serverSampleRate, frameChannels, opus.AppVoIP)
	if err != nil {
		return nil, err
	}
	return &opusStreamEncoder{enc: enc}, nil
}

// Flush encodes any remaining buffered samples as a zero-padded 60ms frame.
// Call this before discarding the encoder so the audio tail is not silently dropped.
func (e *opusStreamEncoder) Flush() [][]byte {
	if len(e.buf) == 0 {
		return nil
	}
	// Pad the tail with silence to reach a full 60ms frame.
	padded := make([]int16, serverFrameSamples)
	copy(padded, e.buf)
	e.buf = e.buf[:0]
	outBuf := make([]byte, 4096)
	n, err := e.enc.Encode(padded, outBuf)
	if err != nil {
		return nil
	}
	frame := make([]byte, n)
	copy(frame, outBuf[:n])
	return [][]byte{frame}
}

// Encode appends pcm to the internal buffer and returns any complete 60ms frames ready to send.
func (e *opusStreamEncoder) Encode(pcm []int16) ([][]byte, error) {
	e.buf = append(e.buf, pcm...)
	outBuf := make([]byte, 4096)
	var frames [][]byte
	for len(e.buf) >= serverFrameSamples {
		n, err := e.enc.Encode(e.buf[:serverFrameSamples], outBuf)
		if err != nil {
			return frames, fmt.Errorf("opus encode: %w", err)
		}
		frame := make([]byte, n)
		copy(frame, outBuf[:n])
		frames = append(frames, frame)
		e.buf = e.buf[serverFrameSamples:]
	}
	return frames, nil
}

// pcmToWAV wraps int16 PCM in a WAV container for Whisper upload.
func pcmToWAV(pcm []int16, sampleRate int) []byte {
	dataSize := len(pcm) * 2
	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:], 1) // mono
	binary.LittleEndian.PutUint32(buf[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(buf[32:], 2)  // block align
	binary.LittleEndian.PutUint16(buf[34:], 16) // bits per sample
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataSize))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(buf[44+i*2:], uint16(s))
	}
	return buf
}
