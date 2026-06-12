package torrent

import "testing"

func TestEncode(t *testing.T) {
	t.Run("dictionary encode", func(t *testing.T) {
		want := "d10:inner_dictd4:key16:value14:key2i42e8:list_keyl5:item15:item2i3eeee"
		input := Dictionary{
			dictionaryValue{
				key: "inner_dict",
				value: Dictionary{
					dictionaryValue{
						key:   "key1",
						value: "value1",
					},
					dictionaryValue{
						key:   "key2",
						value: 42,
					},
					dictionaryValue{
						key:   "list_key",
						value: []any{"item1", "item2", 3},
					},
				},
			},
		}

		got, _ := encode(input)
		if got != want {
			t.Fatalf("expected got=%s, to eqal want=%s", got, want)
		}
	})
}
