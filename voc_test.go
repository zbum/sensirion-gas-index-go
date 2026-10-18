package gasindex

import "testing"

func TestVOCReferenceVector(t *testing.T) {
	wantAt := map[int]int32{
		0: 0, 45: 0, 46: 1, 60: 46, 600: 100, 1200: 101,
		2700: 101, 3600: 94, 5220: 215, 7200: 361,
		10800: 133, 14400: 202, 19999: 292,
	}

	voc := NewVOC()
	for i := range 20000 {
		got := voc.Process(referenceRaw(i))
		if want, ok := wantAt[i]; ok && got != want {
			t.Errorf("sample %d: got %d, want %d", i, got, want)
		}
	}

	mean, std := voc.State()
	if diff := abs(mean - 11995.7988); diff > 0.02 {
		t.Errorf("mean = %.6f, want 11995.7988", mean)
	}
	if diff := abs(std - 198.215698); diff > 0.02 {
		t.Errorf("std = %.6f, want 198.215698", std)
	}
}

func TestVOCReset(t *testing.T) {
	voc := NewVOC()
	for range 100 {
		voc.Process(30000)
	}
	voc.Reset()
	if got := voc.Process(30000); got != 0 {
		t.Fatalf("first sample after reset = %d, want 0", got)
	}
	if !voc.InBlackout() {
		t.Fatal("processor should be in blackout after reset")
	}
}

func TestVOCInvalidRawReusesLastSample(t *testing.T) {
	a, b := NewVOC(), NewVOC()
	for range 200 {
		a.Process(30000)
		b.Process(30000)
	}
	want := a.Process(30000)
	if got := b.Process(0); got != want {
		t.Fatalf("invalid sample result = %d, want %d", got, want)
	}
}

func referenceRaw(i int) int32 {
	jitter := int32((i*37)%101) - 50
	switch {
	case i < 3600:
		return 32000 + int32(i/30) + jitter
	case i < 7200:
		return 32120 - int32((i-3600)/6) + jitter
	case i < 10800:
		return 31520 + int32((i-7200)/8) + jitter
	default:
		return 31970 - int32((i-10800)/20) + jitter
	}
}
