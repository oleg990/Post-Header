// Package spell — проверка орфографии по словарю hunspell (только суффиксы,
// как в русском словаре А. Лебедева). Работает без интернета.
package spell

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type rule struct {
	flag  byte
	strip string
	add   string
	cond  *regexp.Regexp // условие на конец основы
}

type Checker struct {
	words map[string]string // основа → флаги
	byAdd map[string][]rule // окончание → правила с ним
	try   []rune
	user  map[string]bool // личный словарь
}

func gunzip(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

// Load разбирает сжатые .dic и .aff.
func Load(dicGz, affGz []byte) (*Checker, error) {
	aff, err := gunzip(affGz)
	if err != nil {
		return nil, err
	}
	dic, err := gunzip(dicGz)
	if err != nil {
		return nil, err
	}
	c := &Checker{words: make(map[string]string, 160000), byAdd: map[string][]rule{}, user: map[string]bool{}}
	sc := bufio.NewScanner(bytes.NewReader(aff))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		switch {
		case len(f) == 2 && f[0] == "TRY":
			c.try = []rune(norm(f[1]))
		case len(f) >= 5 && f[0] == "SFX":
			strip, add := f[2], f[3]
			if strip == "0" {
				strip = ""
			}
			if add == "0" {
				add = ""
			}
			var cond *regexp.Regexp
			if f[4] != "." {
				cond, err = regexp.Compile(norm(f[4]) + "$")
				if err != nil {
					continue
				}
			}
			add = norm(add)
			c.byAdd[add] = append(c.byAdd[add], rule{flag: f[1][0], strip: norm(strip), add: add, cond: cond})
		}
	}
	sc = bufio.NewScanner(bytes.NewReader(dic))
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first { // первая строка — число слов
			first = false
			continue
		}
		if i := strings.IndexAny(line, " \t"); i >= 0 {
			line = line[:i]
		}
		word, flags := line, ""
		if i := strings.IndexByte(line, '/'); i >= 0 {
			word, flags = line[:i], line[i+1:]
		}
		if word == "" {
			continue
		}
		w := norm(word)
		c.words[w] = c.words[w] + flags
	}
	c.addTrade()
	return c, nil
}

// addTrade — слова из недвижимости и соцсетей, которых нет в общем словаре.
func (c *Checker) addTrade() {
	masc := []string{"риелтор", "риэлтор", "кешбэк", "кэшбэк", "кешбек", "прайс", "таунхаус", "дуплекс", "пентхаус", "лофт", "эскроу", "репост", "лайк", "хештег", "хэштег", "мессенджер", "чат", "кэш", "апарт", "застройщик"}
	for _, w := range masc {
		for _, e := range []string{"", "а", "у", "ом", "е", "ы", "ов", "ам", "ами", "ах"} {
			c.words[norm(w+e)] += ""
		}
	}
	fem := []string{"вторичк", "новостройк", "госпрограмм", "ипотек", "студи", "сторис"}
	for _, w := range fem {
		for _, e := range []string{"а", "и", "е", "у", "ой", "ам", "ами", "ах", "й"} {
			c.words[norm(w+e)] += ""
		}
	}
	for _, w := range []string{"риелторский", "риелторская", "риелторское", "риелторские", "риелторских", "риелторской", "риелторскую", "риелторскими",
		"онлайн", "офлайн", "апартаменты", "апартаментов", "апартаментах", "сторис", "директ", "вотсап", "телеграм", "телеграме", "вконтакте", "ватсап"} {
		c.words[norm(w)] += ""
	}
}

// norm — нижний регистр и ё → е: в постах пишут по-разному.
func norm(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "ё", "е")
}

// AddUser добавляет слова личного словаря.
func (c *Checker) AddUser(words ...string) {
	for _, w := range words {
		c.user[norm(w)] = true
	}
}

// Correct сообщает, есть ли слово в словаре (с учётом всех окончаний).
func (c *Checker) Correct(word string) bool {
	w := norm(word)
	if c.user[w] {
		return true
	}
	if _, ok := c.words[w]; ok {
		return true
	}
	if strings.Contains(w, "-") { // «по-новому», «кешбэк-сервис»
		ok := true
		for _, part := range strings.Split(w, "-") {
			if part != "" && !c.known(part) && part != "то" && part != "либо" && part != "нибудь" && part != "ка" {
				ok = false
			}
		}
		if ok {
			return true
		}
	}
	return c.known(w)
}

func (c *Checker) known(w string) bool {
	if _, ok := c.words[w]; ok {
		return true
	}
	for i := 0; i <= len(w); i++ {
		if i < len(w) && !utf8.RuneStart(w[i]) {
			continue
		}
		rules, ok := c.byAdd[w[i:]]
		if !ok {
			continue
		}
		for _, r := range rules {
			stem := w[:i] + r.strip
			flags, ok := c.words[stem]
			if !ok || strings.IndexByte(flags, r.flag) < 0 {
				continue
			}
			if r.cond == nil || r.cond.MatchString(stem) {
				return true
			}
		}
	}
	return false
}

// Suggest — до n вариантов на расстоянии одной правки (пропущенная,
// лишняя, перепутанная или переставленная буква).
func (c *Checker) Suggest(word string, n int) []string {
	rs := []rune(norm(word))
	seen := map[string]bool{}
	var out []string
	try := func(s string) bool {
		if seen[s] || s == "" {
			return len(out) >= n
		}
		seen[s] = true
		if c.Correct(s) {
			out = append(out, matchCase(word, s))
		}
		return len(out) >= n
	}
	for i := 0; i+1 < len(rs); i++ { // перестановка соседних
		t := append([]rune(nil), rs...)
		t[i], t[i+1] = t[i+1], t[i]
		if try(string(t)) {
			return out
		}
	}
	for i := range rs { // лишняя буква
		if try(string(rs[:i]) + string(rs[i+1:])) {
			return out
		}
	}
	for i := range rs { // не та буква
		for _, l := range c.try {
			if l == rs[i] || unicode.IsUpper(l) {
				continue
			}
			t := append([]rune(nil), rs...)
			t[i] = l
			if try(string(t)) {
				return out
			}
		}
	}
	for i := 0; i <= len(rs); i++ { // пропущенная буква
		for _, l := range c.try {
			if unicode.IsUpper(l) {
				continue
			}
			if try(string(rs[:i]) + string(l) + string(rs[i:])) {
				return out
			}
		}
	}
	return out
}

// matchCase переносит регистр исходного слова на вариант.
func matchCase(orig, s string) string {
	letters, upper := 0, 0
	for _, r := range orig {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	switch {
	case letters > 1 && upper == letters:
		return strings.ToUpper(s)
	case upper > 0 && unicode.IsUpper([]rune(orig)[0]):
		r := []rune(s)
		r[0] = unicode.ToUpper(r[0])
		return string(r)
	}
	return s
}

// Mistake — слово с ошибкой: позиция в тексте (в рунах) и варианты.
type Mistake struct {
	Word       string
	Start, End int
	Suggest    []string
}

var wordRe = regexp.MustCompile(`[\p{L}]+(?:-[\p{L}]+)*`)

// Check находит в тексте слова, которых нет в словаре. Проверяются только
// русские слова от двух букв; латиница, аббревиатуры (ЖК, СВО) и слова
// с цифрами пропускаются.
func (c *Checker) Check(text string, max int) []Mistake {
	var out []Mistake
	byteToRune := func(b int) int { return utf8.RuneCountInString(text[:b]) }
	for _, loc := range wordRe.FindAllStringIndex(text, -1) {
		w := text[loc[0]:loc[1]]
		if !cyrillic(w) || utf8.RuneCountInString(w) < 2 {
			continue
		}
		if abbrev(w) || c.Correct(w) {
			continue
		}
		out = append(out, Mistake{Word: w, Start: byteToRune(loc[0]), End: byteToRune(loc[1])})
		if len(out) >= max {
			break
		}
	}
	for i := range out {
		out[i].Suggest = c.Suggest(out[i].Word, 3)
	}
	return out
}

func cyrillic(w string) bool {
	for _, r := range w {
		if r != '-' && !unicode.Is(unicode.Cyrillic, r) {
			return false
		}
	}
	return true
}

// abbrev — короткое слово прописными (ЖК, СВО, ИП): в словаре их нет.
func abbrev(w string) bool {
	n := utf8.RuneCountInString(w)
	return n <= 5 && strings.ToUpper(w) == w
}
