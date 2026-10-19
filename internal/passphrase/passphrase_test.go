package passphrase

import (
	"strings"
	"testing"
)

func TestWordList(t *testing.T) {
	if len(words) != 1296 {
		t.Fatalf("Expected 1296 words, got %d", len(words))
	}
	seen := map[string]bool{}
	for _, w := range words {
		if w == "" || seen[w] {
			t.Errorf("Blank or repeated word %q", w)
		}
		seen[w] = true
	}
	if words[0] != "acid" || words[len(words)-1] != "zoom" {
		t.Errorf("Unexpected first or last word: %q, %q", words[0], words[len(words)-1])
	}
}

func TestGenerate(t *testing.T) {
	inList := map[string]bool{}
	for _, w := range words {
		inList[w] = true
	}

	phrase, err := Generate(DefaultWords)
	if err != nil {
		t.Fatal(err)
	}
	// yo-yo is the one word containing the separator
	got := strings.Split(strings.ReplaceAll(phrase, "yo-yo", "yo\x00yo"), Separator)
	if len(got) != DefaultWords {
		t.Errorf("Expected %d words in %q, got %d", DefaultWords, phrase, len(got))
	}
	for _, w := range got {
		if w = strings.ReplaceAll(w, "\x00", "-"); !inList[w] {
			t.Errorf("%q in %q is not in the list", w, phrase)
		}
	}

	other, err := Generate(DefaultWords)
	if err != nil {
		t.Fatal(err)
	}
	if other == phrase {
		t.Errorf("Two passphrases were the same: %q", phrase)
	}
}

func TestGenerateRefusesNoWords(t *testing.T) {
	if _, err := Generate(0); err == nil {
		t.Error("Expected an error")
	}
}
