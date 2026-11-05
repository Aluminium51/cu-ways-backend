package domain

import "testing"

func TestAttachmentTypeIsValidMatchesDatabaseCheck(t *testing.T) {
	for _, tc := range []struct {
		value AttachmentType
		valid bool
	}{
		{AttachmentTypeBrief, true},
		{AttachmentTypeProof, true},
		{AttachmentTypeOther, true},
		{"", false},
		{"proof", false},
		{"PROOF", false},
		{" Proof", false},
		{"Invoice", false},
	} {
		if got := tc.value.IsValid(); got != tc.valid {
			t.Errorf("%q.IsValid() = %v, want %v", tc.value, got, tc.valid)
		}
	}
}
