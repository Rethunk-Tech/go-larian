package larian

import "fmt"

// Version64 is Larian's packed 64-bit module version: 9 bits major, 8 bits
// minor, 16 bits revision and 31 bits build, high to low.
type Version64 uint64

const (
	versionBuildBits = 31
	versionRevBits   = 16
	versionMinorBits = 8
	versionMajorBits = 9
)

// NewVersion64 packs the four parts; each is truncated to its field width.
func NewVersion64(major, minor, revision, build uint64) Version64 {
	return Version64(
		(major&(1<<versionMajorBits-1))<<(versionMinorBits+versionRevBits+versionBuildBits) |
			(minor&(1<<versionMinorBits-1))<<(versionRevBits+versionBuildBits) |
			(revision&(1<<versionRevBits-1))<<versionBuildBits |
			build&(1<<versionBuildBits-1))
}

// Major is the top field.
func (v Version64) Major() uint64 {
	return uint64(v) >> (versionMinorBits + versionRevBits + versionBuildBits) & (1<<versionMajorBits - 1)
}

// Minor is the second field.
func (v Version64) Minor() uint64 {
	return uint64(v) >> (versionRevBits + versionBuildBits) & (1<<versionMinorBits - 1)
}

// Revision is the third field.
func (v Version64) Revision() uint64 {
	return uint64(v) >> versionBuildBits & (1<<versionRevBits - 1)
}

// Build is the low field.
func (v Version64) Build() uint64 {
	return uint64(v) & (1<<versionBuildBits - 1)
}

// String formats the version as "major.minor.revision.build".
func (v Version64) String() string {
	return fmt.Sprintf("%d.%d.%d.%d", v.Major(), v.Minor(), v.Revision(), v.Build())
}
