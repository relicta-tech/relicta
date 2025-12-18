package version

import (
	"testing"
)

// Benchmark version strings representing various formats.
var benchVersionStrings = []string{
	"1.0.0",
	"v1.0.0",
	"1.2.3",
	"10.20.30",
	"1.0.0-alpha",
	"1.0.0-alpha.1",
	"1.0.0-beta.2",
	"1.0.0-rc.1",
	"1.0.0+build.123",
	"1.0.0-alpha+build.456",
	"2.0.0-alpha.1+meta.data",
	"100.200.300",
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()

	b.Run("simple", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = Parse("1.0.0")
		}
	})

	b.Run("with_v_prefix", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = Parse("v1.0.0")
		}
	})

	b.Run("large_numbers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = Parse("100.200.300")
		}
	})

	b.Run("with_prerelease", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = Parse("1.0.0-alpha.1")
		}
	})

	b.Run("with_metadata", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = Parse("1.0.0+build.123")
		}
	})

	b.Run("full_format", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = Parse("2.0.0-alpha.1+meta.data")
		}
	})

	b.Run("mixed_batch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			v := benchVersionStrings[i%len(benchVersionStrings)]
			_, _ = Parse(v)
		}
	})

	b.Run("invalid", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = Parse("not-a-version")
		}
	})
}

func BenchmarkVersionBump_Apply(b *testing.B) {
	b.ReportAllocs()

	v := NewSemanticVersion(1, 2, 3)
	majorBump := NewVersionBump(BumpMajor)
	minorBump := NewVersionBump(BumpMinor)
	patchBump := NewVersionBump(BumpPatch)

	b.Run("major", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = majorBump.Apply(v)
		}
	})

	b.Run("minor", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = minorBump.Apply(v)
		}
	})

	b.Run("patch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = patchBump.Apply(v)
		}
	})

	b.Run("with_prerelease", func(b *testing.B) {
		prereleaseV := NewSemanticVersionWithPrerelease(1, 0, 0, PrereleaseAlpha)
		for i := 0; i < b.N; i++ {
			_ = patchBump.Apply(prereleaseV)
		}
	})

	b.Run("sequential_bumps", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			current := NewSemanticVersion(0, 1, 0)
			current = patchBump.Apply(current)
			current = patchBump.Apply(current)
			current = minorBump.Apply(current)
			_ = majorBump.Apply(current)
		}
	})
}

func BenchmarkSemanticVersion_String(b *testing.B) {
	b.ReportAllocs()

	b.Run("simple", func(b *testing.B) {
		v := NewSemanticVersion(1, 2, 3)
		for i := 0; i < b.N; i++ {
			_ = v.String()
		}
	})

	b.Run("with_prerelease", func(b *testing.B) {
		v := NewSemanticVersionWithPrerelease(1, 0, 0, PrereleaseAlpha)
		for i := 0; i < b.N; i++ {
			_ = v.String()
		}
	})

	b.Run("large_numbers", func(b *testing.B) {
		v := NewSemanticVersion(999, 888, 777)
		for i := 0; i < b.N; i++ {
			_ = v.String()
		}
	})
}

func BenchmarkSemanticVersion_Compare(b *testing.B) {
	b.ReportAllocs()

	v1 := NewSemanticVersion(1, 2, 3)
	v2 := NewSemanticVersion(1, 2, 4)
	v3 := NewSemanticVersion(2, 0, 0)

	b.Run("equal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = v1.Compare(v1)
		}
	})

	b.Run("less_patch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = v1.Compare(v2)
		}
	})

	b.Run("less_major", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = v1.Compare(v3)
		}
	})
}

func BenchmarkNewSemanticVersion(b *testing.B) {
	b.ReportAllocs()

	b.Run("new", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewSemanticVersion(1, 2, 3)
		}
	})

	b.Run("with_prerelease", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewSemanticVersionWithPrerelease(1, 2, 3, PrereleaseAlpha)
		}
	})
}
