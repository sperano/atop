package board

import (
	"fmt"
	"time"

	"github.com/sperano/vigie/internal/source"
)

// unknownAge is the age of a row whose time is unknown.
const unknownAge = "-"

var ageUnits = []struct {
	suffix string
	size   time.Duration
}{
	{"d", source.Day},
	{"h", time.Hour},
	{"m", time.Minute},
	{"s", time.Second},
}

// FormatAge renders the time since `since` in its largest whole unit.
func FormatAge(since, now time.Time) string {
	if since.IsZero() {
		return unknownAge
	}
	elapsed := max(now.Sub(since), 0)
	for _, u := range ageUnits {
		if elapsed >= u.size {
			return fmt.Sprintf("%d%s", elapsed/u.size, u.suffix)
		}
	}
	return "0" + ageUnits[len(ageUnits)-1].suffix
}
