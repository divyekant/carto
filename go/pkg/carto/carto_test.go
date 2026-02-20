package carto

import (
	"testing"
)

func TestIndexOptionsDefaults(t *testing.T) {
	opts := IndexOptions{}
	if opts.Incremental {
		t.Fatal("expected incremental=false by default")
	}
}

func TestQueryOptionsDefaults(t *testing.T) {
	opts := QueryOptions{}
	if opts.K != 0 {
		t.Fatal("expected K=0 by default (caller sets)")
	}
}
