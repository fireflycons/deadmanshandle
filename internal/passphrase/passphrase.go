// Package passphrase generates passphrases of random words, easy to read
// out, write down and type.
//
// The words are the EFF's short word list 1
// (https://www.eff.org/dice), by the Electronic Frontier Foundation,
// licensed under CC BY 3.0 US (https://creativecommons.org/licenses/by/3.0/us/).
package passphrase

import (
	"crypto/rand"
	_ "embed"
	"errors"
	"math/big"
	"strings"
)

// DefaultWords gives about 62 bits: 6 words from 1296
const DefaultWords = 6

// Separator joins the words
const Separator = "-"

//go:embed eff_short_wordlist_1.txt
var wordList string

// words are the list's words, one per line after the dice number and a tab
var words = func() []string {
	lines := strings.Split(strings.TrimSpace(wordList), "\n")
	list := make([]string, len(lines))
	for i, line := range lines {
		_, word, _ := strings.Cut(line, "\t")
		list[i] = strings.TrimSpace(word)
	}
	return list
}()

// Generate returns n words chosen at random, joined by Separator
func Generate(n int) (string, error) {
	if n < 1 {
		return "", errors.New("a passphrase needs at least one word")
	}
	chosen := make([]string, n)
	limit := big.NewInt(int64(len(words)))
	for i := range chosen {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		chosen[i] = words[index.Int64()]
	}
	return strings.Join(chosen, Separator), nil
}
