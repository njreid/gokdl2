package tokenizer

import "testing"

func TestIsBareIdentifierVersion(t *testing.T) {
	tests := []struct {
		s    string
		v    Version
		want bool
	}{
		{"node", VersionV2, true},
		{"é", VersionV2, true},
		{"+", VersionV2, true},
		{"-", VersionV2, true},
		{".", VersionV2, true},
		{"-.", VersionV2, true},
		{"+.", VersionV2, true},
		{"--flag", VersionV2, true},
		{"+-1", VersionV2, true},
		{"-.x", VersionV2, true},
		{"..5", VersionV2, true},
		{"-nan", VersionV2, true},
		{"true1", VersionV2, true},
		{"a1", VersionV2, true},
		{"", VersionV2, false},
		{"true", VersionV2, false},
		{"false", VersionV2, false},
		{"null", VersionV2, false},
		{"inf", VersionV2, false},
		{"-inf", VersionV2, false},
		{"nan", VersionV2, false},
		{"1", VersionV2, false},
		{"+1", VersionV2, false},
		{"-1x", VersionV2, false},
		{".5", VersionV2, false},
		{"-.5", VersionV2, false},
		{"+.5", VersionV2, false},
		{"a\uFEFFb", VersionV2, false},
		{"a=b", VersionV2, false},

		// KDL v1 keeps its own rules.
		{"node", VersionV1, true},
		{"inf", VersionV1, true},
		{".5", VersionV1, true},
		{"-.5", VersionV1, true},
		{"1", VersionV1, false},
	}
	for _, tt := range tests {
		if got := IsBareIdentifierVersion(tt.s, 0, tt.v); got != tt.want {
			t.Errorf("IsBareIdentifierVersion(%q, %v) = %v, want %v", tt.s, tt.v, got, tt.want)
		}
	}
}
