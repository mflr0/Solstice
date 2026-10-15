package api

// Board order is a string per issue, compared byte by byte: Jira Software's
// LexoRank values ("0|i0000f:") are kept as they are, and Solstice computes
// new ones between two neighbours. Generated digits stay within '0'…'z', the
// range LexoRank itself uses after its "0|" bucket prefix.

const (
	rankMin byte = '0'
	rankMax byte = 'z'
)

// rankBetween returns a string that sorts strictly between a and b. An
// empty a means "before everything", an empty b "after everything". It
// requires a < b when both are set.
func rankBetween(a, b string) string {
	var out []byte
	upperOpen := b == ""
	for i := 0; ; i++ {
		lo := rankMin - 1 // a has ended: anything goes from here on
		if i < len(a) {
			lo = a[i]
		}
		hi := rankMax + 1
		if !upperOpen && i < len(b) {
			hi = b[i]
		}
		switch {
		case lo == hi:
			out = append(out, lo)
		case hi-lo > 1:
			mid := lo + (hi-lo)/2
			out = append(out, mid)
			if mid == rankMin {
				// Never end on the smallest digit: nothing could ever be
				// placed between a and the result. Keep one more digit.
				upperOpen = true
				continue
			}
			return string(out)
		default:
			// Adjacent digits: keep a's digit and look further, with no
			// upper bound any more (we are already below b).
			out = append(out, lo)
			upperOpen = true
		}
	}
}
