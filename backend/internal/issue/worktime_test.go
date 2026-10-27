package issue

import "testing"

func TestDuration(t *testing.T) {
	for secs, want := range map[int64]string{0: "0m", 59: "0m", 60: "1m", 5400: "1h 30m", 8 * 3600: "1d",
		5*8*3600 + 3600: "1w 1h", 9*8*3600 + 30*60: "1w 4d 30m"} {
		if got := Duration(secs); got != want {
			t.Errorf("Duration(%d) = %q, want %q", secs, got, want)
		}
	}
}
