package safefs

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// Tier is one way to put a source file at a destination.
type Tier int

// The tiers, cheapest sharing first.
const (
	TierClone    Tier = 1
	TierHardlink Tier = 2
	TierSymlink  Tier = 3
	TierCopy     Tier = 4
)

// Materializer puts files at a destination by the cheapest sharing the destination allows, falling through its
// tiers on LinkUnsupported errors. It remembers per destination device which tiers that device refused; that memory
// is process-wide, shared by every Materializer.
type Materializer struct {
	// Tiers is the order to try; zero means clone, hardlink, symlink, copy. Copy is the tier that never fails over.
	Tiers []Tier
	// Shareable reports whether rel may share an inode or be a link; false keeps it off the hardlink and symlink
	// tiers. rel is slash-separated on every OS, so one policy reads the same paths everywhere. Nil means every
	// file.
	Shareable func(rel string) bool
	// Copy is the options of the copy tier.
	Copy CopyOptions
}

// tierOps is the function behind each tier; tests replace them.
type tierOps struct {
	clone, hardlink, symlink, copy func(src, dst string) error
}

// capSlots has one slot per tier; the copy slot is never written.
type capSlots [TierCopy]atomic.Int32

// capCache maps a destination device (uint64) to the *capSlots that device's refusals are recorded in.
var capCache sync.Map

func (m Materializer) defaultOps() tierOps {
	return tierOps{
		clone:    Clone,
		hardlink: os.Link,
		symlink: func(src, dst string) error {
			abs, err := filepath.Abs(src)
			if err != nil {
				return err
			}
			return os.Symlink(abs, dst)
		},
		copy: func(src, dst string) error { return CopyFile(src, dst, m.Copy) },
	}
}

func (o tierOps) fn(t Tier) func(src, dst string) error {
	switch t {
	case TierClone:
		return o.clone
	case TierHardlink:
		return o.hardlink
	case TierSymlink:
		return o.symlink
	case TierCopy:
		return o.copy
	default:
		return nil
	}
}

// Tree puts src's files into dst.
func (m Materializer) Tree(src, dst string) error {
	_, err := CopyTree(src, dst, CopyOptions{Put: m.File})
	return err
}

// File puts the one file src at dst. rel is its path inside the tree, and decides Shareable.
func (m Materializer) File(src, dst, rel string) error {
	return m.put(m.defaultOps(), true, src, dst, rel)
}

func (m Materializer) put(ops tierOps, useCache bool, src, dst, rel string) error {
	shareable := m.Shareable == nil || m.Shareable(filepath.ToSlash(rel))
	srcDev, srcErr := Device(src)
	dstDev, dstErr := Device(filepath.Dir(dst))
	sameDev := srcErr == nil && dstErr == nil && srcDev == dstDev
	var caps *capSlots
	if useCache && dstErr == nil {
		v, _ := capCache.LoadOrStore(dstDev, &capSlots{})
		caps, _ = v.(*capSlots)
	}
	tiers := m.Tiers
	if len(tiers) == 0 {
		tiers = []Tier{TierClone, TierHardlink, TierSymlink, TierCopy}
	}
	for _, t := range tiers {
		switch t {
		case TierHardlink, TierSymlink:
			if !shareable {
				continue
			}
		case TierClone, TierCopy:
		default:
			continue
		}
		if !sameDev && (t == TierClone || t == TierHardlink) {
			continue
		}
		if caps != nil && t != TierCopy && caps[t-1].Load() < 0 {
			continue
		}
		fn := ops.fn(t)
		if fn == nil {
			continue
		}
		err := fn(src, dst)
		if err == nil {
			if caps != nil && t != TierCopy {
				caps[t-1].Store(1)
			}
			return nil
		}
		if !LinkUnsupported(err) {
			return err
		}
		if caps != nil && t != TierCopy && !isCrossDevice(err) {
			caps[t-1].Store(-1)
		}
	}
	return ops.copy(src, dst)
}
