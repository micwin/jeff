package main

import "testing"

func TestParseVaultlineVersion(t *testing.T) {
	version, ok := parseVaultlineVersion("0.3.44\n")
	if !ok {
		t.Fatalf("expected version to parse")
	}
	if version.major != 0 || version.minor != 3 || version.patch != 44 {
		t.Fatalf("unexpected version: %#v", version)
	}
	if _, ok := parseVaultlineVersion("not-a-version"); ok {
		t.Fatalf("expected malformed version to fail")
	}
}

func TestCompareVaultlineVersions(t *testing.T) {
	if compareVaultlineVersions(vaultlineVersion{0, 3, 44}, vaultlineVersion{0, 3, 43}) <= 0 {
		t.Fatalf("expected newer patch to compare greater")
	}
	if compareVaultlineVersions(vaultlineVersion{0, 4, 0}, vaultlineVersion{0, 3, 99}) <= 0 {
		t.Fatalf("expected newer minor to compare greater")
	}
	if compareVaultlineVersions(vaultlineVersion{1, 0, 0}, vaultlineVersion{0, 99, 99}) <= 0 {
		t.Fatalf("expected newer major to compare greater")
	}
	if compareVaultlineVersions(vaultlineVersion{0, 3, 44}, vaultlineVersion{0, 3, 44}) != 0 {
		t.Fatalf("expected identical versions to compare equal")
	}
}
