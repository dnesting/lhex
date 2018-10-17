package lhex_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/dnesting/lhex"
)

func init() {
}

func verify(t *testing.T, desc string, buf bytes.Buffer, expected string) {
	t.Helper()
	actual := buf.String()
	// For ease of reading/writing tests, we have an extra newline at the start and
	// don't include the final one at the end, so fix that.
	if expected != "" && expected[0] == '\n' {
		expected = expected[1:] + "\n"
	}
	if expected != actual {
		t.Errorf("%s, expected:\n%s\ngot:\n%s", desc, expected, actual)
	}
}

func TestDumper(t *testing.T) {
	data := make([]byte, 0x50)
	for i := 0; i < 0x50; i++ {
		data[i] = byte(i)
	}
	var buf bytes.Buffer

	// Test nothing
	buf.Reset()
	w := lhex.NewDumper(&buf, nil)
	w.Close()
	verify(t, "simple close", buf, "")

	// Test nothing but seek
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Seek(5, io.SeekStart)
	w.Close()
	verify(t, "simple seek", buf, "")

	// Test empty write
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Write(nil)
	w.Close()
	verify(t, "empty write", buf, `
00000000                                                    ||`)

	// Test one line
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Write(data[:0x10])
	w.Close()
	verify(t, "single line", buf, `
00000000  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|`)

	// Test multiple lines
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Write(data[:0x30])
	w.Close()
	verify(t, "multiple lines", buf, `
00000000  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
00000010  10 11 12 13 14 15 16 17  18 19 1A 1B 1C 1D 1E 1F  |................|
00000020  20 21 22 23 24 25 26 27  28 29 2A 2B 2C 2D 2E 2F  | !"#$%&'()*+,-./|`)

	// Incomplete line, starts late but finishes at the end
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Seek(5, io.SeekStart)
	w.Write(data[5:0x10])
	w.Close()
	verify(t, "late start", buf, `
                         05 06 07  08 09 0A 0B 0C 0D 0E 0F       |...........|
00000010                                                    ||`)

	// Incomplete line, finishes early
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Write(data[:0x0a])
	w.Close()
	verify(t, "early end", buf, `
00000000  00 01 02 03 04 05 06 07  08 09                    |..........|`)

	// Incomplete line, starts late AND finishes early, should force a non-multiple-of-0x10 offset.
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Seek(5, io.SeekStart)
	w.Write(data[5:0x0b])
	w.Close()
	verify(t, "late start with early end", buf, `
00000005  05 06 07 08 09 0A                                 |......|`)

	// Incomplete line, multiple writes to get there, finishes early
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Write(data[:0x05])
	w.Write(data[0x05:0x07])
	w.Write(data[0x07:0x0a])
	w.Close()
	verify(t, "early end multi-write", buf, `
00000000  00 01 02 03 04 05 06 07  08 09                    |..........|`)

	// Incomplete line sandwiched between regular ones
	buf.Reset()
	w = lhex.NewDumper(&buf, nil)
	w.Write(data[:0x10])
	w.Seek(0x104, io.SeekStart)
	w.Write(data[0x10:0x14])
	w.Seek(0x200, io.SeekStart)
	w.Write(data[0x20:0x30])
	w.Close()
	verify(t, "incomplete sandwich", buf, `
00000000  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|

00000104  10 11 12 13                                       |....|

00000200  20 21 22 23 24 25 26 27  28 29 2A 2B 2C 2D 2E 2F  | !"#$%&'()*+,-./|`)
}

func TestDumperLabels(t *testing.T) {
	data := make([]byte, 0x50)
	for i := 0; i < 0x50; i++ {
		data[i] = byte(i)
	}
	var buf bytes.Buffer
	labels := lhex.NewLabels(map[string]int64{
		"start": 0,
		"x100":  0x100,
		"x100b": 0x100,
		"x103":  0x103,
		"x105":  0x105,
	})

	// Test nothing
	buf.Reset()
	w := lhex.NewDumper(&buf, labels)
	w.Close()
	verify(t, "simple close", buf, "")

	// Test that an empty write gets a label
	buf.Reset()
	w = lhex.NewDumper(&buf, labels)
	w.Write(nil)
	w.Close()
	verify(t, "empty write", buf, `
:start
00000000                                                    ||`)

	buf.Reset()
	w = lhex.NewDumper(&buf, labels)
	w.Seek(0xF0, io.SeekStart)
	w.Write(data[:0x30])
	w.Close()
	verify(t, "some labels", buf, `
000000F0  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
:x100
:x100b
00000100  10 11 12                                          |...|
:x103
                   13 14                                       |..|
:x105
                         15 16 17  18 19 1A 1B 1C 1D 1E 1F       |...........|
00000110  20 21 22 23 24 25 26 27  28 29 2A 2B 2C 2D 2E 2F  | !"#$%&'()*+,-./|`)

	buf.Reset()
	w = lhex.NewDumper(&buf, labels)
	w.Seek(0x101, io.SeekStart)
	w.Write(data[:0x08])
	w.Close()
	verify(t, "some labels", buf, `
             00 01                                           |..|
:x103
                   02 03                                       |..|
:x105
00000105  04 05 06 07                                       |....|`)
}

func TestSeek(t *testing.T) {
	data := []byte("1BCDEFGHIJKLMNOP" + "ABCDEFGHIJKLMNOP")
	var b bytes.Buffer

	d := lhex.NewDumper(&b, nil)
	d.Write(data)
	d.Seek(0x1001, io.SeekStart)
	d.Write(data)                   // should be at 0x1021
	d.Seek(-0x0820, io.SeekCurrent) // 0x801
	d.Write(data)
	d.Close()

	expected := `
00000000  31 42 43 44 45 46 47 48  49 4A 4B 4C 4D 4E 4F 50  |1BCDEFGHIJKLMNOP|
00000010  41 42 43 44 45 46 47 48  49 4A 4B 4C 4D 4E 4F 50  |ABCDEFGHIJKLMNOP|

             31 42 43 44 45 46 47  48 49 4A 4B 4C 4D 4E 4F   |1BCDEFGHIJKLMNO|
00001010  50 41 42 43 44 45 46 47  48 49 4A 4B 4C 4D 4E 4F  |PABCDEFGHIJKLMNO|
00001020  50                                                |P|

             31 42 43 44 45 46 47  48 49 4A 4B 4C 4D 4E 4F   |1BCDEFGHIJKLMNO|
00000810  50 41 42 43 44 45 46 47  48 49 4A 4B 4C 4D 4E 4F  |PABCDEFGHIJKLMNO|
00000820  50                                                |P|
`
	actual := b.String()
	if expected[1:] != actual {
		t.Errorf("Dumper with Seek should result in\n%s\ngot:\n%s", expected, actual)
	}
}

func TestDump(t *testing.T) {
	data := []byte("ABCDEFGHIJKLMNOP" + "ABCDEFGHIJKLMNOP" + "ABCDEFGHIJKLMNOP" + "ABCDEFGHIJKLMNOP")
	expected := `                                   41 42 43 44 45 46 47 48          |ABCDEFGH|
00000020  49 4A 4B 4C 4D 4E 4F 50  41 42 43 44 45 46 47 48  |IJKLMNOPABCDEFGH|
00000030  49 4A 4B 4C 4D 4E 4F 50  41 42 43 44 45 46 47 48  |IJKLMNOPABCDEFGH|
00000040  49 4A 4B 4C 4D 4E 4F 50  41 42 43 44 45 46 47 48  |IJKLMNOPABCDEFGH|
00000050  49 4A 4B 4C 4D 4E 4F 50                           |IJKLMNOP|
`
	actual := lhex.Dump(data, 0x18, nil)
	if expected != actual {
		t.Errorf("Dump should result in\n%s\ngot:\n%s", expected, actual)
	}

	var labels lhex.Labels
	labels.Set("ignore", 5)
	labels.Set("foo", 0x33)
	labels.Set("bar", 0x35)
	labels.Set("baz", 0x35)
	expected = `                                   41 42 43 44 45 46 47 48          |ABCDEFGH|
00000020  49 4A 4B 4C 4D 4E 4F 50  41 42 43 44 45 46 47 48  |IJKLMNOPABCDEFGH|
00000030  49 4A 4B                                          |IJK|
:foo
                   4C 4D                                       |LM|
:bar
:baz
                         4E 4F 50  41 42 43 44 45 46 47 48       |NOPABCDEFGH|
00000040  49 4A 4B 4C 4D 4E 4F 50  41 42 43 44 45 46 47 48  |IJKLMNOPABCDEFGH|
00000050  49 4A 4B 4C 4D 4E 4F 50                           |IJKLMNOP|
`
	actual = lhex.Dump(data, 0x18, &labels)
	if expected != actual {
		t.Errorf("Dump with labels should result in\n%s\ngot:\n%s", expected, actual)
	}
}

func TestDumpGolden(t *testing.T) {
	data := make([]byte, 0x120)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i)
	}

	var buf bytes.Buffer
	d := lhex.NewDumper(&buf, lhex.NewLabels(map[string]int64{
		"foo": 0x40,
		"bar": 0xff000037,
	}))
	d.Seek(0x10, io.SeekStart)
	d.Write(data[0:0x40])
	d.Seek(0xFF000010, io.SeekStart)
	d.Write(data[0x40:0x57])
	d.Seek(0xFF000030, io.SeekStart)
	d.Write(data[0x57:0x77])
	d.Seek(0x7FFFFFFF00000000, io.SeekStart)
	d.Write(data[0x77:0x87])
	d.Close()

	expected := stripBlankLines(golden)
	actual := stripBlankLines(buf.String())

	if expected != actual {
		t.Errorf("failed to regenerate golden\nexpected:\n%s\ngot:\n%s", expected, actual)
	}
}

func ExampleDumper() {
	dmp := lhex.NewDumper(os.Stdout, nil)
	data := make([]byte, 0x30)
	for i := range data {
		data[i] = byte('A' + i)
	}

	dmp.Write(data[:0x20])
	dmp.Seek(0x100, io.SeekCurrent)
	dmp.Write(data[0x20:0x30])
	dmp.Close()
	// Output:
	// 00000000  41 42 43 44 45 46 47 48  49 4A 4B 4C 4D 4E 4F 50  |ABCDEFGHIJKLMNOP|
	// 00000010  51 52 53 54 55 56 57 58  59 5A 5B 5C 5D 5E 5F 60  |QRSTUVWXYZ[\]^_`|
	//
	// 00000120  61 62 63 64 65 66 67 68  69 6A 6B 6C 6D 6E 6F 70  |abcdefghijklmnop|
}

func ExampleDump() {
	labels := lhex.NewLabels(map[string]int64{
		"start": 0x10,
		"foo":   0x18,
		"end":   0x30,
	})
	data := make([]byte, 0x20)
	for i := range data {
		data[i] = byte('A' + i)
	}

	fmt.Print(lhex.Dump(data, 0x10, labels))
	// Output:
	// :start
	// 00000010  41 42 43 44 45 46 47 48                           |ABCDEFGH|
	// :foo
	//                                    49 4A 4B 4C 4D 4E 4F 50          |IJKLMNOP|
	// 00000020  51 52 53 54 55 56 57 58  59 5A 5B 5C 5D 5E 5F 60  |QRSTUVWXYZ[\]^_`|
	// :end
}
