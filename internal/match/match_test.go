package match

import "testing"

func TestArtifactMatches(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"", "any.jar", true},
		{"*", "any.jar", true},
		{"chatchannels-*.jar", "chatchannels-1.1.jar", true},
		{"chatchannels-*.jar", "other.jar", false},
		{"LuckPerms-Bukkit-5.5.71.jar", "LuckPerms-Bukkit-5.5.71.jar", true},
	}
	for _, tc := range cases {
		if got := ArtifactMatches(tc.pattern, tc.name); got != tc.want {
			t.Errorf("ArtifactMatches(%q,%q)=%v want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestVersionMatchesExactAndWildcard(t *testing.T) {
	if !VersionMatches("", "1.0") {
		t.Fatal("empty should match")
	}
	if !VersionMatches("*", "1.0") {
		t.Fatal("* should match")
	}
	if !VersionMatches("1.1", "v1.1") {
		t.Fatal("tag normalize")
	}
	if VersionMatches("1.2", "1.1") {
		t.Fatal("exact mismatch")
	}
}

func TestVersionRanges(t *testing.T) {
	ok, err := MatchRange("[1.0,2.0)", "1.5")
	if err != nil || !ok {
		t.Fatalf("[1.0,2.0) vs 1.5: %v %v", ok, err)
	}
	ok, err = MatchRange("[1.0,2.0)", "2.0")
	if err != nil || ok {
		t.Fatalf("[1.0,2.0) vs 2.0 should be false: %v %v", ok, err)
	}
	ok, err = MatchRange("(,1.0]", "1.0")
	if err != nil || !ok {
		t.Fatalf("(,1.0] vs 1.0: %v %v", ok, err)
	}
	ok, err = MatchRange("[1.2.1]", "1.2.1")
	if err != nil || !ok {
		t.Fatalf("[1.2.1] exact: %v %v", ok, err)
	}
	// Maven: 33.0.0-jre < 33.0, so it is inside [32.0,33.0)
	best := MaxMatching("[32.0,33.0)", []string{"31.0-jre", "32.1.3-jre", "33.0.0-jre", "32.0.0-jre"})
	if best != "33.0.0-jre" {
		t.Fatalf("MaxMatching got %q", best)
	}
	best = MaxMatching("[32.0,33.0.0-jre)", []string{"31.0-jre", "32.1.3-jre", "33.0.0-jre", "32.0.0-jre"})
	if best != "32.1.3-jre" {
		t.Fatalf("MaxMatching exclusive jre got %q", best)
	}
}

func TestCompareQualifiers(t *testing.T) {
	a, _ := ParseVersion("1.0-alpha")
	b, _ := ParseVersion("1.0")
	if Compare(a, b) >= 0 {
		t.Fatal("alpha should be less than release")
	}
}
