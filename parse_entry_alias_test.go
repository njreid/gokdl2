package kdl

import "testing"

func TestUnmarshalStructInputMutation(t *testing.T) {
	type S struct {
		Name string `kdl:",arg"`
		Key  string `kdl:"key,prop"`
	}
	type Top struct {
		Name S `kdl:"name"`
	}
	src := []byte("name \"hello world\" key=\"some value\"\n")
	var got Top
	if err := Unmarshal(src, &got); err != nil {
		t.Fatal(err)
	}
	for i := range src {
		src[i] = 'X'
	}
	if got.Name.Name != "hello world" || got.Name.Key != "some value" {
		t.Errorf("unmarshaled value changed after mutating input: %+v", got)
	}
}
