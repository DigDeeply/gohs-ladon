package main

import (
	"testing"
)

// test build scratch
func TestBuildScratch(t *testing.T) {
	RegexMap = make(map[int]RegexLine)
	filepath := "patterns/pattern1.txt"
	err := buildScratch(filepath)
	if err != nil {
		t.Error(err)
	}
}
