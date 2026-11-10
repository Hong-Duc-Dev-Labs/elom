package winget

import (
	"context"
	"os"
	"testing"
)

func TestRealWinget(t *testing.T) {
	if os.Getenv("ELOM_REAL_WINGET") == "" {
		t.Skip("set ELOM_REAL_WINGET=1")
	}
	list, err := List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	t.Logf("%d upgrades", len(list))
	for i, u := range list {
		if i >= 3 {
			break
		}
		t.Logf("  %s %s -> %s", u.ID, u.Current, u.Available)
	}
}
