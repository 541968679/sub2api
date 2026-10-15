package service

import "testing"

func TestParseAccountGroupIDList(t *testing.T) {
	ids, matchNone, err := ParseAccountGroupIDList(" 46, 48,46 ")
	if err != nil {
		t.Fatal(err)
	}
	if matchNone {
		t.Fatal("expected ids, not the empty set")
	}
	if len(ids) != 2 || ids[0] != 46 || ids[1] != 48 {
		t.Fatalf("ids = %v", ids)
	}

	ids, matchNone, err = ParseAccountGroupIDList("0")
	if err != nil || !matchNone || len(ids) != 0 {
		t.Fatalf("empty set: ids=%v matchNone=%v err=%v", ids, matchNone, err)
	}

	if _, _, err = ParseAccountGroupIDList("-1"); err == nil {
		t.Fatal("expected invalid id")
	}
	if _, _, err = ParseAccountGroupIDList("1,0"); err == nil {
		t.Fatal("expected mixed zero to be invalid")
	}
}
