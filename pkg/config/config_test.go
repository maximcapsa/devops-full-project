package config

import (
	"testing"
	"time"
)

func TestString(t *testing.T) {
	t.Setenv("FOO", "bar")
	if got := String("FOO", "def"); got != "bar" {
		t.Errorf("String set = %q, want bar", got)
	}
	if got := String("MISSING_FOO", "def"); got != "def" {
		t.Errorf("String unset = %q, want def", got)
	}
}

func TestMustString(t *testing.T) {
	t.Setenv("REQ", "v")
	if _, err := MustString("REQ"); err != nil {
		t.Errorf("MustString set: unexpected err %v", err)
	}
	if _, err := MustString("MISSING_REQ"); err == nil {
		t.Error("MustString unset: want error, got nil")
	}
}

func TestInt(t *testing.T) {
	t.Setenv("N", "42")
	if got := Int("N", 7); got != 42 {
		t.Errorf("Int = %d, want 42", got)
	}
	t.Setenv("BAD", "notanint")
	if got := Int("BAD", 7); got != 7 {
		t.Errorf("Int bad = %d, want fallback 7", got)
	}
}

func TestDuration(t *testing.T) {
	t.Setenv("D", "1500ms")
	if got := Duration("D", time.Second); got != 1500*time.Millisecond {
		t.Errorf("Duration = %v, want 1.5s", got)
	}
}

func TestStrings(t *testing.T) {
	t.Setenv("LIST", " a, b ,c ")
	got := Strings("LIST", nil)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("Strings = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Strings[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
