package services

import (
	"testing"
)

func TestStreaksOf(t *testing.T) {
	cases := []struct {
		name              string
		days              map[string]bool
		today             string
		wantCur, wantLong int
	}{
		{"empty", map[string]bool{}, "2025-09-05", 0, 0},
		{"today checked", map[string]bool{"2025-09-05": true, "2025-09-04": true}, "2025-09-05", 2, 2},
		{"today missing falls back to yesterday", map[string]bool{"2025-09-04": true, "2025-09-03": true}, "2025-09-05", 2, 2},
		{"gap breaks current", map[string]bool{"2025-09-01": true, "2025-09-04": true}, "2025-09-05", 1, 1},
		{"year boundary", map[string]bool{"2024-12-31": true, "2025-01-01": true, "2025-01-02": true}, "2025-01-02", 3, 3},
		{"longest in the past", map[string]bool{
			"2025-08-01": true, "2025-08-02": true, "2025-08-03": true, "2025-08-04": true,
			"2025-09-05": true,
		}, "2025-09-05", 1, 4},
		{"no today and no yesterday", map[string]bool{"2025-09-01": true}, "2025-09-05", 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StreaksOf(c.days, c.today)
			if got.Current != c.wantCur || got.Longest != c.wantLong {
				t.Fatalf("current=%d longest=%d, want %d/%d", got.Current, got.Longest, c.wantCur, c.wantLong)
			}
		})
	}
}
