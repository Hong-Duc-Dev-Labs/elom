package metrics

import "testing"

func TestShortCPU(t *testing.T) {
	cases := map[string]string{
		"13th Gen Intel(R) Core(TM) i7-1360P":                          "i7-1360P",
		"Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz":                     "i5-8250U",
		"AMD Ryzen 7 5800X 8-Core Processor":                           "Ryzen 7 5800X",
		"Intel(R) Core(TM) Ultra 7 155H":                               "Ultra 7 155H",
		"Snapdragon(R) X Elite - X1E78100 - Qualcomm(R) Oryon(TM) CPU": "Snapdragon X Elite - X1E78100 - Qualcomm Oryon",
	}
	for in, want := range cases {
		if got := shortCPU(in); got != want {
			t.Errorf("shortCPU(%q) = %q, want %q", in, got, want)
		}
	}
}
