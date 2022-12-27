package datatype //nolint:revive

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/multiformats/go-varint"
)

var viBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, varint.MaxLenUvarint63)
		return &b // https://staticcheck.io/docs/checks/#SA6002
	},
}

type UVarInt uint64 //nolint:revive

func (u UVarInt) Len() int { //nolint:revive
	switch {
	case u < 1<<(7*1):
		return 1
	case u < 1<<(7*2):
		return 2
	case u < 1<<(7*3):
		return 3
	case u < 1<<(7*4):
		return 4
	case u < 1<<(7*5):
		return 5
	case u < 1<<(7*6):
		return 6
	case u < 1<<(7*7):
		return 7
	case u < 1<<(7*8):
		return 8
	case u < 1<<(7*9):
		return 9
	default:
		return -1
	}
}

func FromBytes(buf []byte) (UVarInt, int, error) { //nolint:revive
	v, n, err := varint.FromUvarint(buf)
	if err != nil {
		return 0, 0, err
	}
	return UVarInt(v), n, nil
}
func FromBufio(r bufio.Reader) (UVarInt, int, error) { //nolint:revive
	buf, maybeErr := r.Peek(varint.MaxLenUvarint63)
	if len(buf) == 0 || maybeErr == bufio.ErrBufferFull {
		return 0, 0, maybeErr
	}
	v, n, err := varint.FromUvarint(buf)
	if err != nil {
		return 0, 0, err
	}
	// performed a successful decode - advance
	if _, err := r.Discard(n); err != nil {
		return 0, 0, err
	}
	return UVarInt(v), n, nil
}
func FromReaderAt(r io.ReaderAt, offset int64) (UVarInt, int, error) { //nolint:revive
	buf := viBufPool.Get().(*[]byte)
	defer viBufPool.Put(buf)

	n, maybeErr := r.ReadAt(*buf, offset)
	if n == 0 {
		return 0, 0, maybeErr
	}

	return FromBytes((*buf)[:n])
}

func (u UVarInt) Append(target []byte) []byte { //nolint:revive
	if u > varint.MaxValueUvarint63 {
		// only spot we get to panic / no easy way to signal error
		panic(fmt.Sprintf("value %d larger than permitted 2^63-1", u))
	}
	buf := viBufPool.Get().(*[]byte)
	defer viBufPool.Put(buf)

	return append(
		target,
		(*buf)[:varint.PutUvarint(*buf, uint64(u))]...,
	)
}
func (u UVarInt) WriteTo(w io.Writer) (int64, error) { //nolint:revive
	if u > varint.MaxValueUvarint63 {
		return 0, errors.New("value larger than the maximum 2^63 - 1")
	}
	buf := viBufPool.Get().(*[]byte)
	defer viBufPool.Put(buf)

	n, err := w.Write(
		(*buf)[:varint.PutUvarint(*buf, uint64(u))],
	)
	return int64(n), err
}
func (u UVarInt) WriteAt(w io.WriterAt, offset int64) (int, error) { //nolint:revive
	if u > varint.MaxValueUvarint63 {
		return 0, errors.New("value larger than the maximum 2^63 - 1")
	}
	buf := viBufPool.Get().(*[]byte)
	defer viBufPool.Put(buf)

	return w.WriteAt(
		(*buf)[:varint.PutUvarint(*buf, uint64(u))],
		offset,
	)
}

var _ io.WriterTo = UVarInt(0)
