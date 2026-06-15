package bencode

import "testing"

func TestEncode(t *testing.T) {
	t.Run("dictionary encode", func(t *testing.T) {
		want := "d10:inner_dictd4:key16:value14:key2i42e8:list_keyl5:item15:item2i3eeee"
		input := Dictionary{
			DictionaryValue{
				Key: "inner_dict",
				Value: Dictionary{
					DictionaryValue{
						Key:   "key1",
						Value: "value1",
					},
					DictionaryValue{
						Key:   "key2",
						Value: 42,
					},
					DictionaryValue{
						Key:   "list_key",
						Value: []any{"item1", "item2", 3},
					},
				},
			},
		}

		got, _ := Encode(input)
		if got != want {
			t.Fatalf("expected got=%s, to eqal want=%s", got, want)
		}
	})
}
