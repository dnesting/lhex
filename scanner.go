package lhex

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
)

type scanner struct {
	rd   *bufio.Reader
	line []byte
	ch   byte
	off  int
	eol  bool
}

func newScanner(r io.Reader) *scanner {
	return &scanner{
		rd: bufio.NewReader(r),
	}
}

func (d *scanner) next() {
	if d.off < len(d.line)-1 {
		d.off++
		d.ch = d.line[d.off]
		d.eol = d.ch == '\n'
	} else {
		d.ch = 0
		d.eol = true
	}
	/*
		if d.eol {
			gotrace.Log("EOL")
		} else {
			gotrace.Log("%q", d.ch)
		}
	*/
}

func (d *scanner) rewind(i int) {
	if i > 0 && i == d.off {
		return
	}
	//gotrace.Log("rewind(%d)", i)
	if i < 0 {
		panic("cannot rewind line before 0")
	}
	d.off = i - 1
	d.eol = false
	d.next()
}

// decodeLine reads and decodes a single line.  Returns io.EOF if no data was read.
func (d *scanner) decodeLine() (offset int64, hasOffset bool, data []byte, label string, err error) {
	//defer gotrace.In("decodeLine")()
	d.line, err = d.rd.ReadBytes('\n')
	if err != nil {
		if err != io.EOF || len(d.line) == 0 {
			//gotrace.Log(err.Error())
			return 0, false, nil, "", err
		}
	}
	d.rewind(0)
	return d.scanLine()
}

func (d *scanner) scanLine() (offset int64, hasOffset bool, data []byte, label string, err error) {
	//defer gotrace.In("scanLine")()
	if isHex(d.ch) {
		if offset, hasOffset, err = d.decodeOffset(); err != nil {
			return
		}
		//gotrace.Log("= offset %v %X", hasOffset, offset)
	} else if d.ch == ':' {
		d.next()
		label, err = d.decodeLabel()
		return
	}

	d.skipSpacesOrHyphen()
	if isHex(d.ch) {
		data = make([]byte, 16)
		var i int
		for i = 0; i < len(data); i++ {
			if _, err = d.decodeHexBytes(data[i : i+1]); err != nil {
				return
			}
			//gotrace.Log("= char %s", hex.EncodeToString(data[i:i+1]))
			d.skipSpacesOrHyphen()
			if !isHex(d.ch) {
				//gotrace.Log("done looking for hex chars, !hex(%c)", d.ch)
				break
			}
		}
		data = data[:i+1]
	}

	return
}

func (d *scanner) decodeLabel() (label string, err error) {
	var notFirst bool
	start := d.off
	for isLabel(d.ch, notFirst) {
		d.next()
	}
	label = string(d.line[start:d.off])
	d.skipSpaces()
	d.skipComment()
	if !d.eol {
		err = fmt.Errorf("illegal text after label: %q", d.line[d.off:])
	}
	return
}

func (d *scanner) skipSpaces() {
	for d.ch == ' ' {
		d.next()
	}
}

func (d *scanner) skipSpacesOrHyphen() {
	for d.ch == ' ' || d.ch == '-' {
		d.next()
	}
}

func (d *scanner) skipComment() {
	if d.ch == '#' {
		d.eol = true // just pretend we're at the end of the line
	}
}

func (d *scanner) decodeHexBytes(buf []byte) (n int, err error) {
	start := d.off
	for isHex(d.ch) {
		d.next()
	}
	n, err = hex.Decode(buf, d.line[start:d.off])
	d.rewind(start + n*2)
	if isHex(d.ch) {
		err = fmt.Errorf("too many characters reading hex string: %q", d.line[start:d.off+1])
	} else if !d.eol && d.ch != ' ' && d.ch != '-' {
		err = fmt.Errorf("illegal character %q reading hex string: %q", d.ch, d.line[start:d.off+1])
	}
	return
}

func rightAlign(data []byte) []byte {
	i := cap(data) - len(data)
	data = data[:cap(data)]
	if i > 0 {
		copy(data[i:], data)
		for i--; i >= 0; i-- {
			data[i] = 0
		}
	}
	return data
}

func (d *scanner) decodeOffset() (offset int64, hasOffset bool, err error) {
	data := make([]byte, 8)
	n, err := d.decodeHexBytes(data)
	if err != nil {
		return 0, false, err
	}
	data = rightAlign(data[:n])
	offset = int64(binary.BigEndian.Uint64(data))
	hasOffset = true
	if offset < 0 {
		return 0, false, fmt.Errorf("offset too large: %X", hex.EncodeToString(data))
	}
	return
}

/*
func (f *File) parse(input []byte, offset int64) error {
	s := Scanner{data: input}
	s.next()
	var data []byte
	type ur struct {
		label  string
		behind int
	}
	var unresolved []ur
	for !s.eof {
		ofsstr, d, lab, err := s.Line()
		if err != nil {
			return err
		}

		// Set aside labels for now, and note how many bytes "behind" we are if we're still waiting
		// to get an offset.
		for _, l := range lab {
			unresolved = append(unresolved, ur{l, len(data)})
		}

		// If an offset was given, parse it now and rewind a bit if we've been accumulating bytes
		// without an offset before this line.
		ofs := int64(-1)
		if len(ofsstr) > 0 {
			ofs = int64(binary.BigEndian.Uint64(ofsstr))
		}
		if ofs >= 0 {
			ofs -= int64(len(data))
		}
		if len(d) > 0 {
			data = append(data, d...)
		}

		// Only emit data when we have it, and we know where we are.
		if len(data) > 0 && ofs >= 0 {
			f.WriteAt(data, ofs+offset)
			for _, l := range unresolved {
				// We have an offset now, so we can interpret any labels provided just before.
				f.AddLabel(l.label, ofs+int64(l.behind))
			}
			data = data[:0]
			unresolved = unresolved[:0]
		}
	}
	return nil
}

// Label retrieves the most recently defined offset associated with the label. If the label was
// not found, ok will be false.
func (f *File) Label(s string) (off int64, ok bool) {
	off, ok = f.labels[s]
	return
}

// Labels returns all of the label->offset mappings.
func (f *File) Labels() map[string]int64 {
	return f.labels
}
*/

func isHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'F'
}

func isLabel(b byte, notFirst bool) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '_' || b == '-' || notFirst && b >= '0' && b <= '9'
}
