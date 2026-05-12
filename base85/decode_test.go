package base85

import (
	"bytes"
	"encoding/ascii85"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	// test cases avoid 4 consecutive zero bytes (which ascii85.Encode outputs as 'z')
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"single_byte", []byte{0x01}},
		{"two_bytes", []byte{0x01, 0x02}},
		{"three_bytes", []byte{0x01, 0x02, 0x03}},
		{"four_bytes", []byte{0x01, 0x02, 0x03, 0x04}},
		{"five_bytes", []byte{0x01, 0x02, 0x03, 0x04, 0x05}},
		{"eight_bytes", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}},
		{"hello_world", []byte("Hello, World!")},
		{"binary_data", []byte{0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA}},
		{"high_values", []byte{0xFF, 0xFF, 0xFF, 0xFF}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// encode with standard ascii85
			encoded := make([]byte, ascii85.MaxEncodedLen(len(tc.input)))
			n := ascii85.Encode(encoded, tc.input)
			encoded = encoded[:n]

			// decode with our implementation
			dst := make([]byte, ascii85Encoding.DecodedLen(len(encoded)))
			ndst, err := ascii85Encoding.Decode(dst, encoded)

			require.NoError(t, err)
			assert.Equal(t, tc.input, dst[:ndst])
		})
	}
}

func TestDecodeString(t *testing.T) {
	t.Parallel()

	// bytes for "日本語" without triggering gosmopolitan
	unicodeBytes := []byte{0xe6, 0x97, 0xa5, 0xe6, 0x9c, 0xac, 0xe8, 0xaa, 0x9e}

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"simple_text", []byte("test")},
		{"unicode", unicodeBytes},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// encode with standard ascii85
			encoded := make([]byte, ascii85.MaxEncodedLen(len(tc.input)))
			n := ascii85.Encode(encoded, tc.input)
			encodedStr := string(encoded[:n])

			// decode with our implementation
			decoded, err := ascii85Encoding.DecodeString(encodedStr)

			require.NoError(t, err)
			assert.Equal(t, tc.input, decoded)
		})
	}
}

func TestAppendDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix []byte
		input  []byte
	}{
		{"empty_prefix", nil, []byte("test")},
		{"with_prefix", []byte("decoded:"), []byte("data")},
		{"empty_input", []byte("prefix:"), []byte{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// encode with standard ascii85
			encoded := make([]byte, ascii85.MaxEncodedLen(len(tc.input)))
			n := ascii85.Encode(encoded, tc.input)
			encoded = encoded[:n]

			// decode with our implementation
			result, err := ascii85Encoding.AppendDecode(tc.prefix, encoded)

			require.NoError(t, err)

			// verify prefix is preserved
			if tc.prefix != nil {
				assert.Equal(t, string(tc.prefix), string(result[:len(tc.prefix)]))
			}

			// verify decoded content
			decodedPortion := result[len(tc.prefix):]
			assert.Equal(t, tc.input, decodedPortion)
		})
	}
}

func TestDecodeCorruptInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		enc   *Encoding
		input string
	}{
		{"invalid_char", ascii85Encoding, "hello\x00world"},
		{"out_of_alphabet", ascii85Encoding, "~~~~~"},
		{"single_trailing_char", ascii85Encoding, "AAAAA0"}, // 5 valid chars + 1 trailing (invalid)
		// overflow: in-alphabet values whose decoded uint32 would exceed MaxUint32
		{"overflow_full_block", ascii85Encoding, "uuuuu"}, // 84*85^4 + ... + 84 > 2^32-1
		{"overflow_partial_4", ascii85Encoding, "uuuu"},   // partial block, implicit 84 padding overflows
		{"overflow_partial_3", ascii85Encoding, "uuu"},
		{"overflow_partial_2", ascii85Encoding, "uu"},
		{"overflow_rfc1924", RFC1924, "~~~~~"}, // `~` is the highest digit (84) in RFC1924
		// with padding enabled, unpadded trailing blocks are invalid (must use NoPadding for lenient decode)
		{"padded_unpadded_2", RFC1924.WithPadding('.'), "00"},
		{"padded_unpadded_3", RFC1924.WithPadding('.'), "000"},
		{"padded_unpadded_4", RFC1924.WithPadding('.'), "0000"},
		{"padded_full_plus_unpadded", RFC1924.WithPadding('.'), "0000000"}, // 5 valid + 2 unpadded
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.enc.DecodeString(tc.input)
			require.Error(t, err)

			var corruptErr CorruptInputError
			assert.ErrorAs(t, err, &corruptErr)
		})
	}
}

func TestDecoderOverflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		enc   *Encoding
		input string
	}{
		{"full_block", ascii85Encoding, "uuuuu"},
		{"partial_tail", ascii85Encoding, "AAAAAuuuu"}, // valid block then overflow partial
		{"rfc1924", RFC1924, "~~~~~"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := NewDecoder(tc.enc, bytes.NewReader([]byte(tc.input)))
			_, err := io.ReadAll(dec)
			require.Error(t, err)

			var corruptErr CorruptInputError
			assert.ErrorAs(t, err, &corruptErr)
		})
	}
}

func TestDecoderErrorOffset(t *testing.T) {
	t.Parallel()

	// 10 valid chars then an invalid byte; one-byte reader forces chunk boundaries
	input := "AAAAAAAAAA\x00"
	dec := NewDecoder(ascii85Encoding, iotest.OneByteReader(bytes.NewReader([]byte(input))))
	_, err := io.ReadAll(dec)

	var corruptErr CorruptInputError
	require.ErrorAs(t, err, &corruptErr)
	assert.Equal(t, int64(10), int64(corruptErr))
}

type noProgressReader struct{}

func (noProgressReader) Read(p []byte) (int, error) { return 0, nil }

func TestDecoderNoProgress(t *testing.T) {
	t.Parallel()

	dec := NewDecoder(ascii85Encoding, noProgressReader{})
	_, err := dec.Read(make([]byte, 16))
	assert.ErrorIs(t, err, io.ErrNoProgress)
}

func TestDecodeWhitespace(t *testing.T) {
	t.Parallel()

	input := []byte("test")

	// encode with standard ascii85
	encoded := make([]byte, ascii85.MaxEncodedLen(len(input)))
	n := ascii85.Encode(encoded, input)
	encodedStr := string(encoded[:n])

	// insert whitespace
	withWhitespace := encodedStr[:2] + " \t\n\r" + encodedStr[2:]

	// decode should ignore whitespace
	decoded, err := ascii85Encoding.DecodeString(withWhitespace)

	require.NoError(t, err)
	assert.Equal(t, input, decoded)
}

func TestNewDecoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		readSize int // buffer size for Read calls, 0 means read all at once
	}{
		{"empty", []byte{}, 0},
		{"single_byte", []byte{0x01}, 0},
		{"four_bytes", []byte{0x01, 0x02, 0x03, 0x04}, 0},
		{"five_bytes", []byte{0x01, 0x02, 0x03, 0x04, 0x05}, 0},
		{"hello_world", []byte("Hello, World!"), 0},
		{"small_reads", []byte("Hello, World!"), 3},
		{"byte_by_byte", []byte("test"), 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// encode with standard ascii85
			encoded := make([]byte, ascii85.MaxEncodedLen(len(tc.input)))
			n := ascii85.Encode(encoded, tc.input)
			encoded = encoded[:n]

			// decode with our stream decoder
			dec := NewDecoder(ascii85Encoding, bytes.NewReader(encoded))

			var decoded []byte
			if tc.readSize == 0 {
				// read all at once
				var err error
				decoded, err = io.ReadAll(dec)
				require.NoError(t, err)
			} else {
				// read in chunks
				buf := make([]byte, tc.readSize)
				for {
					nr, err := dec.Read(buf)
					if nr > 0 {
						decoded = append(decoded, buf[:nr]...)
					}
					if err == io.EOF {
						break
					}
					require.NoError(t, err)
				}
			}

			assert.Equal(t, tc.input, decoded)
		})
	}
}

func TestNewDecoderWithWhitespace(t *testing.T) {
	t.Parallel()

	input := []byte("Hello, World!")

	// encode with standard ascii85
	encoded := make([]byte, ascii85.MaxEncodedLen(len(input)))
	n := ascii85.Encode(encoded, input)
	encodedStr := string(encoded[:n])

	// insert whitespace
	withWhitespace := encodedStr[:5] + "\n\t " + encodedStr[5:]

	// decode with stream decoder
	dec := NewDecoder(ascii85Encoding, bytes.NewReader([]byte(withWhitespace)))
	decoded, err := io.ReadAll(dec)

	require.NoError(t, err)
	assert.Equal(t, input, decoded)
}

func TestNewDecoderPaddedRequiresPadding(t *testing.T) {
	t.Parallel()

	paddedEnc := RFC1924.WithPadding('.')

	tests := []struct {
		name  string
		input string
	}{
		{"unpadded_2", "00"},
		{"unpadded_3", "000"},
		{"unpadded_4", "0000"},
		{"full_plus_unpadded", "0000000"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := NewDecoder(paddedEnc, bytes.NewReader([]byte(tc.input)))
			_, err := io.ReadAll(dec)
			require.Error(t, err)

			var corruptErr CorruptInputError
			assert.ErrorAs(t, err, &corruptErr)
		})
	}
}
