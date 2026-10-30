package crawler

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
)

var (
	reTagWithClass = regexp.MustCompile(`(?i)<([a-z][a-z0-9]*)[^>]*class=["']([^"']*)["']`)
	reTagWithID    = regexp.MustCompile(`(?i)<([a-z][a-z0-9]*)[^>]*\bid=["']([^"']*)["']`)
	reAnyTag       = regexp.MustCompile(`(?i)<([a-z][a-z0-9]*)[\s/>]`)
)

// ComputeSimHash menghitung 64-bit fuzzy hash dari struktur DOM.
// Dua halaman dengan template yang sama akan menghasilkan hash dengan
// Hamming distance yang kecil, meskipun kontennya berbeda.
func ComputeSimHash(html string) string {
	features := extractStructuralFeatures(html)
	if len(features) == 0 {
		return ""
	}

	var v [64]int

	for _, feature := range features {
		h := fnv64a(feature)
		for i := 0; i < 64; i++ {
			if h&(1<<uint(i)) != 0 {
				v[i]++
			} else {
				v[i]--
			}
		}
	}

	var result uint64
	for i := 0; i < 64; i++ {
		if v[i] > 0 {
			result |= 1 << uint(i)
		}
	}

	return fmt.Sprintf("%016x", result)
}

// HammingDistance menghitung jarak antara dua SimHash.
// 0 = identik, ≤ 8 = template sangat mirip, ≤ 16 = mirip.
func HammingDistance(a, b string) int {
	if len(a) != 16 || len(b) != 16 {
		return 64
	}
	ua, err1 := strconv.ParseUint(a, 16, 64)
	ub, err2 := strconv.ParseUint(b, 16, 64)
	if err1 != nil || err2 != nil {
		return 64
	}
	xor := ua ^ ub
	return popcount(xor)
}

// IsSimilarTemplate returns true jika dua SimHash punya Hamming distance ≤ threshold.
func IsSimilarTemplate(a, b string, threshold int) bool {
	return HammingDistance(a, b) <= threshold
}

func extractStructuralFeatures(html string) []string {
	var features []string

	// tag.class combinations — paling kuat untuk template detection
	for _, m := range reTagWithClass.FindAllStringSubmatch(html, -1) {
		tag := strings.ToLower(m[1])
		for _, cls := range strings.Fields(strings.ToLower(m[2])) {
			if len(cls) > 1 && len(cls) < 40 {
				features = append(features, tag+"."+cls)
			}
		}
	}

	// tag#id combinations
	for _, m := range reTagWithID.FindAllStringSubmatch(html, -1) {
		tag := strings.ToLower(m[1])
		id := strings.ToLower(strings.TrimSpace(m[2]))
		if len(id) > 1 && len(id) < 40 {
			features = append(features, tag+"#"+id)
		}
	}

	// Frekuensi tag — kontribusi minor
	tagCount := make(map[string]int)
	for _, m := range reAnyTag.FindAllStringSubmatch(html, -1) {
		tagCount[strings.ToLower(m[1])]++
	}
	for tag, count := range tagCount {
		// Tambah sesuai frekuensi (bukan per-occurrence) agar tidak overwhelm
		for i := 0; i < min(count/10+1, 5); i++ {
			features = append(features, "tag:"+tag)
		}
	}

	return features
}

func fnv64a(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func popcount(x uint64) int {
	count := 0
	for x != 0 {
		count += int(x & 1)
		x >>= 1
	}
	return count
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
