package torrent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func TestBencodedString(t *testing.T) {
	t.Run("base example", func(t *testing.T) {
		want := "hello"
		dec := &Decoder{
			Data: fmt.Appendf(nil, "%d:%s", len(want), want),
		}

		got := dec.Decode()
		if got != want {
			t.Fatalf("decode(%d:%s) expected %s, got %s", len(want), want, want, got)
		}
	})

	t.Run("complex example", func(t *testing.T) {
		want := "codecrafters-bittorent-go"
		dec := &Decoder{
			Data: fmt.Appendf(nil, "%d:%s", len(want), want),
		}

		got := dec.Decode()
		if got != want {
			t.Fatalf("decode(%d:%s) expected %s, got %s", len(want), want, want, got)
		}
	})

	t.Run("invalid len value", func(t *testing.T) {
		want := "codecrafters-bittorent-go"
		dec := &Decoder{
			Data: fmt.Appendf(nil, "%d:%s", 100, want),
		}

		dec.Decode()
		if dec.Err == nil {
			t.Fatal("expected error from decode, got nil")
		}
	})
}

func TestBencodedInteger(t *testing.T) {
	t.Run("base example", func(t *testing.T) {
		want := 32
		dec := &Decoder{
			Data: fmt.Appendf(nil, "i%de", want),
		}

		got := dec.Decode()
		if got != want {
			t.Fatalf("decode(i%de) expected %d, got %s", want, want, got)
		}
	})

	t.Run("negative number", func(t *testing.T) {
		want := -32
		dec := &Decoder{
			Data: fmt.Appendf(nil, "i%de", want),
		}

		got := dec.Decode()
		if got != want {
			t.Fatalf("decode(i%de) expected %d, got %s", want, want, got)
		}
	})

	t.Run("negative number", func(t *testing.T) {
		want := -32
		dec := &Decoder{
			Data: fmt.Appendf(nil, "i%de", want),
		}

		got := dec.Decode()
		if got != want {
			t.Fatalf("decode(i%de) expected %d, got %s", want, want, got)
		}
	})

	t.Run("large number", func(t *testing.T) {
		want := math.MaxInt64
		dec := &Decoder{
			Data: fmt.Appendf(nil, "i%de", want),
		}

		got := dec.Decode()
		if got != want {
			t.Fatalf("decode(i%de) expected %d, got %s", want, want, got)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		want := 10
		dec := &Decoder{
			Data: fmt.Appendf(nil, "i%d", want),
		}

		dec.Decode()
		if dec.Err == nil {
			t.Fatal("expected error from decode, got nil")
		}
	})
}

func TestBencodedList(t *testing.T) {
	t.Run("base example", func(t *testing.T) {
		dec := &Decoder{
			Data: fmt.Appendf(nil, "l5:helloi32ee"),
		}

		want := []any{"hello", 32}
		got := dec.Decode()

		gotList, ok := got.([]any)
		if !ok {
			t.Fatal("expected array from decode, got nil")
		}

		jsonWant, _ := json.Marshal(want)
		jsonGot, _ := json.Marshal(gotList)

		if string(jsonGot) != string(jsonWant) {
			t.Fatalf("expected got=%s, to eqal want=%s", jsonGot, jsonWant)
		}
	})

	t.Run("inner list", func(t *testing.T) {
		dec := &Decoder{
			Data: fmt.Appendf(nil, "ll5:helloee"),
		}

		want := []any{[]any{"hello"}}
		got := dec.Decode()

		gotList, ok := got.([]any)
		if !ok {
			t.Fatal("expected array from decode, got nil")
		}

		jsonWant, _ := json.Marshal(want)
		jsonGot, _ := json.Marshal(gotList)

		if string(jsonGot) != string(jsonWant) {
			t.Fatalf("expected got=%s, to eqal want=%s", jsonGot, jsonWant)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		dec := &Decoder{
			Data: fmt.Appendf(nil, "l5:helloi32e"),
		}

		dec.Decode()
		if dec.Err == nil {
			t.Fatal("expected error from decode, got nil")
		}
	})
}

func TestBencodedDict(t *testing.T) {
	t.Run("base example", func(t *testing.T) {
		dec := &Decoder{
			Data: fmt.Appendf(nil, "d3:foo3:bar5:helloi52ee"),
		}

		want := map[string]any{
			"foo":   "bar",
			"hello": 52,
		}

		decodeRes := dec.Decode()

		got, ok := decodeRes.(Dictionary)
		if !ok {
			t.Fatal("expected map from decode, got nil")
		}

		gotMap := got.ToMap()

		jsonWant, _ := json.Marshal(want)
		jsonGot, _ := json.Marshal(gotMap)

		if string(jsonGot) != string(jsonWant) {
			t.Fatalf("expected got=%s, to eqal want=%s", jsonGot, jsonWant)
		}
	})

	t.Run("inner dictionary and list", func(t *testing.T) {
		dec := &Decoder{
			Data: fmt.Appendf(nil, "d10:inner_dictd4:key16:value14:key2i42e8:list_keyl5:item15:item2i3eeee"),
		}

		want := map[string]any{
			"inner_dict": map[string]any{
				"key1":     "value1",
				"key2":     42,
				"list_key": []any{"item1", "item2", 3},
			},
		}

		decodeRes := dec.Decode()

		got, ok := decodeRes.(Dictionary)
		if !ok {
			t.Fatal("expected map from decode, got nil")
		}

		gotMap := got.ToMap()

		jsonWant, _ := json.Marshal(want)
		jsonGot, _ := json.Marshal(gotMap)

		if string(jsonGot) != string(jsonWant) {
			t.Fatalf("expected got=%s, to eqal want=%s", jsonGot, jsonWant)
		}
	})
}

var benchInput = []byte("d10:inner_dictd4:key16:value14:key2i42e8:list_keyl5:item15:item2i3eeee")

func BenchmarkDecoder(b *testing.B) {
	for b.Loop() {
		dec := &Decoder{Data: benchInput}
		dec.Decode()
	}
}

func BenchmarkReaderDecoder(b *testing.B) {
	for b.Loop() {
		dec := &readerDecoder{r: bufio.NewReader(bytes.NewReader(benchInput))}
		dec.decode()
	}
}
