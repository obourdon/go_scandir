// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mime collects common mime constants.
package mime_hash

import (
	"hash"
	"strconv"
)

// Hash identifies a cryptographic hash function that is implemented in another
// package. Inspired by https://golang.org/src/crypto/crypto.go
type Hash uint

// HashFunc simply returns the value of h so that Hash implements SignerOpts.
func (h Hash) HashFunc() Hash {
	return h
}

const (
	MAGIC	Hash = 1 + iota // import mime/magic
	HTTP                    // import mime/http
	maxHash
)

// Size returns the length, in bytes, of a digest resulting from the given hash
// function. It doesn't require that the hash function in question be linked
// into the program.
func (h Hash) Size() int {
	return 64
}

func (h Hash) Check() {
	if h > 0 && h < maxHash {
		return
	}
	panic("mime: Size of unknown hash function")
}

var hashes = make([]func()hash.Hash, maxHash)

// New returns a new hash.Hash calculating the given hash function. New panics
// if the hash function is not linked into the binary.
func (h Hash) New() hash.Hash {
	if h > 0 && h < maxHash {
		f := hashes[h]
		if f != nil {
			return f()
		}
	}
	panic("mime: requested hash function #" + strconv.Itoa(int(h)) + " is unavailable")
}

// Available reports whether the given hash function is linked into the binary.
func (h Hash) Available() bool {
	return h < maxHash && hashes[h] != nil
}

// RegisterHash registers a function that returns a new instance of the given
// hash function. This is intended to be called from the init function in
// packages that implement hash functions.
func RegisterHash(h Hash, f func() hash.Hash) {
	if h >= maxHash {
		panic("mime: RegisterHash of unknown hash function")
	}
	hashes[h] = f
}
