package base85

import (
	"io"
	"runtime"
	"strconv"
)

const (
	// NoPadding is used with WithPadding to disable padding. This is the default for base85 encodings.
	NoPadding rune = -1

	alphabetSize = 85

	// powers of 85 for decoding
	pow85_1 = 85
	pow85_2 = 85 * 85
	pow85_3 = 85 * 85 * 85
	pow85_4 = 85 * 85 * 85 * 85
)

// RFC1924 is the encoding defined in RFC 1924 for compact representation of IPv6 addresses.
// It uses characters 0-9, A-Z, a-z, and 23 punctuation symbols,  with no padding by default.
var RFC1924 = NewEncoding("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}~")

// Z85 is the encoding used by ZeroMQ for representing binary data as printable ASCII text.
// It uses characters 0-9, a-z, A-Z, and 23 punctuation symbols, with no padding by default.
var Z85 = NewEncoding("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-:+=^!/*?&<>()[]{}@%$#")

// Encoding represents a base85 encoding/decoding scheme defined by an 85-character alphabet.
type Encoding struct {
	encode    [alphabetSize]byte
	decodeMap [256]uint8
	padChar   rune
}

// NewEncoding returns a new Encoding defined by the given 85-character
// alphabet, which must contain only unique ASCII characters and must not
// contain newline characters ('\r', '\n'). The resulting Encoding uses no
// padding by default.
func NewEncoding(encoder string) *Encoding {
	if len(encoder) != alphabetSize {
		panic("base85: encoding alphabet must be 85 bytes long")
	}

	e := &Encoding{padChar: NoPadding}
	copy(e.encode[:], encoder)

	for i := range e.decodeMap {
		e.decodeMap[i] = 0xFF
	}

	for i, c := range encoder {
		if c > 127 {
			panic("base85: encoding alphabet must contain only ASCII characters")
		} else if c == '\n' || c == '\r' {
			panic("base85: encoding alphabet contains newline character")
		} else if e.decodeMap[c] != 0xFF {
			panic("base85: encoding alphabet contains duplicate character")
		}
		e.decodeMap[c] = uint8(i)
	}

	return e
}

// WithPadding creates a new Encoding identical to enc except with a specified
// padding character, or NoPadding to disable padding. The padding character
// must be an ASCII character, must not be a whitespace character ('\r', '\n',
// ' ', '\t'), and must not be contained in the encoding alphabet.
func (enc Encoding) WithPadding(padding rune) *Encoding {
	if padding == '\r' || padding == '\n' || padding == ' ' || padding == '\t' {
		panic("base85: invalid padding character")
	}

	if padding != NoPadding {
		if padding < 0 || padding > 127 {
			panic("base85: padding character must be ASCII")
		}
		for _, c := range enc.encode {
			if rune(c) == padding {
				panic("base85: padding character is in alphabet")
			}
		}
	}

	enc.padChar = padding
	return &enc
}

// EncodedLen returns the length in bytes of the base85 encoding of an input buffer of length n.
func (enc *Encoding) EncodedLen(n int) int {
	if enc.padChar == NoPadding {
		// 4 bytes -> 5 chars, partial blocks: 1->2, 2->3, 3->4
		fullBlocks := n / 4
		remainder := n % 4
		length := fullBlocks * 5
		if remainder > 0 {
			length += remainder + 1
		}
		return length
	}
	// with padding: always multiple of 5
	return (n + 3) / 4 * 5
}

// DecodedLen returns the maximum length in bytes of the decoded data corresponding to n bytes of base85-encoded data.
func (enc *Encoding) DecodedLen(n int) int {
	// 5 chars -> 4 bytes, partial: 2->1, 3->2, 4->3
	fullBlocks := n / 5
	remainder := n % 5
	length := fullBlocks * 4
	if remainder > 1 {
		length += remainder - 1
	}
	return length
}

// Encode encodes src using the encoding enc, writing EncodedLen(len(src)) bytes to dst.
func (enc *Encoding) Encode(dst, src []byte) {
	if len(src) == 0 {
		return
	}

	di := 0
	for len(src) >= 4 {
		// big-endian uint32
		val := uint32(src[0])<<24 | uint32(src[1])<<16 | uint32(src[2])<<8 | uint32(src[3])

		dst[di+4] = enc.encode[val%85]
		val /= 85
		dst[di+3] = enc.encode[val%85]
		val /= 85
		dst[di+2] = enc.encode[val%85]
		val /= 85
		dst[di+1] = enc.encode[val%85]
		val /= 85
		dst[di] = enc.encode[val%85]

		src = src[4:]
		di += 5
	}

	// handle remaining bytes
	if len(src) > 0 {
		var val uint32
		switch len(src) {
		case 3:
			val = uint32(src[0])<<24 | uint32(src[1])<<16 | uint32(src[2])<<8
		case 2:
			val = uint32(src[0])<<24 | uint32(src[1])<<16
		case 1:
			val = uint32(src[0]) << 24
		}

		// encode and output only needed characters
		var buf [5]byte
		buf[4] = enc.encode[val%85]
		val /= 85
		buf[3] = enc.encode[val%85]
		val /= 85
		buf[2] = enc.encode[val%85]
		val /= 85
		buf[1] = enc.encode[val%85]
		val /= 85
		buf[0] = enc.encode[val%85]

		// output chars: 1 byte -> 2 chars, 2 bytes -> 3 chars, 3 bytes -> 4 chars
		outLen := len(src) + 1
		copy(dst[di:], buf[:outLen])
		di += outLen

		// add padding if needed
		if enc.padChar != NoPadding {
			for i := outLen; i < 5; i++ {
				dst[di] = byte(enc.padChar)
				di++
			}
		}
	}
}

// EncodeToString returns the base85 encoding of src.
func (enc *Encoding) EncodeToString(src []byte) string {
	buf := make([]byte, enc.EncodedLen(len(src)))
	enc.Encode(buf, src)
	return string(buf)
}

// AppendEncode appends the base85 encoding of src to dst and returns the extended buffer.
func (enc *Encoding) AppendEncode(dst, src []byte) []byte {
	n := enc.EncodedLen(len(src))
	dst = append(dst, make([]byte, n)...)
	enc.Encode(dst[len(dst)-n:], src)
	return dst
}

// Decode decodes src using the encoding enc. It writes at most DecodedLen(len(src)) bytes
// to dst and returns the number of bytes written. Whitespace (space, tab, CR, LF) is ignored
// unless included in the encoding alphabet. If src contains invalid base85 data, it will
// return the number of bytes successfully written and CorruptInputError.
func (enc *Encoding) Decode(dst, src []byte) (n int, err error) {
	if len(src) == 0 {
		return 0, nil
	}

	var nb int
	var digits [5]uint32
	hasPadding := enc.padChar != NoPadding
	padCount := 0 // tracks padding chars seen in current block

	for i := 0; i < len(src); i++ {
		c := src[i]

		// skip whitespace if not in alphabet
		if (c == ' ' || c == '\t' || c == '\n' || c == '\r') && enc.decodeMap[c] == 0xFF {
			continue
		}

		// padding handling
		if hasPadding && rune(c) == enc.padChar {
			if nb < 2 {
				return n, CorruptInputError(i)
			}
			padCount++
			if nb+padCount == 5 {
				// block complete - decode and reset
				written := decodePartial(dst[n:], digits[:], nb)
				if written < 0 {
					return n, CorruptInputError(i)
				}
				n += written
				nb = 0
				padCount = 0
			}
			continue
		}

		// data char after padding started is an error
		if padCount > 0 {
			return n, CorruptInputError(i)
		}

		d := enc.decodeMap[c]
		if d == 0xFF {
			return n, CorruptInputError(i)
		}

		digits[nb] = uint32(d)
		nb++

		if nb == 5 {
			val := uint64(digits[0])*pow85_4 + uint64(digits[1])*pow85_3 + uint64(digits[2])*pow85_2 + uint64(digits[3])*pow85_1 + uint64(digits[4])
			if val>>32 != 0 {
				return n, CorruptInputError(i)
			}
			dst[n] = byte(val >> 24)
			dst[n+1] = byte(val >> 16)
			dst[n+2] = byte(val >> 8)
			dst[n+3] = byte(val)
			n += 4
			nb = 0
		}
	}

	// handle remaining digits (unpadded case)
	if nb > 0 {
		// with padding enabled, the trailing block must be padded to 5 chars
		if nb == 1 || padCount > 0 || hasPadding {
			return n, CorruptInputError(len(src))
		}
		written := decodePartial(dst[n:], digits[:], nb)
		if written < 0 {
			return n, CorruptInputError(len(src))
		}
		n += written
	}

	return n, nil
}

// decodePartial decodes 2-4 accumulated digit values into output bytes.
// Returns the number of bytes written, or -1 if the value overflows uint32.
func decodePartial(dst []byte, digits []uint32, nb int) int {
	// fill remaining with 84 (highest digit) for implicit padding
	for i := nb; i < 5; i++ {
		digits[i] = 84
	}
	val := uint64(digits[0])*pow85_4 + uint64(digits[1])*pow85_3 + uint64(digits[2])*pow85_2 + uint64(digits[3])*pow85_1 + uint64(digits[4])
	if val>>32 != 0 {
		return -1
	}
	v := uint32(val)
	for i := 0; i < nb-1; i++ {
		dst[i] = byte(v >> 24)
		v <<= 8
	}
	return nb - 1
}

// decodeBlock decodes 2-5 base85 alphabet bytes into 1-4 output bytes.
// Returns the number of bytes written to dst, or -1 if the value overflows
// uint32. Caller must ensure all bytes in src are valid alphabet characters
// (not padding, not invalid).
func (enc *Encoding) decodeBlock(dst, src []byte) int {
	// initialize with highest alphabet index (84) for implicit padding
	d0, d1, d2, d3, d4 := uint64(84), uint64(84), uint64(84), uint64(84), uint64(84)

	// map input bytes to digit values
	switch len(src) {
	case 5:
		d4 = uint64(enc.decodeMap[src[4]])
		fallthrough
	case 4:
		d3 = uint64(enc.decodeMap[src[3]])
		fallthrough
	case 3:
		d2 = uint64(enc.decodeMap[src[2]])
		fallthrough
	case 2:
		d1 = uint64(enc.decodeMap[src[1]])
		d0 = uint64(enc.decodeMap[src[0]])
	}

	val := d0*pow85_4 + d1*pow85_3 + d2*pow85_2 + d3*pow85_1 + d4
	if val>>32 != 0 {
		return -1
	}

	// output length: 5 chars -> 4 bytes, otherwise len-1
	// only write the bytes we actually produce
	switch len(src) {
	case 5:
		dst[3] = byte(val)
		fallthrough
	case 4:
		dst[2] = byte(val >> 8)
		fallthrough
	case 3:
		dst[1] = byte(val >> 16)
		fallthrough
	case 2:
		dst[0] = byte(val >> 24)
	}

	if len(src) == 5 {
		return 4
	}
	return len(src) - 1
}

// decodeFiltered decodes pre-validated and filtered input (whitespace removed,
// but padding chars still present if padding is enabled).
func (enc *Encoding) decodeFiltered(dst, src []byte) (n int, err error) {
	if len(src) == 0 {
		return 0, nil
	}

	consumed := 0
	for len(src) >= 5 {
		// find data length in this block (may end early due to padding)
		dataLen := 5
		if enc.padChar != NoPadding {
			for i := 0; i < 5; i++ {
				if rune(src[i]) == enc.padChar {
					dataLen = i
					break
				}
			}
			// validate: all chars after first padding must also be padding
			for i := dataLen; i < 5; i++ {
				if rune(src[i]) != enc.padChar {
					return n, CorruptInputError(consumed + i)
				}
			}
			// need at least 2 data chars to produce 1 byte
			if dataLen < 2 {
				return n, CorruptInputError(consumed + dataLen)
			}
		}

		// validate all data chars are in alphabet
		for i := 0; i < dataLen; i++ {
			if enc.decodeMap[src[i]] == 0xFF {
				return n, CorruptInputError(consumed + i)
			}
		}

		written := enc.decodeBlock(dst[n:], src[:dataLen])
		if written < 0 {
			return n, CorruptInputError(consumed)
		}
		n += written
		src = src[5:]
		consumed += 5
	}

	// handle remaining 1-4 chars (only valid when padding is disabled)
	if len(src) > 0 {
		// with padding enabled, all blocks must be padded to 5 chars
		if len(src) == 1 || enc.padChar != NoPadding {
			return n, CorruptInputError(consumed)
		}
		for i, c := range src {
			if enc.decodeMap[c] == 0xFF {
				return n, CorruptInputError(consumed + i)
			}
		}
		written := enc.decodeBlock(dst[n:], src)
		if written < 0 {
			return n, CorruptInputError(consumed)
		}
		n += written
	}

	return n, nil
}

// DecodeString returns the bytes represented by the base85 string s.
func (enc *Encoding) DecodeString(s string) ([]byte, error) {
	dst := make([]byte, enc.DecodedLen(len(s)))
	n, err := enc.Decode(dst, []byte(s))
	return dst[:n], err
}

// AppendDecode appends the base85 decoding of src to dst and returns the extended buffer.
// If the input is malformed, it returns the partially decoded src and an error.
func (enc *Encoding) AppendDecode(dst, src []byte) ([]byte, error) {
	n := enc.DecodedLen(len(src))
	dst = append(dst, make([]byte, n)...)
	written, err := enc.Decode(dst[len(dst)-n:], src)
	return dst[:len(dst)-n+written], err
}

// NewEncoder returns a new base85 stream encoder. Data written to the
// returned writer will be encoded using enc and then written to w.
// Base85 encodings operate in 4-byte blocks; when finished writing,
// the caller must Close the returned encoder to flush any partially written blocks.
func NewEncoder(enc *Encoding, w io.Writer) io.WriteCloser {
	return &encoder{enc: enc, w: w}
}

type encoder struct {
	enc    *Encoding
	w      io.Writer
	buf    [4]byte
	nbuf   int
	outBuf [5]byte
	err    error
}

func (e *encoder) Write(p []byte) (n int, err error) {
	if e.err != nil {
		return 0, e.err
	}

	// use buffered data first
	if e.nbuf > 0 {
		for len(p) > 0 && e.nbuf < 4 {
			e.buf[e.nbuf] = p[0]
			e.nbuf++
			p = p[1:]
			n++
		}
		if e.nbuf == 4 {
			e.enc.Encode(e.outBuf[:], e.buf[:])
			if _, e.err = e.w.Write(e.outBuf[:]); e.err != nil {
				return n, e.err
			}
			e.nbuf = 0
		}
	}

	// encode full blocks
	for len(p) >= 4 {
		e.enc.Encode(e.outBuf[:], p[:4])
		if _, e.err = e.w.Write(e.outBuf[:]); e.err != nil {
			return n, e.err
		}
		p = p[4:]
		n += 4
	}

	// buffer remaining
	for len(p) > 0 {
		e.buf[e.nbuf] = p[0]
		e.nbuf++
		p = p[1:]
		n++
	}

	return n, nil
}

func (e *encoder) Close() error {
	if e.err != nil {
		return e.err
	}

	if e.nbuf > 0 {
		encoded := make([]byte, e.enc.EncodedLen(e.nbuf))
		e.enc.Encode(encoded, e.buf[:e.nbuf])
		e.nbuf = 0
		if _, e.err = e.w.Write(encoded); e.err != nil {
			return e.err
		}
	}

	return nil
}

// NewDecoder constructs a new base85 stream decoder. Data read from the returned reader will be decoded using enc.
func NewDecoder(enc *Encoding, r io.Reader) io.Reader {
	return &decoder{enc: enc, r: r}
}

// maxConsecutiveEmptyReads bounds retries when the underlying reader returns (0, nil).
const maxConsecutiveEmptyReads = 100

type decoder struct {
	enc        *Encoding
	r          io.Reader
	readBuf    [1024]byte
	encBuf     [4]byte // buffer for incomplete encoded blocks (max 4 chars waiting for 5th)
	nenc       int     // number of valid bytes in encBuf
	outBuf     []byte  // buffered decoded output
	err        error
	eof        bool
	streamPos  int64 // total bytes consumed from r (start of next read)
	emptyReads int   // consecutive (0, nil) reads
}

func (d *decoder) Read(p []byte) (n int, err error) {
	// return buffered decoded data first
	if len(d.outBuf) > 0 {
		n = copy(p, d.outBuf)
		d.outBuf = d.outBuf[n:]
		return n, nil
	} else if d.err != nil {
		return 0, d.err
	} else if d.eof {
		return 0, io.EOF
	}

	// loop until we have data to return or hit EOF/error
	for {
		// read more encoded data
		nr, readErr := d.r.Read(d.readBuf[:])
		if readErr != nil && readErr != io.EOF {
			// store error but process any data that was read
			d.err = readErr
			d.eof = true // treat terminal error as end of stream for decoding
			if nr == 0 {
				return 0, readErr
			}
		} else if readErr == io.EOF {
			d.eof = true
		}
		if nr == 0 && !d.eof && d.err == nil {
			// reader returned (0, nil), bounded retry then give up
			d.emptyReads++
			if d.emptyReads >= maxConsecutiveEmptyReads {
				d.err = io.ErrNoProgress
				return 0, d.err
			}
			runtime.Gosched()
			continue
		}
		d.emptyReads = 0

		chunkStart := d.streamPos
		d.streamPos += int64(nr)

		// filter whitespace and padding, combine with buffered encoded data
		filtered := make([]byte, 0, d.nenc+nr)
		filtered = append(filtered, d.encBuf[:d.nenc]...)
		d.nenc = 0
		for i, c := range d.readBuf[:nr] {
			if d.enc.padChar != NoPadding && rune(c) == d.enc.padChar {
				filtered = append(filtered, c)
				continue
			} else if (c == ' ' || c == '\t' || c == '\n' || c == '\r') && d.enc.decodeMap[c] == 0xFF {
				continue
			} else if d.enc.decodeMap[c] == 0xFF {
				d.err = CorruptInputError(chunkStart + int64(i))
				return 0, d.err
			}
			filtered = append(filtered, c)
		}

		// if not at EOF, buffer incomplete block for next read
		if !d.eof {
			remainder := len(filtered) % 5
			if remainder > 0 {
				d.nenc = copy(d.encBuf[:], filtered[len(filtered)-remainder:])
				filtered = filtered[:len(filtered)-remainder]
			}
		}

		if len(filtered) == 0 {
			if d.eof {
				if d.err == nil {
					d.err = io.EOF
				}
				return 0, d.err
			} else if d.err != nil {
				return 0, d.err
			}
			continue // need more data
		}

		// decode the filtered data
		decoded := make([]byte, d.enc.DecodedLen(len(filtered)))
		nd, decErr := d.enc.decodeFiltered(decoded, filtered)
		if decErr != nil {
			// approximate offset, error lies within this read's chunk
			d.err = CorruptInputError(chunkStart)
			// still return what we decoded
			n = copy(p, decoded[:nd])
			if n < nd {
				d.outBuf = decoded[n:nd]
			}
			if n > 0 {
				return n, nil // defer error until buffer drained
			}
			return 0, d.err
		}

		// copy to output
		n = copy(p, decoded[:nd])
		if n < nd {
			d.outBuf = decoded[n:nd]
		}
		return n, nil
	}
}

// CorruptInputError is returned by Decode when the input contains invalid base85 data.
// The integer value represents the byte offset where the error was detected.
type CorruptInputError int64

func (e CorruptInputError) Error() string {
	return "base85: illegal character at offset " + strconv.FormatInt(int64(e), 10)
}
