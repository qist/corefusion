package system

import "testing"

func TestMaskDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "postgres password contains at sign",
			dsn:  "postgres://corefusion:Cf@2026test@192.168.3.100:5432/cf116_test",
			want: "postgres://corefusion:********@192.168.3.100:5432/cf116_test",
		},
		{
			name: "mysql scheme password contains at sign",
			dsn:  "mysql://corefusion:Cf@2026test@tcp(192.168.3.70:3306)/cf116_test",
			want: "mysql://corefusion:********@tcp(192.168.3.70:3306)/cf116_test",
		},
		{
			name: "bare mysql password contains at sign",
			dsn:  "corefusion:Cf@2026test@tcp(192.168.3.70:3306)/cf116_test",
			want: "corefusion:********@tcp(192.168.3.70:3306)/cf116_test",
		},
		{
			name: "sqlite path untouched",
			dsn:  "/opt/corefusion/data/sqlite/app.db",
			want: "/opt/corefusion/data/sqlite/app.db",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskDSN(tc.dsn); got != tc.want {
				t.Fatalf("maskDSN(%q) = %q, want %q", tc.dsn, got, tc.want)
			}
		})
	}
}

func TestNormalizeDSNForCompare(t *testing.T) {
	cases := []struct{ a, b string }{
		{"mysql://u:p@tcp(h:3306)/db", "u:p@tcp(h:3306)/db"},
		{"postgres://u:p@h:5432/db?sslmode=disable", "u:p@h:5432/db?sslmode=disable"},
		{"postgresql://u:p@h/db", "u:p@h/db"},
		{"sqlite:///opt/app.db", "/opt/app.db"},
	}
	for _, c := range cases {
		if got, want := normalizeDSNForCompare(c.a), normalizeDSNForCompare(c.b); got != want {
			t.Fatalf("normalize mismatch: %q -> %q, %q -> %q", c.a, got, c.b, want)
		}
	}
}
