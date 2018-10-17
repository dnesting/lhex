package lhex

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/dnesting/gotrace"
)

// dataBuf separates out some important logic to guarantee one data-centric idea about what the
// current offset is, and disallow an attempt to change it inappropriately.  Not honoring these
// rules leads to bugs.
type dataBuf struct {
	ofs  int64
	data [16]byte
	have int
}

// fill tries to bring len(data) up to want by copying bytes from p.
func (b *dataBuf) fill(p []byte, want int) (n int) {
	if want > len(b.data) {
		panic("want more than data capacity")
	}
	if want < 0 {
		panic("want negative")
	}
	n = copy(b.data[b.have:want], p)
	b.have += n
	return n
}

// take empties the buffer, returns its contents, and advances the canonical offset by len(data).
func (b *dataBuf) take() (ofs int64, data []byte) {
	ofs = b.ofs
	data = b.data[:b.have]
	b.ofs += int64(b.have)
	b.have = 0
	return
}

// set moves the current data offset to ofs. It is only appropriate to do this when the data
// buffer is empty, so panic if this is not the case.
func (b *dataBuf) set(ofs int64) {
	if b.have > 0 {
		panic(fmt.Sprintf("cannot set 0x%X when we have %d bytes of data at 0x%X", ofs, b.have, b.ofs))
	}
	b.ofs = ofs
}

// Dumper accepts Writes and emits a hex dump of the written bytes to a provided writer, with
// optional labels at offsets in the data.  This type also has a Seek method that can be used to
// change the offset reported in the hex dump.
type Dumper struct {
	closed bool // Close() was called
	w      io.Writer

	nextOff       int64 // Seek request that gets honored during Write via honorSeekIfNeeded.
	writePending  bool  // Write was called, which implies intent to write something
	wroteAnything bool  // Once we start writing, we start emitting blank lines between sections.

	labels    *Labels
	labelIter *labelIter
	data      dataBuf
}

// NewDumper creates a Dumper writing to w, optionally writing labels where appropriate.
func NewDumper(w io.Writer, labels *Labels) *Dumper {
	gotrace.Log("NewDumper(%v)", labels)
	return &Dumper{w: w, labels: labels, labelIter: labels.iter(0)}
}

// If the current offset is the same as the next label, write any labels pointing to this
// offset, ordered by label.
func (d *Dumper) writeLabelsIfNeeded() {
	if d.data.ofs == d.labelIter.Ofs {
		for _, l := range d.labelIter.Labels {
			fmt.Fprintf(d.w, ":%s\n", l)
		}
		d.labelIter.Next()
	}
}

// If Seek was previously called, flush any pending data, emit a newline if needed, and
// move us ahead to the seeked offset.
func (d *Dumper) honorSeekIfNeeded() {
	if d.data.ofs != d.nextOff {
		d.wrapUp()
		if d.wroteAnything {
			fmt.Fprintln(d.w)
		}
		d.data.set(d.nextOff)
		d.labelIter = d.labels.iter(d.nextOff)
	}
}

// Write continues writing a hex dump using input from p to the wrapped writer. This may trigger the
// writing of any labels at the current offset (a Write with an empty or nil p might be appropriate
// if you don't want to write any data with it).  Returns the number of bytes consumed from p and
// whether an error was encountered writing the hex dump to the wrapped writer.
func (d *Dumper) Write(p []byte) (n int, err error) {
	defer gotrace.In("Write(%q)", p)()
	// Check that we've encountered a Seek, and if so, finish up any previous segment before
	// moving on.
	d.honorSeekIfNeeded()

	// Mark that a Write occurred to ensure *something* gets written if we encounter a Close
	// or a Seek without data being written otherwise.  This enables empty writes to nevertheless
	// trigger writing out labels attached to the offset.
	d.writePending = true

	for n < len(p) {
		// Aim to complete a full line of 0x10 bytes, less if the offset starts mid-way into the
		// line, and less if we have to break the line in order to get a label written.
		want := 0x10 - int(d.data.ofs%0x10)
		if d.labelIter.Ofs > 0 && d.data.ofs+int64(want) > d.labelIter.Ofs {
			want = int(d.labelIter.Ofs - d.data.ofs)
		}

		// If we're short, try to get more from p.
		n += d.data.fill(p[n:], want)
		gotrace.Log("have so far %q", p[:n])

		// We should always have <= want bytes at this point.  If we didn't get enough bytes from
		// p to satisfy our want, that implies we're at n==len(p) and we'll leave it to a future
		// Write or Close call to finish the line up.
		if d.data.have == want {
			gotrace.Log("== want=%d", want)
			d.writePending = d.data.ofs%0x10 > 0 // no offset this time, so ensure we write one later
			d.writeLabelsIfNeeded()
			if d.data.have > 0 {
				d.writeLine(false)
			}
			d.wroteAnything = true // used by honorSeekIfNeeded to emit a blank line
		}
	}
	return
}

// wrapUp is called when we need to honor a seek, or when Close is called, to finish any
// pending lines.
func (d *Dumper) wrapUp() {
	if d.writePending || d.data.have > 0 {
		d.writeLabelsIfNeeded()
		d.writeLine(true) // force writing an offset because there will not be a following line with one
	}
}

// writeLine emits one line of data, draining d.data in the process.  If the offset is not
// a multiple of 0x10 (and forceOffset is false), the offset will be skipped and should be
// inferred from the offset of the next line.
func (d *Dumper) writeLine(forceOffset bool) (err error) {
	ofs, buf := d.data.take()
	skipLeft := int(ofs % 0x10)
	skipRight := 0x10 - (len(buf) + skipLeft)

	var sb bytes.Buffer // accumulate the line here and we'll Write it all at once

	// Normally if ofs isn't a multiple of 0x10 we skip writing the offset, because a following line
	// should give us an offset instead.  But after a Seek or a Close, we won't get that chance and
	// have to emit an offset whether we want to or not.  In this case, the offset will not be a
	// multiple of 0x10, and so it's inappropriate to have a gap between the start of the line at the
	// first byte.
	if forceOffset && skipLeft > 0 {
		skipRight += skipLeft
		skipLeft = 0
	}

	// 00000010  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|

	// Part 1: Offset
	if skipLeft == 0 || forceOffset {
		fmt.Fprintf(&sb, "%08X  ", ofs)
	} else {
		fmt.Fprintf(&sb, "%8s  ", "")
	}

	// Part 2: Hex values
	for i := 0; i < skipLeft; i++ {
		fmt.Fprint(&sb, "   ")
		if i == 7 {
			fmt.Fprint(&sb, " ") // extra space mid-way through
		}
	}
	for i, b := range buf {
		fmt.Fprintf(&sb, "%02X ", b)
		if i+skipLeft == 7 {
			fmt.Fprint(&sb, " ")
		}
	}
	for i := 0; i < skipRight; i++ {
		fmt.Fprint(&sb, "   ")
		if i+len(buf)+skipLeft == 7 {
			fmt.Fprint(&sb, " ")
		}
	}

	// Part 3: Printable characters
	for i := 0; i < skipLeft; i++ {
		fmt.Fprint(&sb, " ")
	}
	fmt.Fprint(&sb, " |")
	for _, b := range buf {
		if strconv.IsPrint(rune(b)) && b != '|' {
			fmt.Fprintf(&sb, "%c", b)
		} else {
			fmt.Fprint(&sb, ".")
		}
	}
	fmt.Fprintln(&sb, "|")

	// Write the completed line to d.w.
	_, err = d.w.Write(sb.Bytes())
	return
}

// Seek changes the offset in the hex dump to ofs.  Whence can be set to
// io.SeekStart, io.SeekCurrent, or io.SeekEnd to change how ofs is interpreted.
// io.SeekCurrent is the same as io.SeekEnd.  A Write is necessary to actually emit
// a line with the new offset.  It's OK to call Write(nil) if your goal is just to
// get labels emitted at ofs without emitting any actual data.
func (d *Dumper) Seek(ofs int64, whence int) (n int64, err error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent, io.SeekEnd:
		ofs = ofs + d.data.ofs + int64(d.data.have)
	default:
		return d.data.ofs, errors.New("invalid whence")
	}
	if ofs < 0 {
		return d.data.ofs, errors.New("seek offset must not be negative")
	}

	// For now, just record the last seek offset.  This won't actually do anything until a
	// future write.
	d.nextOff = ofs
	return d.nextOff, nil
}

// Close finishes writing any partial hex dump line.  This does not close the underlying
// writer.
func (d *Dumper) Close() (err error) {
	d.wrapUp()
	if d.wroteAnything {
		d.writeLabelsIfNeeded() // any lingering labels pointing to the end of the data
	}
	d.closed = true
	return nil
}

// Dump returns a hex dump of data, with optional labels.
func Dump(data []byte, offset int64, labels *Labels) string {
	var wr bytes.Buffer
	rd := bytes.NewBuffer(data)
	dmp := NewDumper(&wr, labels)
	dmp.Seek(offset, io.SeekStart)
	io.Copy(dmp, rd)
	dmp.Close()
	return wr.String()
}
