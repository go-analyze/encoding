package base85

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRFC1924RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"single_byte", []byte{0x00}},
		{"single_nonzero", []byte{0x42}},
		{"two_bytes", []byte{0x01, 0x02}},
		{"three_bytes", []byte{0x01, 0x02, 0x03}},
		{"four_bytes", []byte{0x01, 0x02, 0x03, 0x04}},
		{"five_bytes", []byte{0x01, 0x02, 0x03, 0x04, 0x05}},
		{"eight_bytes", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}},
		{"hello_world", []byte("Hello, World!")},
		{"all_zeros", []byte{0x00, 0x00, 0x00, 0x00}},
		{"all_ones", []byte{0xFF, 0xFF, 0xFF, 0xFF}},
		{"binary_sequence", []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}},
		{"unicode_text", []byte{0xe6, 0x97, 0xa5, 0xe6, 0x9c, 0xac, 0xe8, 0xaa, 0x9e}},
		{"mixed_content", []byte("Test\x00\x01\x02\xFF\xFEdata")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := RFC1924.EncodeToString(tc.input)
			decoded, err := RFC1924.DecodeString(encoded)

			require.NoError(t, err)
			assert.Equal(t, tc.input, decoded)
		})
	}
}

func TestRFC1924RoundTripWithPadding(t *testing.T) {
	t.Parallel()

	// use '.' since '=' is in the RFC1924 alphabet
	paddedRFC1924 := RFC1924.WithPadding('.')

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"single_byte", []byte{0x42}},
		{"two_bytes", []byte{0x01, 0x02}},
		{"three_bytes", []byte{0x01, 0x02, 0x03}},
		{"four_bytes", []byte{0x01, 0x02, 0x03, 0x04}},
		{"five_bytes", []byte{0x01, 0x02, 0x03, 0x04, 0x05}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := paddedRFC1924.EncodeToString(tc.input)

			// padded output length is always a multiple of 5
			if len(tc.input) > 0 {
				assert.Equal(t, 0, len(encoded)%5)
			}

			decoded, err := paddedRFC1924.DecodeString(encoded)

			require.NoError(t, err)
			assert.Equal(t, tc.input, decoded)
		})
	}
}

func TestNewEncoding(t *testing.T) {
	t.Parallel()

	t.Run("valid_alphabets", func(t *testing.T) {
		enc := NewEncoding(ascii85Alphabet)
		require.NotNil(t, enc)
		assert.Equal(t, NoPadding, RFC1924.padChar)
		assert.Equal(t, NoPadding, Z85.padChar)
	})

	t.Run("wrong_length", func(t *testing.T) {
		assert.Panics(t, func() { NewEncoding("short") })
	})

	t.Run("duplicate_char", func(t *testing.T) {
		// 85 chars with duplicate
		alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}0"
		assert.Panics(t, func() { NewEncoding(alphabet) })
	})

	t.Run("contains_newline", func(t *testing.T) {
		alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}\n"
		assert.Panics(t, func() { NewEncoding(alphabet) })
	})

	t.Run("contains_cr", func(t *testing.T) {
		alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}\r"
		assert.Panics(t, func() { NewEncoding(alphabet) })
	})

	t.Run("contains_non_ascii", func(t *testing.T) {
		// 85 chars with a non-ASCII character (é = 0xC3 0xA9 in UTF-8)
		alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}é"
		assert.Panics(t, func() { NewEncoding(alphabet) })
	})

	t.Run("contains_high_byte", func(t *testing.T) {
		// 85 bytes with a high byte (0x80+) that's not valid ASCII
		alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}\x80"
		assert.Panics(t, func() { NewEncoding(alphabet) })
	})
}

func TestWithPadding(t *testing.T) {
	t.Parallel()

	t.Run("valid_padding", func(t *testing.T) {
		// use '.' since '=' is in the RFC1924 alphabet
		enc := RFC1924.WithPadding('.')
		require.NotNil(t, enc)
		assert.Equal(t, '.', enc.padChar)
	})

	t.Run("no_padding", func(t *testing.T) {
		enc := RFC1924.WithPadding(NoPadding)
		require.NotNil(t, enc)
		assert.Equal(t, NoPadding, enc.padChar)
	})

	t.Run("newline_padding", func(t *testing.T) {
		assert.Panics(t, func() { RFC1924.WithPadding('\n') })
	})

	t.Run("cr_padding", func(t *testing.T) {
		assert.Panics(t, func() { RFC1924.WithPadding('\r') })
	})

	t.Run("space_padding", func(t *testing.T) {
		assert.Panics(t, func() { RFC1924.WithPadding(' ') })
	})

	t.Run("tab_padding", func(t *testing.T) {
		assert.Panics(t, func() { RFC1924.WithPadding('\t') })
	})

	t.Run("padding_in_alphabet", func(t *testing.T) {
		assert.Panics(t, func() {
			RFC1924.WithPadding('0') // '0' is in RFC1924 alphabet
		})
	})

	t.Run("non_ascii_padding", func(t *testing.T) {
		assert.Panics(t, func() {
			RFC1924.WithPadding('é') // non-ASCII character
		})
	})

	t.Run("negative_padding", func(t *testing.T) {
		assert.Panics(t, func() { RFC1924.WithPadding(-2) })
	})
}

func TestEncodedLen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		enc      *Encoding
		inputLen int
		expected int
	}{
		// no padding: 4 bytes -> 5 chars, partial: 1->2, 2->3, 3->4
		{"no_pad_zero", RFC1924, 0, 0},
		{"no_pad_one", RFC1924, 1, 2},
		{"no_pad_two", RFC1924, 2, 3},
		{"no_pad_three", RFC1924, 3, 4},
		{"no_pad_four", RFC1924, 4, 5},
		{"no_pad_five", RFC1924, 5, 7},   // 5 + 2
		{"no_pad_eight", RFC1924, 8, 10}, // 10

		// with padding: always multiple of 5
		{"pad_zero", RFC1924.WithPadding('.'), 0, 0},
		{"pad_one", RFC1924.WithPadding('.'), 1, 5},
		{"pad_two", RFC1924.WithPadding('.'), 2, 5},
		{"pad_three", RFC1924.WithPadding('.'), 3, 5},
		{"pad_four", RFC1924.WithPadding('.'), 4, 5},
		{"pad_five", RFC1924.WithPadding('.'), 5, 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.enc.EncodedLen(tc.inputLen))
		})
	}
}

func TestDecodedLen(t *testing.T) {
	t.Parallel()

	// DecodedLen returns max possible decoded bytes
	// 5 chars -> 4 bytes, partial: 2->1, 3->2, 4->3, 1->0
	tests := []struct {
		name       string
		encodedLen int
		expected   int
	}{
		{"zero", 0, 0},
		{"one", 1, 0},   // 1 char can't decode to anything
		{"two", 2, 1},   // 2 chars -> 1 byte
		{"three", 3, 2}, // 3 chars -> 2 bytes
		{"four", 4, 3},  // 4 chars -> 3 bytes
		{"five", 5, 4},  // 5 chars -> 4 bytes
		{"six", 6, 4},   // 5 + 1 -> 4 + 0
		{"seven", 7, 5}, // 5 + 2 -> 4 + 1
		{"ten", 10, 8},  // 2 full blocks
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, RFC1924.DecodedLen(tc.encodedLen))
		})
	}
}

func TestCorruptInputError(t *testing.T) {
	t.Parallel()

	err := CorruptInputError(42)
	errStr := err.Error()

	assert.Contains(t, errStr, "42")
}

func TestStreamEncoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		chunks [][]byte
	}{
		{"single_chunk", [][]byte{[]byte("Hello, World!")}},
		{"multiple_chunks", [][]byte{[]byte("Hello"), []byte(", "), []byte("World!")}},
		{"byte_by_byte", [][]byte{{0x01}, {0x02}, {0x03}, {0x04}, {0x05}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// combine chunks for expected result
			var combined []byte
			for _, chunk := range tc.chunks {
				combined = append(combined, chunk...)
			}

			// encode via stream
			var buf bytes.Buffer
			encoder := NewEncoder(RFC1924, &buf)
			for _, chunk := range tc.chunks {
				n, err := encoder.Write(chunk)
				require.NoError(t, err)
				assert.Len(t, chunk, n)
			}
			err := encoder.Close()
			require.NoError(t, err)

			// decode and verify
			decoded, err := RFC1924.DecodeString(buf.String())
			require.NoError(t, err)
			assert.Equal(t, combined, decoded)
		})
	}
}

func TestStreamEncoderClose(t *testing.T) {
	t.Parallel()

	t.Run("double_close", func(t *testing.T) {
		var buf bytes.Buffer
		encoder := NewEncoder(RFC1924, &buf)
		_, err := encoder.Write([]byte{0x01, 0x02, 0x03, 0x04, 0x05}) // full block + partial
		require.NoError(t, err)

		require.NoError(t, encoder.Close())
		expected := buf.String()
		require.NoError(t, encoder.Close())
		assert.Equal(t, expected, buf.String())

		decoded, err := RFC1924.DecodeString(buf.String())
		require.NoError(t, err)
		assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04, 0x05}, decoded)
	})
}

func TestStreamDecoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"simple", []byte("Hello, World!")},
		{"binary", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}},
		{"longer_data", bytes.Repeat([]byte("test"), 100)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// encode first
			encoded := RFC1924.EncodeToString(tc.input)

			// decode via stream
			decoder := NewDecoder(RFC1924, bytes.NewReader([]byte(encoded)))
			decoded, err := io.ReadAll(decoder)

			require.NoError(t, err)
			assert.Equal(t, tc.input, decoded)
		})
	}
}

func TestEncodeDirectBuffer(t *testing.T) {
	t.Parallel()

	input := []byte("test data")
	dst := make([]byte, RFC1924.EncodedLen(len(input)))

	RFC1924.Encode(dst, input)

	decoded, err := RFC1924.DecodeString(string(dst))
	require.NoError(t, err)
	assert.Equal(t, input, decoded)
}

func TestDecodeDirectBuffer(t *testing.T) {
	t.Parallel()

	input := []byte("test data")
	encoded := RFC1924.EncodeToString(input)

	dst := make([]byte, RFC1924.DecodedLen(len(encoded)))
	n, err := RFC1924.Decode(dst, []byte(encoded))

	require.NoError(t, err)
	assert.Len(t, input, n)
	assert.Equal(t, input, dst[:n])
}

func TestRFC1924AppendEncode(t *testing.T) {
	t.Parallel()

	t.Run("append_to_empty", func(t *testing.T) {
		input := []byte("test")
		result := RFC1924.AppendEncode(nil, input)

		expected := RFC1924.EncodeToString(input)
		assert.Equal(t, expected, string(result))
	})

	t.Run("append_to_existing", func(t *testing.T) {
		prefix := []byte("prefix:")
		input := []byte("test")
		result := RFC1924.AppendEncode(prefix, input)

		expected := "prefix:" + RFC1924.EncodeToString(input)
		assert.Equal(t, expected, string(result))
	})

	t.Run("append_empty_input", func(t *testing.T) {
		prefix := []byte("prefix:")
		result := RFC1924.AppendEncode(prefix, nil)

		assert.Equal(t, "prefix:", string(result))
	})

	t.Run("preserves_original_capacity", func(t *testing.T) {
		prefix := make([]byte, 0, 100)
		prefix = append(prefix, "start:"...)
		input := []byte("data")

		result := RFC1924.AppendEncode(prefix, input)

		// verify original slice not modified
		assert.Len(t, prefix, 6)
		assert.Greater(t, len(result), len(prefix))
	})
}

func TestRFC1924AppendDecode(t *testing.T) {
	t.Parallel()

	t.Run("append_to_empty", func(t *testing.T) {
		input := []byte("test")
		encoded := RFC1924.EncodeToString(input)

		result, err := RFC1924.AppendDecode(nil, []byte(encoded))

		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("append_to_existing", func(t *testing.T) {
		prefix := []byte("prefix:")
		input := []byte("test")
		encoded := RFC1924.EncodeToString(input)

		result, err := RFC1924.AppendDecode(prefix, []byte(encoded))

		require.NoError(t, err)
		expected := append([]byte("prefix:"), input...)
		assert.Equal(t, expected, result)
	})

	t.Run("append_empty_input", func(t *testing.T) {
		prefix := []byte("prefix:")
		result, err := RFC1924.AppendDecode(prefix, nil)

		require.NoError(t, err)
		assert.Equal(t, "prefix:", string(result))
	})

	t.Run("corrupt_input_returns_partial", func(t *testing.T) {
		prefix := []byte("prefix:")
		// valid block + invalid char
		input := []byte("test")
		encoded := RFC1924.EncodeToString(input) + "[[["

		result, err := RFC1924.AppendDecode(prefix, []byte(encoded))

		require.Error(t, err)
		// should return prefix + successfully decoded data
		assert.GreaterOrEqual(t, len(result), len(prefix))
	})
}

type errorWriter struct {
	n   int
	err error
}

func (w *errorWriter) Write(p []byte) (int, error) {
	return w.n, w.err
}

func TestStreamEncoderWriteError(t *testing.T) {
	t.Parallel()

	t.Run("error_on_full_block", func(t *testing.T) {
		w := &errorWriter{err: io.ErrShortWrite}
		encoder := NewEncoder(RFC1924, w)

		_, err := encoder.Write([]byte("test"))
		require.Error(t, err)

		// subsequent writes should fail
		_, err = encoder.Write([]byte("more"))
		require.Error(t, err)
	})

	t.Run("error_on_buffered_flush", func(t *testing.T) {
		w := &errorWriter{err: io.ErrShortWrite}
		encoder := NewEncoder(RFC1924, w)

		// write less than 4 bytes (buffers)
		_, err := encoder.Write([]byte("ab"))
		require.NoError(t, err)

		// write more to trigger flush
		_, err = encoder.Write([]byte("cdef"))
		require.Error(t, err)
	})

	t.Run("error_on_close", func(t *testing.T) {
		w := &errorWriter{err: io.ErrShortWrite}
		encoder := NewEncoder(RFC1924, w)

		// write less than 4 bytes
		_, err := encoder.Write([]byte("ab"))
		require.NoError(t, err)

		// close should fail when flushing remaining
		err = encoder.Close()
		require.Error(t, err)

		// close again should return same error
		err = encoder.Close()
		require.Error(t, err)
	})

	t.Run("close_without_write", func(t *testing.T) {
		var buf bytes.Buffer
		encoder := NewEncoder(RFC1924, &buf)

		err := encoder.Close()
		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

type errorReader struct {
	data []byte
	err  error
}

func (r *errorReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 && r.err != nil {
		return n, r.err
	}
	return n, nil
}

// zeroReader returns (0, nil) a specified number of times before returning actual data.
type zeroReader struct {
	data       []byte
	zeroReads  int
	chunkSize  int
	zerosDone  int
	betweenAll bool // if true, return zero read between every chunk
}

func (r *zeroReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	// return (0, nil) the specified number of times at the start
	if r.zerosDone < r.zeroReads {
		r.zerosDone++
		return 0, nil
	}
	// optionally reset for next chunk
	if r.betweenAll {
		r.zerosDone = 0
	}
	n := r.chunkSize
	if n > len(r.data) {
		n = len(r.data)
	}
	if n > len(p) {
		n = len(p)
	}
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

// chunkReader splits data into chunks of specified size to test buffering behavior.
type chunkReader struct {
	data      []byte
	chunkSize int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := r.chunkSize
	if n > len(r.data) {
		n = len(r.data)
	}
	if n > len(p) {
		n = len(p)
	}
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

func TestStreamDecoderPartialBlockBuffering(t *testing.T) {
	t.Parallel()

	// This test verifies that the decoder correctly handles reads that split
	// in the middle of a 5-character encoded block.
	input := []byte("Hello, World! This is a longer test string.")
	encoded := RFC1924.EncodeToString(input)

	tests := []struct {
		name      string
		chunkSize int
	}{
		{"chunk_size_1", 1},
		{"chunk_size_3", 3},
		{"chunk_size_7", 7},   // splits mid-block (7 = 5 + 2)
		{"chunk_size_11", 11}, // splits mid-block (11 = 10 + 1)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &chunkReader{data: []byte(encoded), chunkSize: tc.chunkSize}
			decoder := NewDecoder(RFC1924, reader)
			decoded, err := io.ReadAll(decoder)

			require.NoError(t, err)
			assert.Equal(t, input, decoded)
		})
	}
}

// dataWithErrorReader returns all data with an error in a single Read call.
type dataWithErrorReader struct {
	data []byte
	err  error
	read bool
}

func (r *dataWithErrorReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, r.err
	}
	r.read = true
	n := copy(p, r.data)
	return n, r.err
}

// whitespaceOnlyReader returns only whitespace, then an error.
type whitespaceOnlyReader struct {
	returned bool
	err      error
}

func (r *whitespaceOnlyReader) Read(p []byte) (int, error) {
	if r.returned {
		return 0, r.err
	}
	r.returned = true
	// return whitespace that will be filtered out
	data := []byte("   \t\n\r   ")
	n := copy(p, data)
	return n, nil
}

// whitespaceWithErrorReader returns whitespace with a non-EOF error on first read.
type whitespaceWithErrorReader struct {
	called bool
	err    error
}

func (r *whitespaceWithErrorReader) Read(p []byte) (int, error) {
	if r.called {
		return 0, r.err
	}
	r.called = true
	// return whitespace that filters to empty, with error
	data := []byte("   \t\n\r   ")
	n := copy(p, data)
	return n, r.err
}

func TestStreamDecoderReadError(t *testing.T) {
	t.Parallel()

	t.Run("read_error", func(t *testing.T) {
		r := &errorReader{err: io.ErrUnexpectedEOF}
		decoder := NewDecoder(RFC1924, r)

		buf := make([]byte, 100)
		_, err := decoder.Read(buf)
		require.Error(t, err)

		// subsequent reads should return same error
		_, err = decoder.Read(buf)
		require.Error(t, err)
	})

	t.Run("error_after_whitespace_only", func(t *testing.T) {
		// reader returns only whitespace (filtered to empty) then error
		r := &whitespaceOnlyReader{err: io.ErrUnexpectedEOF}
		decoder := NewDecoder(RFC1924, r)

		buf := make([]byte, 100)
		_, err := decoder.Read(buf)
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("whitespace_with_error_not_eof", func(t *testing.T) {
		// reader returns whitespace with error in same read (filters to empty, not EOF)
		r := &whitespaceWithErrorReader{err: io.ErrUnexpectedEOF}
		decoder := NewDecoder(RFC1924, r)

		buf := make([]byte, 100)
		_, err := decoder.Read(buf)
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("decode_error_with_partial_data_small_buffer", func(t *testing.T) {
		// This tests the error path where some data is decoded before
		// the error, and the output buffer is too small. Uses padding mode
		// to trigger an error mid-stream after valid blocks are decoded.
		paddedEnc := RFC1924.WithPadding('.')

		// Create input: valid padded block + block with non-contiguous padding
		// Valid: 4 bytes = 5 chars (full block)
		valid := paddedEnc.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04})
		// Invalid: "AB.C." has non-contiguous padding (all chars valid, but padding broken)
		// The 'C' is a valid alphabet char, so it passes stream filter but fails
		// decodeFiltered's padding validation at line 297-299
		invalid := "AB.C."

		allData := valid + invalid

		// Use a reader that returns all data at once with EOF
		decoder := NewDecoder(paddedEnc, bytes.NewReader([]byte(allData)))

		// Use small buffer to trigger buffering on decode error
		buf := make([]byte, 2)

		n, err := decoder.Read(buf)
		// Should decode 4 bytes from valid block before hitting error
		// But buffer only holds 2, so should buffer the rest and defer error
		require.Equal(t, 2, n)
		require.NoError(t, err, "error should be deferred")

		// Continue reading to get buffered data
		var result []byte
		result = append(result, buf[:n]...)
		for {
			n, err = decoder.Read(buf)
			if n > 0 {
				result = append(result, buf[:n]...)
			}
			if err != nil {
				break
			}
		}

		// Should have gotten the 4 valid bytes
		assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, result)
		// And eventually hit the error
		assert.Error(t, err)
	})

	t.Run("data_with_error", func(t *testing.T) {
		// encode some data
		input := []byte("Hello, World!")
		encoded := RFC1924.EncodeToString(input)

		// reader returns all data with an error in single read
		r := &dataWithErrorReader{data: []byte(encoded), err: io.ErrUnexpectedEOF}
		decoder := NewDecoder(RFC1924, r)

		// should get data first despite error
		decoded, err := io.ReadAll(decoder)

		// data should be decoded successfully
		assert.Equal(t, input, decoded)
		// error should be returned after data exhausted
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("small_output_buffer", func(t *testing.T) {
		encoded := RFC1924.EncodeToString([]byte("Hello, World!"))
		decoder := NewDecoder(RFC1924, bytes.NewReader([]byte(encoded)))

		// read with very small buffer to trigger buffering
		buf := make([]byte, 2)
		var result []byte
		for {
			n, err := decoder.Read(buf)
			result = append(result, buf[:n]...)
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		assert.Equal(t, []byte("Hello, World!"), result)
	})

	t.Run("corrupt_input", func(t *testing.T) {
		// use invalid characters for RFC1924
		decoder := NewDecoder(RFC1924, bytes.NewReader([]byte("[[[")))

		buf := make([]byte, 100)
		_, err := decoder.Read(buf)
		assert.Error(t, err)
	})

	t.Run("buffered_data_with_error", func(t *testing.T) {
		// use input that encodes to complete 5-char blocks (8 bytes = 10 chars = 2 blocks)
		input := []byte("12345678")
		encoded := RFC1924.EncodeToString(input)
		require.Len(t, encoded, 10) // verify our assumption

		// add a single trailing valid character - this creates an invalid 1-char partial block
		// (minimum 2 chars needed to decode 1 byte)
		encodedWithError := encoded + "0"

		// use a small read buffer to force outBuf buffering
		decoder := NewDecoder(RFC1924, bytes.NewReader([]byte(encodedWithError)))
		buf := make([]byte, 2)

		var result []byte
		var lastErr error
		for {
			n, err := decoder.Read(buf)
			if n > 0 {
				result = append(result, buf[:n]...)
			}
			if err != nil {
				lastErr = err
				break
			}
		}

		// should have decoded the valid portion (8 bytes from 10 chars)
		assert.Equal(t, input, result)
		// should have received a corrupt input error for the trailing char
		var corruptErr CorruptInputError
		require.ErrorAs(t, lastErr, &corruptErr)
	})

	t.Run("error_with_small_buffer_returns_bytes", func(t *testing.T) {
		// This test verifies that when a decode error occurs and the output
		// buffer is smaller than decoded data, Read correctly returns the
		// number of bytes written (not 0) and defers the error.
		input := []byte("12345678") // 8 bytes -> 10 encoded chars
		encoded := RFC1924.EncodeToString(input)

		// append invalid trailing char to cause error after decoding valid data
		encodedWithError := encoded + "0"

		decoder := NewDecoder(RFC1924, bytes.NewReader([]byte(encodedWithError)))

		// use buffer smaller than decoded output (8 bytes) to trigger buffering
		buf := make([]byte, 3)

		// first read should return bytes, not error (error deferred)
		n, err := decoder.Read(buf)
		require.Equal(t, 3, n, "should return actual bytes written, not 0")
		require.NoError(t, err, "error should be deferred until buffer drained")
		require.Equal(t, input[:3], buf[:n])

		// continue reading to drain buffer
		var result []byte
		result = append(result, buf[:n]...)
		for {
			n, err = decoder.Read(buf)
			if n > 0 {
				result = append(result, buf[:n]...)
			}
			if err != nil {
				break
			}
		}

		// all valid data should be returned before error
		assert.Equal(t, input, result)
		var corruptErr CorruptInputError
		assert.ErrorAs(t, err, &corruptErr)
	})
}

func TestStreamDecoderZeroReads(t *testing.T) {
	t.Parallel()

	input := []byte("Hello, World!")
	encoded := RFC1924.EncodeToString(input)

	tests := []struct {
		name       string
		zeroReads  int
		chunkSize  int
		betweenAll bool
	}{
		{"zeros_at_start", 3, 10, false},
		{"zeros_between_chunks", 1, 5, true},
		{"many_zeros_at_start", 10, 20, false},
		{"single_byte_with_zeros", 2, 1, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &zeroReader{
				data:       []byte(encoded),
				zeroReads:  tc.zeroReads,
				chunkSize:  tc.chunkSize,
				betweenAll: tc.betweenAll,
			}
			decoder := NewDecoder(RFC1924, reader)
			decoded, err := io.ReadAll(decoder)

			require.NoError(t, err)
			assert.Equal(t, input, decoded)
		})
	}
}

func TestPaddedDecoding(t *testing.T) {
	t.Parallel()

	paddedEnc := RFC1924.WithPadding('.')

	t.Run("concatenated_padded_blocks", func(t *testing.T) {
		// encode two 1-byte values separately with padding
		encoded1 := paddedEnc.EncodeToString([]byte{0x42}) // 2 chars + 3 padding
		encoded2 := paddedEnc.EncodeToString([]byte{0x43}) // 2 chars + 3 padding

		concatenated := encoded1 + encoded2

		// should decode both blocks
		decoded, err := paddedEnc.DecodeString(concatenated)
		require.NoError(t, err)
		assert.Equal(t, []byte{0x42, 0x43}, decoded)
	})

	t.Run("padded_blocks_with_whitespace", func(t *testing.T) {
		encoded1 := paddedEnc.EncodeToString([]byte{0x42})
		encoded2 := paddedEnc.EncodeToString([]byte{0x43})

		withWhitespace := encoded1 + " \t\n" + encoded2

		decoded, err := paddedEnc.DecodeString(withWhitespace)
		require.NoError(t, err)
		assert.Equal(t, []byte{0x42, 0x43}, decoded)
	})

	t.Run("invalid_padding_positions", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			input string
		}{
			{"padding_at_start", "....."},
			{"padding_in_middle", "AB.CD"},      // padding not at end of block
			{"non_contiguous", "AB.C."},         // padding chars separated by non-padding
			{"single_padding_remainder", "."},   // 1-char remainder with padding
			{"two_padding_remainder", "AB...X"}, // valid block + padding in remainder
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := paddedEnc.DecodeString(tc.input)
				assert.Error(t, err)
			})
		}
	})

	t.Run("invalid_char_in_padded_block", func(t *testing.T) {
		// '[' is not in RFC1924 alphabet - test invalid char in data portion of block
		_, err := paddedEnc.DecodeString("AB[..")
		assert.Error(t, err)
	})

	t.Run("padding_in_trailing_bytes", func(t *testing.T) {
		// valid 5-char block followed by padding in trailing portion
		encoded := paddedEnc.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}) // 5 chars, no padding
		_, err := paddedEnc.DecodeString(encoded + "A.")                    // trailing 2 chars with padding
		assert.Error(t, err)
	})

	t.Run("invalid_char_in_trailing_bytes", func(t *testing.T) {
		// valid 5-char block followed by invalid char in trailing portion
		encoded := RFC1924.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}) // 5 chars
		_, err := RFC1924.DecodeString(encoded + "A[")                    // '[' is invalid
		assert.Error(t, err)
	})

	t.Run("stream_concatenated_blocks", func(t *testing.T) {
		encoded1 := paddedEnc.EncodeToString([]byte{0x42})
		encoded2 := paddedEnc.EncodeToString([]byte{0x43})
		concatenated := encoded1 + encoded2

		decoder := NewDecoder(paddedEnc, bytes.NewReader([]byte(concatenated)))
		decoded, err := io.ReadAll(decoder)

		require.NoError(t, err)
		assert.Equal(t, []byte{0x42, 0x43}, decoded)
	})

	t.Run("stream_padding_split_across_reads", func(t *testing.T) {
		encoded := paddedEnc.EncodeToString([]byte{0x42, 0x43}) // 2 bytes -> 3 chars + 2 padding

		// split in middle of padding region
		reader := &chunkReader{data: []byte(encoded), chunkSize: 4}
		decoder := NewDecoder(paddedEnc, reader)
		decoded, err := io.ReadAll(decoder)

		require.NoError(t, err)
		assert.Equal(t, []byte{0x42, 0x43}, decoded)
	})

	t.Run("stream_with_whitespace_between_padded", func(t *testing.T) {
		encoded1 := paddedEnc.EncodeToString([]byte{0x42})
		encoded2 := paddedEnc.EncodeToString([]byte{0x43})
		withWhitespace := encoded1 + " \n " + encoded2

		decoder := NewDecoder(paddedEnc, bytes.NewReader([]byte(withWhitespace)))
		decoded, err := io.ReadAll(decoder)

		require.NoError(t, err)
		assert.Equal(t, []byte{0x42, 0x43}, decoded)
	})

	// stream decoder tests for decodeFiltered error paths
	t.Run("stream_dataLen_zero", func(t *testing.T) {
		// "....." has 0 data chars - triggers dataLen < 2 check
		decoder := NewDecoder(paddedEnc, bytes.NewReader([]byte(".....")))
		_, err := io.ReadAll(decoder)
		assert.Error(t, err)
	})

	t.Run("stream_dataLen_one", func(t *testing.T) {
		// "A...." has 1 data char - triggers dataLen < 2 check
		decoder := NewDecoder(paddedEnc, bytes.NewReader([]byte("A....")))
		_, err := io.ReadAll(decoder)
		assert.Error(t, err)
	})

	t.Run("stream_invalid_char_in_padded_block", func(t *testing.T) {
		// "[B..." has invalid char '[' in data portion of padded block
		decoder := NewDecoder(paddedEnc, bytes.NewReader([]byte("[B...")))
		_, err := io.ReadAll(decoder)
		assert.Error(t, err)
	})

	t.Run("stream_padding_in_remainder", func(t *testing.T) {
		// valid 5-char block followed by remainder with padding
		valid := paddedEnc.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}) // 5 chars
		withPaddingRemainder := valid + "A."                              // remainder has padding
		decoder := NewDecoder(paddedEnc, bytes.NewReader([]byte(withPaddingRemainder)))
		_, err := io.ReadAll(decoder)
		assert.Error(t, err)
	})

	t.Run("stream_invalid_char_in_remainder", func(t *testing.T) {
		// valid 5-char block followed by remainder with invalid char
		valid := RFC1924.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}) // 5 chars
		withInvalidRemainder := valid + "A["                            // '[' is invalid
		decoder := NewDecoder(RFC1924, bytes.NewReader([]byte(withInvalidRemainder)))
		_, err := io.ReadAll(decoder)
		assert.Error(t, err)
	})
}

func TestDecodeWhitespaceAlphabetAware(t *testing.T) {
	t.Parallel()

	// create encoding with space in the alphabet (replaces '~' which is the last char in RFC1924)
	alphabetWithSpace := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|} "
	encWithSpace := NewEncoding(alphabetWithSpace)

	// create encoding with tab in the alphabet (replaces '~')
	alphabetWithTab := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}\t"
	encWithTab := NewEncoding(alphabetWithTab)

	t.Run("space_in_alphabet_not_skipped", func(t *testing.T) {
		// space is at alphabet position 84 (value 84)
		// create a 5-char encoded block where the last char is space (value 84)
		// value % 85 = 84 means value = 84, 84+85=169, 84+85*2=254, etc.
		// for simplicity, encode a full block where value 84 appears
		// we'll construct a known encoded string with space and decode it

		// "00000" decodes to: 0*85^4 + 0*85^3 + 0*85^2 + 0*85 + 0 = 0 -> [0,0,0,0]
		// "0000 " (with space = 84) decodes to: 0 + 0 + 0 + 0 + 84 = 84 -> different bytes

		// first verify that space in encoded data produces different result than without space
		decoded1, err := encWithSpace.DecodeString("00000")
		require.NoError(t, err)

		decoded2, err := encWithSpace.DecodeString("0000 ") // space = value 84
		require.NoError(t, err)

		assert.NotEqual(t, decoded1, decoded2) // space in alphabet should decode differently than '0'

		// round-trip with data that encodes to include spaces
		input := []byte{0x00, 0x00, 0x00, 0x54} // 0x54 = 84, should encode with space
		encoded := encWithSpace.EncodeToString(input)
		decoded, err := encWithSpace.DecodeString(encoded)
		require.NoError(t, err)
		assert.Equal(t, input, decoded)
	})

	t.Run("tab_in_alphabet_not_skipped", func(t *testing.T) {
		// similar test with tab
		decoded1, err := encWithTab.DecodeString("00000")
		require.NoError(t, err)

		decoded2, err := encWithTab.DecodeString("0000\t") // tab = value 84
		require.NoError(t, err)

		assert.NotEqual(t, decoded1, decoded2) // tab in alphabet should decode differently than '0'

		// round-trip
		input := []byte{0x00, 0x00, 0x00, 0x54}
		encoded := encWithTab.EncodeToString(input)
		decoded, err := encWithTab.DecodeString(encoded)
		require.NoError(t, err)
		assert.Equal(t, input, decoded)
	})

	t.Run("space_not_in_alphabet_skipped", func(t *testing.T) {
		// RFC1924 does not include space, so spaces should be ignored
		input := []byte{0x01, 0x02, 0x03, 0x04}
		encoded := RFC1924.EncodeToString(input)

		// insert spaces between each character
		var sb strings.Builder
		for i, c := range encoded {
			if i > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteRune(c)
		}

		decoded, err := RFC1924.DecodeString(sb.String())
		require.NoError(t, err)
		assert.Equal(t, input, decoded) // space not in alphabet should be skipped
	})

	t.Run("tab_not_in_alphabet_skipped", func(t *testing.T) {
		input := []byte{0x01, 0x02, 0x03, 0x04}
		encoded := RFC1924.EncodeToString(input)

		// insert tabs
		withTabs := encoded[:2] + "\t\t" + encoded[2:]
		decoded, err := RFC1924.DecodeString(withTabs)
		require.NoError(t, err)
		assert.Equal(t, input, decoded) // tab not in alphabet should be skipped
	})

	t.Run("stream_decoder_space_in_alphabet", func(t *testing.T) {
		input := []byte("test data for stream")
		encoded := encWithSpace.EncodeToString(input)

		decoder := NewDecoder(encWithSpace, bytes.NewReader([]byte(encoded)))
		decoded, err := io.ReadAll(decoder)

		require.NoError(t, err)
		assert.Equal(t, input, decoded)
	})

	t.Run("stream_decoder_whitespace_skipped", func(t *testing.T) {
		input := []byte("test data")
		encoded := RFC1924.EncodeToString(input)

		// insert whitespace between blocks (every 5 chars)
		var sb strings.Builder
		for i, c := range encoded {
			sb.WriteRune(c)
			if (i+1)%5 == 0 {
				sb.WriteString(" \t\n\r")
			}
		}

		decoder := NewDecoder(RFC1924, bytes.NewReader([]byte(sb.String())))
		decoded, err := io.ReadAll(decoder)

		require.NoError(t, err)
		assert.Equal(t, input, decoded)
	})
}
