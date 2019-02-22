package http

import (
	"../../mime_hash"
	"hash"
	"net/http"
)

// digest represents the partial evaluation of a checksum.
type digest struct {
	crc string
	done bool
}

func (d *digest) Size() int {
	return 1
}

func (d *digest) BlockSize() int {
	return 1
}

func (d *digest) Reset() {
	d.crc = ""
	d.done = false
}

func (d *digest) Write(p []byte) (n int, err error) {
	if !d.done {
		d.done = true
		d.crc = http.DetectContentType(p)
	}
	return len(p), nil
}

func (d *digest) Sum32() string {
	return d.crc
}

func (d *digest) Sum(in []byte) []byte {
	s := d.Sum32()
	return append(in, s...)
}

func init() {
	mime_hash.RegisterHash(mime_hash.HTTP, New)
}

// New returns a new hash.Hash computing the MD5 checksum. The Hash also
// implements encoding.BinaryMarshaler and encoding.BinaryUnmarshaler to
// marshal and unmarshal the internal state of the hash.
func New() hash.Hash {
	d := new(digest)
	d.Reset()
	return d
}
