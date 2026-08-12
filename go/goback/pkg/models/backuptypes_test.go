package models

import "testing"

func TestBackupTypeStrings(t *testing.T) {
	cases := []struct {
		backupType BackupTypes
		want       string
	}{
		{Daily{}, "daily"},
		{Weekly{}, "weekly"},
		{Monthly{}, "monthly"},
		{Mirror{}, "mirror"},
		{NoBackupType{}, ""},
	}

	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		if got := tc.backupType.String(); got != tc.want {
			t.Fatalf("%T.String() = %q, want %q", tc.backupType, got, tc.want)
		}
		if seen[tc.want] {
			t.Fatalf("%T.String() = %q, want a value no other backup type uses", tc.backupType, tc.want)
		}
		seen[tc.want] = true
	}
}
