package main

import (
	"bytes"
	"regexp"
	"testing"
)

func TestSampleReportIncludesCoverageMap(t *testing.T) {
	data, err := sampleReport()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatal("sample is not a PDF")
	}
	if !regexp.MustCompile(`/Subtype\s*/Image`).Match(data) {
		t.Fatal("sample has no coverage image")
	}
}
