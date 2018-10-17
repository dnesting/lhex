package lhex_test

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/dnesting/lhex"
	"github.com/dnesting/sparse"
)

// golden comes directly from the package docs.
const golden = `
# Comments start with '#' and blank lines are ignored.

00000010  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|
00000020  10 11 12 13 14 15 16 17  18 19 1A 1B 1C 1D 1E 1F  |................|
00000030  20 21 22 23 24 25 26 27  28 29 2A 2B 2C 2D 2E 2F  | !"#$%&'()*+,-./|

# Locations in the file can be marked with labels of the style :labelname
:foo
00000040  30 31 32 33 34 35 36 37  38 39 3A 3B 3C 3D 3E 3F  |0123456789:;<=>?|

# The file can contain gaps in the data, where the offset skips ahead.
FF000010  40 41 42 43 44 45 46 47  48 49 4A 4B 4C 4D 4E 4F  |@ABCDEFGHIJKLMNO|

# Partial lines are OK too.  If the line has an offset, the missing bytes are
# taken to be at the end of the line.  FF000027 thru 2F are missing.
FF000020  50 51 52 53 54 55 56                              |PQRSTUV|

# The next line is split to accommodate a label.  If a line doesn't start with
# an offset, the first offset following is used to work out where these bytes
# are.  In this case, the next two lines are contiguous, so there's no gap
# within FF000030-3F.
FF000030  57 58 59 5A 5B 5C 5D                              |WXYZ[\]|
:bar
                               5E  5F 60 61 62 63 64 65 66         |^_` + "`" + `abcdef|
FF000040  67 68 69 6A 6B 6C 6D 6E  6F 70 71 72 73 74 75 76  |ghijklmnopqrstuv|

# Offsets can be up to 63 bits long.
7FFFFFFF00000000  77 78 79 7A 7B 7C 7D 7E  7F 80 81 82 83 84 85 86  |wxyz{.}~........|
`

func stripBlankLines(s string) string {
	var w bytes.Buffer
	r := bufio.NewReader(bytes.NewReader([]byte(s)))
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if len(line) == 0 || line[0] == '#' || line == "\n" {
			continue
		}
		w.WriteString(line)
	}
	return w.String()
}

func TestRoundTrip(t *testing.T) {
	orig := bytes.NewBufferString(golden)
	decoder := lhex.NewDecoder(orig)
	var dest bytes.Buffer
	dumper := lhex.NewDumper(&dest, decoder.Labels())

	sparse.Copy(dumper, decoder)

	expected := stripBlankLines(golden)
	actual := stripBlankLines(dest.String())
	if expected != actual {
		t.Errorf("round-trip failed, expected:\n%sactual:\n%s", expected, actual)
	}
}
