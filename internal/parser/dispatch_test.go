package parser

import (
	"reflect"
	"testing"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

// legacyLookup is the original map-plus-classes resolution.
func legacyLookup(state parserState, id tokenizer.TokenID) stateTransitionFunc {
	valid := stateTransitions[state]
	if h, ok := valid[id]; ok {
		return h
	}
	for _, class := range id.Classes() {
		if h, ok := valid[class]; ok {
			return h
		}
	}
	return nil
}

func fnPtr(f stateTransitionFunc) uintptr {
	if f == nil {
		return 0
	}
	return reflect.ValueOf(f).Pointer()
}

func TestDispatchMatchesLegacy(t *testing.T) {
	for s := parserState(0); s < numStates; s++ {
		for id := tokenizer.TokenID(0); int(id) < tokenizer.NumTokenIDs; id++ {
			if got, want := fnPtr(lookupTransition(s, id)), fnPtr(legacyLookup(s, id)); got != want {
				t.Errorf("state %v token %v: handler mismatch", s, id)
			}
		}
	}
	for state := range stateTransitions {
		if state < 0 || state >= numStates {
			t.Errorf("stateTransitions has out-of-range state %d", state)
		}
	}
}

func TestDispatchOutOfRange(t *testing.T) {
	if lookupTransition(-1, tokenizer.Newline) != nil || lookupTransition(numStates, tokenizer.Newline) != nil ||
		lookupTransition(stateDocument, -1) != nil || lookupTransition(stateDocument, tokenizer.TokenID(tokenizer.NumTokenIDs)) != nil {
		t.Error("expected nil handler for out-of-range values")
	}
}
