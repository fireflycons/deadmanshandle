package adapters

import (
	"maps"
	"slices"
	"testing"
)

// DynamoDB rejects a request whose expression attribute names are not all
// used, so each expression must get exactly the names it refers to
func TestUsedNames(t *testing.T) {
	tests := []struct {
		expression string
		want       []string
	}{
		{"SET #t = :t REMOVE #s, #o ADD #c :one ", []string{"#c", "#o", "#s", "#t"}},
		{"ADD #s :r #c = :c", []string{"#c", "#s"}},
		{"SET #o = :true attribute_exists(#t) AND (attribute_not_exists(#c) OR #c = :c)", []string{"#c", "#o", "#t"}},
		{"SET #e = :n attribute_exists(#t) AND attribute_not_exists(#e)", []string{"#e", "#t"}},
		{"SET #e = :n #e = :p", []string{"#e"}},
	}
	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			got := slices.Sorted(maps.Keys(usedNames(tt.expression)))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Expected names %v, got %v", tt.want, got)
			}
		})
	}
}
