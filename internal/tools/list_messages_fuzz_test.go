package tools

import (
	"math"
	"testing"
)

func FuzzPageRange(f *testing.F) {
	// Seeds from existing TestPageRange table.
	f.Add(uint32(250), 1, 100)
	f.Add(uint32(250), 2, 100)
	f.Add(uint32(250), 3, 100)
	f.Add(uint32(1), 1, 100)
	f.Add(uint32(100), 1, 100)
	f.Add(uint32(50), 2, 100)
	f.Add(uint32(50), 0, 100)
	f.Add(uint32(50), -1, 100)

	// Boundary seeds.
	f.Add(uint32(0), 1, 100)
	f.Add(uint32(math.MaxUint32), 1, 100)
	f.Add(uint32(100), math.MaxInt, 100)
	f.Add(uint32(100), 1, 0)
	f.Add(uint32(100), 1, 1)
	f.Add(uint32(100), 1, math.MaxInt)

	f.Fuzz(func(
		t *testing.T,
		total uint32,
		page, pageSz int,
	) {
		lo, hi, totalPages, err := pageRange(
			total, page, pageSz,
		)
		// Must never panic (implicit).
		if err != nil {
			return
		}

		// 1 <= lo <= hi <= total
		if lo < 1 {
			t.Errorf("lo=%d < 1", lo)
		}
		if hi < lo {
			t.Errorf("hi=%d < lo=%d", hi, lo)
		}
		if hi > total {
			t.Errorf(
				"hi=%d > total=%d",
				hi, total,
			)
		}

		// Page never exceeds page size.
		span := hi - lo + 1
		if span > uint32(pageSz) {
			t.Errorf(
				"span=%d > pageSz=%d",
				span, pageSz,
			)
		}

		// totalPages >= 1
		if totalPages < 1 {
			t.Errorf(
				"totalPages=%d < 1",
				totalPages,
			)
		}

		// page <= totalPages
		if page > totalPages {
			t.Errorf(
				"page=%d > totalPages=%d",
				page, totalPages,
			)
		}
	})
}
