package lhex_test

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/dnesting/lhex"
	"github.com/dnesting/sparse"
)

func TestDecodeEmpty(t *testing.T) {
	d := lhex.NewDecoder(strings.NewReader(""))
	skip, err := d.Next()
	if skip > 0 || err != io.EOF {
		t.Errorf("read from empty decoder should result in io.EOF, got skip=%v, %v", skip, err)
	}
}

func TestDecode(t *testing.T) {
	input := `
0010  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
0020  10 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
0030  20 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 37  |................|
  `
	d := lhex.NewDecoder(strings.NewReader(input))
	skip, err := d.Next()
	if skip != 0x10 || err != nil {
		t.Fatalf("Next from decoder should give us skip 0x10 and err=nil, got %d, %v", skip, err)
	}
	data, err := ioutil.ReadAll(d)
	if len(data) != 0x30 || err != nil {
		t.Errorf("Reading should have given us the full data, got %d bytes err=%v\n%s", len(data), err, hex.Dump(data))
	}
	if data[0] != 0 || data[0x10] != 0x10 || data[0x20] != 0x20 || data[0x2F] != 0x37 {
		t.Errorf("data looks wrong\nexpected:\n%sgot:\n%s", hex.Dump([]byte(input)), hex.Dump(data))
	}
}

func TestSkipping(t *testing.T) {
	input := `
0010  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
0020  10 01 02 03 04 05 06 07  08 09 0A 0B              |............|
0040  20 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
  `
	d := lhex.NewDecoder(strings.NewReader(input))
	skip, err := d.Next()
	if skip != 0x10 || err != nil {
		t.Fatalf("Next from decoder should give us skip 0x10 and err=nil, got %d, %v", skip, err)
	}
	data, err := ioutil.ReadAll(d)
	if len(data) != 28 || err != nil {
		t.Errorf("Reading should have given us the first data set, got %d bytes err=%v\n%s", len(data), err, hex.Dump(data))
	}
	if data[0] != 0 || data[0x10] != 0x10 {
		t.Errorf("data looks wrong\nexpected first two from:\n%sgot:\n%s", input, hex.Dump(data))
	}

	skip, err = d.Next()
	if skip != 0x14 || err != nil {
		t.Fatalf("Next from decoder should give us skip 0x14 and err=nil, got %d, %v", skip, err)
	}
	data, err = ioutil.ReadAll(d)
	if len(data) != 0x10 || err != nil {
		t.Errorf("Reading should have given us the second data set, got %d bytes err=%v\n%s", len(data), err, hex.Dump(data))
	}
	if data[0] != 0x20 {
		t.Errorf("data looks wrong\nexpected last from:\n%sgot:\n%s", input, hex.Dump(data))
	}
}

func TestPartialLine(t *testing.T) {
	input := `
                               08 09 0A 0B 0C 0D 0E 0F  |................|
0020  10 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
0040  20 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
  `
	d := lhex.NewDecoder(strings.NewReader(input))
	skip, err := d.Next()
	if skip != 0x18 || err != nil {
		t.Fatalf("Next from decoder should give us skip 0x18 and err=nil, got 0x%X, %v", skip, err)
	}
	data, err := ioutil.ReadAll(d)
	if len(data) != 0x18 || err != nil {
		t.Errorf("Reading should have given us the first data set, got %d bytes err=%v\n%s", len(data), err, hex.Dump(data))
	}
	if data[0] != 8 || data[8] != 0x10 {
		t.Errorf("data looks wrong\nexpected first two from:\n%sgot:\n%s", input, hex.Dump(data))
	}

	skip, err = d.Next()
	if skip != 0x10 || err != nil {
		t.Fatalf("Next from decoder should give us skip 0x10 and err=nil, got %d, %v", skip, err)
	}
	data, err = ioutil.ReadAll(d)
	if len(data) != 0x10 || err != nil {
		t.Errorf("Reading should have given us the second data set, got %d bytes err=%v\n%s", len(data), err, hex.Dump(data))
	}
	if data[0] != 0x20 {
		t.Errorf("data looks wrong\nexpected last from:\n%sgot:\n%s", input, hex.Dump(data))
	}
}

func ExampleDecoder() {
	input := `
00000000  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
00000010  10 11 12 13 14 15 16 17  18 19 1A 1B 1C 1D 1E     |...............|

00000120  20 21 22 23 24 25 26 27  28 29 2A 2B 2C 2D 2E 2F  | !"#$%&'()*+,-./|
`
	dec := lhex.NewDecoder(strings.NewReader(input))
	var n, offset int64
	var err error

	for {
		var got []byte
		if got, err = ioutil.ReadAll(dec); err != nil {
			break
		}
		fmt.Printf("Got %d bytes at offset 0x%X\n", len(got), offset)
		offset += int64(len(got))

		if n, err = dec.Next(); err != nil {
			break
		}
		fmt.Printf("Skipped %d bytes\n", n)
		offset += n
	}
	fmt.Printf("%v", err)
	// Output:
	// Got 31 bytes at offset 0x0
	// Skipped 257 bytes
	// Got 16 bytes at offset 0x120
	// EOF
}

func ExampleDecoder_sparseBuffer() {
	// import "github.com/dnesting/sparse"

	input := `
00000000  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
00000010  10 11 12 13 14 15 16 17  18 19 1A 1B 1C 1D 1E     |...............|

00000120  20 21 22 23 24 25 26 27  28 29 2A 2B 2C 2D 2E 2F  | !"#$%&'()*+,-./|
`
	var buf sparse.Buffer
	dec := lhex.NewDecoder(strings.NewReader(input))
	sparse.Copy(&buf, dec)

	buf.Seek(0x120, io.SeekStart)
	got, _ := ioutil.ReadAll(&buf)
	fmt.Printf("Got %d bytes at offset 0x120\n", len(got))
	// Output:
	// Got 16 bytes at offset 0x120
}
