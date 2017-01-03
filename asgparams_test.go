package main

import "testing"

func TestAutoscalingGroups(t *testing.T) {
	grps := asgParams{}

	grps.Set("a,b,c")

	if len(grps) != 3 {
		t.Errorf("Expected 3 elements to be extracted, but got %d", len(grps))
	}

	if grps[0] != "a" || grps[1] != "b" || grps[2] != "c" {
		t.Error("The input was not split correctly.")
	}

	if grps.String() != "a,b,c" {
		t.Error("Lost data during conversion.")
	}
}
