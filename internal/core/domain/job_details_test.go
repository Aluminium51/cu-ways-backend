package domain

import (
	"testing"
	"time"
)

func TestEffectiveJobDeadlineUsesTheEarlierJobOrSurveyDate(t *testing.T) {
	early := time.Date(2030, 10, 20, 0, 0, 0, 0, time.UTC)
	late := early.AddDate(0, 0, 8)
	for _, test := range []struct {
		job     *time.Time
		surveys []Survey
		want    *time.Time
	}{
		{nil, nil, nil}, {&late, nil, &late}, {nil, []Survey{{Deadline: &early}}, &early},
		{&late, []Survey{{Deadline: &early}}, &early}, {&early, []Survey{{Deadline: &late}}, &early},
	} {
		got := EffectiveJobDeadline(test.job, test.surveys)
		if (got == nil) != (test.want == nil) || (got != nil && !got.Equal(*test.want)) {
			t.Fatalf("got %v, want %v", got, test.want)
		}
	}
}
