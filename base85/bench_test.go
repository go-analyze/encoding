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

// laceWhitespace inserts a space after every 8th char to exercise the stream filter.
func laceWhitespace(encoded []byte) []byte {
	out := make([]byte, 0, len(encoded)+len(encoded)/8+1)
	for i, c := range encoded {
		if i > 0 && i%8 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	return out
}

// BenchmarkStreamDecodeBase85Matrix measures stream decode across stream size,
// consumer read size, and encoding variant.
func BenchmarkStreamDecodeBase85Matrix(b *testing.B) {
	paddedEnc := RFC1924.WithPadding('.')

	sizes := []struct {
		name string
		n    int
	}{
		{"4KB", 4 * 1024},
		{"64KB", 64 * 1024},
		{"1MB", 1 << 20},
	}
	readSizes := []struct {
		name string
		n    int // 0 decodes via io.ReadAll
	}{
		{"read_16B", 16},
		{"read_1KB", 1024},
		{"read_32KB", 32 * 1024},
		{"read_all", 0},
	}

	for _, size := range sizes {
		input := make([]byte, size.n)
		for i := range input {
			input[i] = byte(i)
		}

		// padded stream built from 3-byte blocks so every block carries padding
		paddedStream := make([]byte, 0, (size.n/3+1)*5)
		for i := 0; i < size.n; i += 3 {
			end := i + 3
			if end > size.n {
				end = size.n
			}
			paddedStream = paddedEnc.AppendEncode(paddedStream, input[i:end])
		}

		cases := []struct {
			name    string
			enc     *Encoding
			encoded []byte
		}{
			{"plain", RFC1924, []byte(RFC1924.EncodeToString(input))},
			{"whitespace", RFC1924, laceWhitespace([]byte(RFC1924.EncodeToString(input)))},
			{"padded", paddedEnc, paddedStream},
		}

		for _, rs := range readSizes {
			for _, tc := range cases {
				b.Run(size.name+"/"+rs.name+"/"+tc.name, func(b *testing.B) {
					b.SetBytes(int64(size.n))
					b.ReportAllocs()

					for i := 0; i < b.N; i++ {
						dec := NewDecoder(tc.enc, bytes.NewReader(tc.encoded))
						total := 0
						if rs.n == 0 {
							decoded, err := io.ReadAll(dec)
							total = len(decoded)
							if err != nil {
								b.Fatal(err)
							}
						} else {
							buf := make([]byte, rs.n)
							for {
								n, err := dec.Read(buf)
								total += n
								if err == io.EOF {
									break
								}
								if err != nil {
									b.Fatal(err)
								}
							}
						}
						if total != size.n {
							b.Fatalf("decoded %d bytes, want %d", total, size.n)
						}
					}
				})
			}
		}
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
