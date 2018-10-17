package lhex

import "sort"

// Labels provides a mapping from label name to offset.
type Labels struct {
	lmap map[string]int64 // the actual label data

	// cached derivatives
	offLabels map[int64][]string
	offsets   []int64
}

// NewLabels creates a *Labels instance from the contents of lmap.  Future
// changes to lmap will result in undefined behavior.
func NewLabels(lmap map[string]int64) *Labels {
	var labs Labels
	labs.Reset(lmap)
	return &labs
}

// Get retrieves the label offset for name.  If the label does not exist, ok
// will be false.
func (l Labels) Get(name string) (ofs int64, ok bool) {
	ofs, ok = l.lmap[name]
	return
}

// init ensures we're in a writable state.
func (l *Labels) init() {
	if l.lmap == nil {
		l.lmap = make(map[string]int64)
		l.offLabels = make(map[int64][]string)
	}
}

// Set sets the label name to have the offset ofs.
func (l *Labels) Set(name string, ofs int64) {
	l.init()
	if o, ok := l.lmap[name]; ok {
		if o != ofs {
			// modifying an existing assignment, just re-generate the derived fields
			l.lmap[name] = ofs
			l.Reset(l.lmap)
		}
	} else {
		// new assignment
		l.lmap[name] = ofs
		if _, ok := l.offLabels[ofs]; !ok {
			l.offsets = append(l.offsets, ofs)
			l.sortOffsets()
		}
		l.offLabels[ofs] = append(l.offLabels[ofs], name)
		sort.Strings(l.offLabels[ofs])
	}
}

// Reset reset the Labels instance to use the labels from labels instead.
func (l *Labels) Reset(labels map[string]int64) {
	l.lmap = labels
	l.offLabels = make(map[int64][]string)
	l.offsets = nil
	for name, off := range labels {
		l.offLabels[off] = append(l.offLabels[off], name)
	}
	for off := range l.offLabels {
		sort.Strings(l.offLabels[off])
	}
	for off := range l.offLabels {
		l.offsets = append(l.offsets, off)
	}
	l.sortOffsets()
}

// All returns all of the label to offset mappings.  This map is live and
// may be modified during further decoding.  Caller changes to this map will
// result in undefined behavior.
func (l *Labels) All() map[string]int64 {
	return l.lmap
}

func (l *Labels) sortOffsets() {
	sort.Slice(l.offsets, func(a, b int) bool { return l.offsets[a] < l.offsets[b] })
}

// iter creates an iterator on labels, starting at or after ofs.  Changes to labels
// may not be reflected in the resulting LabelIter.
func (l *Labels) iter(ofs int64) *labelIter {
	var it labelIter
	if l != nil {
		it.offLabels = l.offLabels
		it.iter = l.offsets
	}
	for it.Next() {
		if it.Ofs >= ofs {
			break
		}
	}
	return &it
}

// labelIter is an iterator on label offsets.
type labelIter struct {
	// Ofs is the offset of the next label set.  If no more labels exist, this will be <0.
	Ofs int64
	// Labels contains the labels at Ofs.  If no more labels exist, this will be nil.
	Labels []string

	offLabels map[int64][]string
	iter      []int64
}

// Next advances to the next offset that has labels.  If no more labels exist,
// returns false.
func (it *labelIter) Next() (ok bool) {
	if len(it.iter) > 0 {
		it.Ofs = it.iter[0]
		it.Labels = it.offLabels[it.Ofs]
		ok = true
		it.iter = it.iter[1:]
	} else {
		it.Ofs = -1
		it.Labels = nil
	}
	return
}
