package bbbench_go

import "testing"

func TestHello(t *testing.T) {
	if s := Hello(); s != helloCriteo {
		t.Fatalf("Hello(): got %v; want %v", s, helloCriteo)
	}
}
