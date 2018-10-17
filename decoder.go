package lhex

import (
	"fmt"
	"io"
	"io/ioutil"
)

type unresolved struct {
	label string
	rel   int
}

// Decoder takes an input io.Reader providing input in hexdump form, and
// implements sparse.Reader to make the bytes described by the input available
// to the caller.  Callers may call Read() to read the bytes, and Next() to
// advance between segments of data if the input contains gaps.
type Decoder struct {
	err    error
	labels Labels
	scan   *scanner

	started    bool
	readyOfs   int64 // start of data[]
	data       []byte
	nextData   []byte
	nextOffset int64
	resolv     []unresolved
}

// NewDecoder creates a Decoder from the given reader.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		scan: newScanner(r),
	}
}

// Next moves to the next block of data, in the event the data described
// by the input hexdump isn't contiguous.  Returns io.EOF when the end of
// the hexdump input is reached.  Note that skips may be negative if the
// offsets described by the hexdump are out of order.
func (d *Decoder) Next() (skipped int64, err error) {
	if !d.started {
		if _, err = d.nextContiguous(); err != nil {
			return 0, err
		}
	}
	if d.err != nil {
		return 0, d.err
	}
	if d.nextData == nil && d.nextOffset == 0 {
		_, err = io.Copy(ioutil.Discard, d)
		if err != nil {
			return
		}
	}
	d.data = d.nextData
	lastOfs := d.readyOfs
	d.readyOfs = d.nextOffset
	d.nextData = nil
	d.nextOffset = 0

	skipped = d.readyOfs - lastOfs
	return
}

// Read reads up to len(p) of raw bytes from the hexdump input.  In the event
// the hexdump input skips to a non-contiguous offset, Read will only read
// from the current segment and will then return io.EOF.  Callers should call
// Next() to move to the new segment of data in the hexdump, at which point
// Read will read from that segment.
func (d *Decoder) Read(p []byte) (n int, err error) {
	if d.nextOffset > 0 {
		// we've exhausted one stream and are waiting for the caller to call Next to move on to
		// the next one.
		return 0, io.EOF
	}
	for n < len(p) && err == nil {
		// first try to satisfy from buffer
		if len(d.data) > 0 {
			nn := copy(p[n:], d.data)
			n += nn
			copy(d.data, d.data[nn:])
			d.data = d.data[nn:]
			d.readyOfs += int64(nn)
			continue
		}
		// then try to read from the input
		d.data, err = d.nextContiguous()
		if err != nil {
			d.err = err
		}
		if d.data == nil && err == nil {
			err = io.EOF
		}
	}
	if n == 0 && err == nil {
		//spew.Dump(d)
		panic("n==0 err==nil")
	}
	return
}

// nextContiguous reads until we get a block of data known to be contiguous with the
// last block of data.  Returns nil, nil if we need to interrupt the flow to handle a
// change in offset.
func (d *Decoder) nextContiguous() (data []byte, err error) {
	var pending []byte
	var resolv []unresolved
	d.started = true
	for len(data) == 0 {
		var lineOfs int64
		var hasOfs bool
		var label string
		lineOfs, hasOfs, data, label, err = d.scan.decodeLine()
		if err != nil {
			// no final offset means we just assume any partial data is contiguous with the prior,
			// so return that first.  A subsequent call will presumably get the same error
			// from decodeLine.
			if len(pending) > 0 {
				return pending, nil
			}
			return nil, err
		}
		if label != "" {
			resolv = append(resolv, unresolved{label, len(d.data)})
			continue
		}
		if hasOfs {
			pendOfs := lineOfs - int64(len(pending))
			for len(resolv) > 0 {
				d.labels.Set(resolv[0].label, int64(resolv[0].rel)+pendOfs)
				resolv = resolv[1:]
			}
			if pendOfs < d.readyOfs+int64(len(d.data)) {
				return nil, fmt.Errorf("file contents attempted rewind, %X < %X", pendOfs, d.readyOfs)
			}
			if !d.started || pendOfs > d.readyOfs+int64(len(d.data)) {
				d.nextData = append(pending, data...)
				d.nextOffset = pendOfs
				return nil, nil
			}
			data = append(pending, data...)
			d.readyOfs = pendOfs //lineOfs + int64(len(data))
		} else {
			pending = append(pending, data...)
			data = nil
		}
	}
	return
}

// Labels returns a container of all labels decoded from the hexdump input.
// The returned instance is live and will reflect changes as the decoding
// process occurs.  Labels will be available before calls to Read are satisfied,
// meaning this can be connected directly to a Dumper instance and labels
// will be copied as expected.
func (d *Decoder) Labels() *Labels {
	return &d.labels
}
