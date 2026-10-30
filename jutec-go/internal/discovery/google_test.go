package discovery

import "testing"

func TestIsInstitutionalHost(t *testing.T) {
	yes := []string{"dinkes.jatengprov.go.id", "fkip.uns.ac.id", "sman1.sch.id", "sukamaju.desa.id",
		"wmo.int", "journals.nubip.edu.ua", "cs.ox.ac.uk", "mit.edu", "data.gov"}
	no := []string{"tripslot.net", "indo303a.site", "go.com", "abc.go.com", "slot88.id", "academy.com", "edu-slot.xyz"}
	for _, h := range yes {
		if !IsInstitutionalHost(h) {
			t.Errorf("%s seharusnya institusi", h)
		}
	}
	for _, h := range no {
		if IsInstitutionalHost(h) {
			t.Errorf("%s seharusnya bukan institusi", h)
		}
	}
}

func TestCleanURLsKeepsInjectedInstitutionalPages(t *testing.T) {
	s := &GoogleScraper{}
	got := s.cleanURLs([]string{
		"https://tripslot.net/slot/gacor?ref=1",           // situs biasa → dipangkas ke domain
		"https://tripslot.net/",                           // duplikat domain
		"https://fkip.uns.ac.id/wp-content/slot-gacor/",   // halaman susupan → URL utuh
		"https://fkip.uns.ac.id/",                         // beranda institusi → domain
		"https://wmo.int/news/slot?p=2",                   // institusi internasional → URL utuh + query
		"https://www.youtube.com/watch?v=x",               // diabaikan
	})
	want := []string{"https://tripslot.net", "https://fkip.uns.ac.id/wp-content/slot-gacor/", "https://fkip.uns.ac.id",
		"https://wmo.int/news/slot?p=2"}
	if len(got) != len(want) {
		t.Fatalf("dapat %v, mau %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] dapat %s, mau %s", i, got[i], want[i])
		}
	}
}
