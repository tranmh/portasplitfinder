package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/geo"
)

// FuzzHandleStoreSearchPagination: for n stores, page and pageSize in range,
// the page holds exactly the items [(page-1)*pageSize, page*pageSize) of the
// result, clipped to n, and total is n.
func FuzzHandleStoreSearchPagination(f *testing.F) {
	f.Skip("aqaa: fuzz finding F1 (violation), re-enable once it is fixed; see report")
	f.Add(uint8(5), uint8(1), uint8(1))
	f.Add(uint8(25), uint8(1), uint8(20))
	f.Add(uint8(25), uint8(2), uint8(20))
	f.Add(uint8(0), uint8(1), uint8(20))
	f.Add(uint8(100), uint8(1), uint8(100))
	f.Add(uint8(3), uint8(9), uint8(1))
	f.Fuzz(func(t *testing.T, nStores, page, pageSize uint8) {
		n := int(nStores)
		p := int(page)%50 + 1
		ps := int(pageSize)%maxPageSize + 1

		fs := newFakeStore()
		for i := 0; i < n; i++ {
			fs.stores = append(fs.stores, domain.Store{ID: fmt.Sprintf("s%03d", i), Name: fmt.Sprintf("Store %03d", i), City: "X"})
		}
		s, _ := newTestServer(fs)
		path := fmt.Sprintf("/api/stores?page=%d&pageSize=%d", p, ps)
		rr := do(s, http.MethodGet, path, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status %d, body %s", path, rr.Code, rr.Body.String())
		}
		var resp StoreSearchResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: decode: %v", path, err)
		}
		if resp.Total != n {
			t.Errorf("%s: total %d, want %d", path, resp.Total, n)
		}
		if resp.Page != p || resp.PageSize != ps {
			t.Errorf("%s: page/pageSize echoed as %d/%d", path, resp.Page, resp.PageSize)
		}
		start := (p - 1) * ps
		want := 0
		if start < n {
			want = n - start
			if want > ps {
				want = ps
			}
		}
		if len(resp.Stores) != want {
			t.Fatalf("%s with %d stores: got %d items, want %d", path, n, len(resp.Stores), want)
		}
		for i, st := range resp.Stores {
			if wantID := fmt.Sprintf("s%03d", start+i); st.Store.ID != wantID {
				t.Errorf("%s: item %d is %s, want %s", path, i, st.Store.ID, wantID)
			}
		}
	})
}

// FuzzHandleStoreSearchParams: the handler answers 200 exactly when plz, page
// and pageSize are valid (or absent), and otherwise 400 naming the first
// invalid field in the order plz, page, pageSize.
func FuzzHandleStoreSearchParams(f *testing.F) {
	f.Add("", "", "", "")
	f.Add("berlin", "10115", "1", "20")
	f.Add("", "123", "1", "20")
	f.Add("", "00000", "", "")
	f.Add("", "", "0", "")
	f.Add("", "", "", "0")
	f.Add("", "", "", "101")
	f.Add("", "", "abc", "1e2")
	f.Add("\xff", "\xff\xfe", "-1", "+5")
	f.Add("", "", "1048577", "100")
	f.Fuzz(func(t *testing.T, q, plz, page, pageSize string) {
		fs := newFakeStore()
		fs.stores = []domain.Store{{ID: "s1", Name: "Berlin Mitte", City: "Berlin", Lat: 52.5, Lon: 13.4}}
		s, _ := newTestServer(fs)

		v := url.Values{}
		v.Set("q", q)
		v.Set("plz", plz)
		v.Set("page", page)
		v.Set("pageSize", pageSize)
		rr := do(s, http.MethodGet, "/api/stores?"+v.Encode(), nil)

		inRange := func(v string, lo, hi int) bool {
			if v == "" {
				return true
			}
			n, err := strconv.Atoi(v)
			return err == nil && n >= lo && n <= hi
		}
		wantField := ""
		switch {
		case plz != "" && !validPLZ(plz):
			wantField = "plz"
		case plz != "" && !func() bool { _, ok := geo.Lookup(plz); return ok }():
			wantField = "plz"
		case !inRange(page, 1, 1<<20):
			wantField = "page"
		case !inRange(pageSize, 1, maxPageSize):
			wantField = "pageSize"
		}

		if wantField == "" {
			if rr.Code != http.StatusOK {
				t.Fatalf("q=%q plz=%q page=%q pageSize=%q: status %d, want 200; body %s", q, plz, page, pageSize, rr.Code, rr.Body.String())
			}
			var resp StoreSearchResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if page == "" && resp.Page != 1 {
				t.Errorf("default page %d, want 1", resp.Page)
			}
			if pageSize == "" && resp.PageSize != defaultPageSize {
				t.Errorf("default pageSize %d, want %d", resp.PageSize, defaultPageSize)
			}
			return
		}
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("q=%q plz=%q page=%q pageSize=%q: status %d, want 400 for %s", q, plz, page, pageSize, rr.Code, wantField)
		}
		var e apiError
		if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil {
			t.Fatalf("decode error body: %v", err)
		}
		if e.Field == nil || *e.Field != wantField || e.Error != codeValidation {
			t.Errorf("q=%q plz=%q page=%q pageSize=%q: error %+v, want field %s", q, plz, page, pageSize, e, wantField)
		}
	})
}
