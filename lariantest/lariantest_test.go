package lariantest_test

import (
	"bytes"
	"testing"

	"github.com/Rethunk-Tech/go-larian"
	"github.com/Rethunk-Tech/go-larian/lariantest"
)

func TestBuildPakRoundTrips(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{"Mods/A/meta.lsx": bytes.Repeat([]byte("x"), 99), "b.txt": []byte("hi")}

	for _, v := range []int{15, 16, 18} {
		for _, c := range []larian.Compression{larian.CompressionNone, larian.CompressionZlib, larian.CompressionLZ4} {
			b := lariantest.BuildPak(t, v, files, c)

			p, err := larian.Open(bytes.NewReader(b), int64(len(b)))
			if err != nil {
				t.Fatalf("v%d c%d: %v", v, c, err)
			}

			for n, want := range files {
				if got, err := p.Read(n); err != nil || !bytes.Equal(got, want) {
					t.Fatalf("v%d c%d %s: %v", v, c, n, err)
				}
			}
		}
	}
}
