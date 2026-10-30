package crawler

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strings"
)

type Fingerprint struct {
	HTMLHash    string `json:"html_hash"`
	Title       string `json:"title"`
	FaviconURL  string `json:"favicon_url"`
	ScriptsHash string `json:"scripts_hash"`
}

func GenerateFingerprint(html string) *Fingerprint {
	// 1. HTML Hash (Simple MD5 of content)
	hash := md5.Sum([]byte(html))
	htmlHash := hex.EncodeToString(hash[:])

	// 2. Extract Title
	title := ""
	reTitle := regexp.MustCompile(`(?i)<title>(.*?)</title>`)
	matchTitle := reTitle.FindStringSubmatch(html)
	if len(matchTitle) > 1 {
		title = strings.TrimSpace(matchTitle[1])
	}

	// 3. Extract Favicon URL
	favicon := ""
	reFav := regexp.MustCompile(`(?i)<link.*?rel=["'](?:shortcut )?icon["'].*?href=["'](.*?)["']`)
	matchFav := reFav.FindStringSubmatch(html)
	if len(matchFav) > 1 {
		favicon = matchFav[1]
	}

	// 4. Extract Script tags as a fingerprint
	reScript := regexp.MustCompile(`(?i)<script.*?src=["'](.*?)["']`)
	matches := reScript.FindAllStringSubmatch(html, -1)
	scriptSources := ""
	for _, m := range matches {
		scriptSources += m[1] + ","
	}
	sHash := md5.Sum([]byte(scriptSources))
	scriptsHash := hex.EncodeToString(sHash[:])

	return &Fingerprint{
		HTMLHash:    htmlHash,
		Title:       title,
		FaviconURL:  favicon,
		ScriptsHash: scriptsHash,
	}
}
