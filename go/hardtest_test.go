package main

import "testing"

func TestHardTestReceipt(t *testing.T) {
	job, err := hardTestReceipt(1)
	if err != nil {
		t.Fatal(err)
	}
	black := 0
	for _, c := range job {
		if c != 0 {
			black++
		}
	}
	if len(job) < 576/8*1000 || black == 0 {
		t.Fatalf("job looks wrong: %d bytes, %d non-zero", len(job), black)
	}
}
