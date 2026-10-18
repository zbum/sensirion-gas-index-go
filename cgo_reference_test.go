//go:build cgo

package gasindex

import (
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/zbum/sensirion-gas-index-go/internal/referencec"
)

func TestVOCMatchesCReference(t *testing.T) {
	goVOC := NewVOC()
	cVOC := referencec.NewVOC()

	for i := range 20000 {
		raw := referenceRaw(i)
		got, want := goVOC.Process(raw), cVOC.Process(raw)
		if diff := abs(float32(got - want)); diff > 1 {
			t.Fatalf("sample %d (raw %d): Go index = %d, C index = %d", i, raw, got, want)
		}
	}

	goMean, goStd := goVOC.State()
	cMean, cStd := cVOC.State()
	if abs(goMean-cMean) > 0.02 || abs(goStd-cStd) > 0.02 {
		t.Fatalf(
			"final state differs: Go = (%.9g, %.9g), C = (%.9g, %.9g)",
			goMean, goStd, cMean, cStd,
		)
	}
}

func TestVOCPublishedReferenceData(t *testing.T) {
	file, err := os.Open("testdata/voc.csv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })

	goVOC := NewVOC()
	cVOC := referencec.NewVOC()
	reader := csv.NewReader(file)
	for sample := 0; ; sample++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("sample %d: read CSV: %v", sample, err)
		}
		if len(row) != 2 {
			t.Fatalf("sample %d: got %d CSV fields, want 2", sample, len(row))
		}

		raw := parseReferenceInt32(t, sample, "raw", row[0])
		published := parseReferenceInt32(t, sample, "index", row[1])
		got := goVOC.Process(raw)
		cGot := cVOC.Process(raw)
		if got != cGot {
			t.Fatalf("sample %d (raw %d): Go index = %d, C index = %d", sample, raw, got, cGot)
		}
		if diff := abs(float32(got - published)); diff > 1 {
			t.Errorf("sample %d (raw %d): index = %d, published = %d", sample, raw, got, published)
		}
	}
}

func parseReferenceInt32(t *testing.T, sample int, field, value string) int32 {
	t.Helper()
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		t.Fatalf("sample %d: parse %s %q: %v", sample, field, value, err)
	}
	return int32(parsed)
}
