package vrclog

import "testing"

func TestValidateAdapterID(t *testing.T) {
	valid := []AdapterID{
		"vrchat.core",
		"community.yamaplayer",
		"community.iwasync3",
		"a",
		"a-b_c.d",
		"ABC123",
		"x.y.z",
	}
	for _, id := range valid {
		if err := validateAdapterID(id); err != nil {
			t.Errorf("validateAdapterID(%q) = %v, want nil", id, err)
		}
	}

	invalid := []struct {
		id   AdapterID
		desc string
	}{
		{"", "empty"},
		{"has space", "space"},
		{"has\ttab", "tab"},
		{"has\nnewline", "newline"},
		{"ctrl\x01char", "control char 0x01"},
		{"ctrl\x7fchar", "control char 0x7F"},
		{"has/slash", "slash"},
		{"has@at", "at sign"},
	}
	for _, tc := range invalid {
		if err := validateAdapterID(tc.id); err == nil {
			t.Errorf("validateAdapterID(%q) [%s] = nil, want error", tc.id, tc.desc)
		}
	}
}
