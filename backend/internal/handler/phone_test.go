package handler

import "testing"

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"already E.164", "+6281234567890", "+6281234567890", false},
		{"prefix 62", "6281234567890", "+6281234567890", false},
		{"prefix 0", "081234567890", "+6281234567890", false},
		{"with spaces", "+62 812 3456 7890", "+6281234567890", false},
		{"with dashes", "+62-812-3456-7890", "+6281234567890", false},
		{"non-Indonesia E.164", "+15551234567", "", true},
		{"letters", "abc", "", true},
		{"empty", "", "", true},
		{"too short +62", "+62", "", true},
		{"too short 0", "081", "", true},
		{"only plus", "+", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePhone(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
