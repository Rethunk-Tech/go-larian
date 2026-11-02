package larian

import "testing"

func TestVersion64(t *testing.T) {
	t.Parallel()

	cases := []struct {
		packed                        Version64
		major, minor, revision, build uint64
		str                           string
	}{
		{36028797018963968, 1, 0, 0, 0, "1.0.0.0"},
		{0, 0, 0, 0, 0, "0.0.0.0"},
		{36028797018963969, 1, 0, 0, 1, "1.0.0.1"},
		{NewVersion64(4, 1, 1, 6), 4, 1, 1, 6, "4.1.1.6"},
		{NewVersion64(255, 255, 65535, 1<<31-1), 255, 255, 65535, 1<<31 - 1, "255.255.65535.2147483647"},
	}

	for _, tc := range cases {
		if tc.packed.Major() != tc.major || tc.packed.Minor() != tc.minor ||
			tc.packed.Revision() != tc.revision || tc.packed.Build() != tc.build || tc.packed.String() != tc.str {
			t.Errorf("%d: got %s", tc.packed, tc.packed)
		}

		if got := NewVersion64(tc.major, tc.minor, tc.revision, tc.build); got != tc.packed {
			t.Errorf("repack %s: %d != %d", tc.str, got, tc.packed)
		}
	}

	if NewVersion64(0, 0, 0, 1<<31) != 0 {
		t.Error("build must truncate to 31 bits")
	}
}
