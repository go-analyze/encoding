package base85

import (
	"bytes"
	"encoding/ascii85"
	"io"
	"testing"
)

var benchData = []byte("The quick brown fox jumps over the lazy dog. 0123456789!@#$%^&*()")

func BenchmarkEncodeBase85(b *testing.B) {
	dst := make([]byte, RFC1924.EncodedLen(len(benchData)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		RFC1924.Encode(dst, benchData)
	}
}

func BenchmarkDecodeBase85(b *testing.B) {
	encoded := RFC1924.EncodeToString(benchData)
	src := []byte(encoded)
	dst := make([]byte, RFC1924.DecodedLen(len(src)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = RFC1924.Decode(dst, src)
	}
}

func BenchmarkEncodeAscii85(b *testing.B) {
	dst := make([]byte, ascii85.MaxEncodedLen(len(benchData)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ascii85.Encode(dst, benchData)
	}
}

func BenchmarkStreamEncodeBase85(b *testing.B) {
	var payload []byte
	for i := 0; i < 64; i++ {
		payload = append(payload, benchData...)
	}
	var sink bytes.Buffer
	sink.Grow(RFC1924.EncodedLen(len(payload)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink.Reset()
		w := NewEncoder(RFC1924, &sink)
		_, _ = w.Write(payload)
		_ = w.Close()
	}
}

func BenchmarkStreamDecodeBase85(b *testing.B) {
	// build a larger payload so a stream Read covers many blocks
	var payload []byte
	for i := 0; i < 64; i++ {
		payload = append(payload, benchData...)
	}
	encoded := RFC1924.EncodeToString(payload)
	src := []byte(encoded)
	dst := make([]byte, len(payload))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := NewDecoder(RFC1924, bytes.NewReader(src))
		_, _ = io.ReadFull(r, dst)
	}
}

func BenchmarkDecodeAscii85(b *testing.B) {
	src := make([]byte, ascii85.MaxEncodedLen(len(benchData)))
	n := ascii85.Encode(src, benchData)
	src = src[:n]
	dst := make([]byte, len(benchData))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = ascii85.Decode(dst, src, true)
	}
}
