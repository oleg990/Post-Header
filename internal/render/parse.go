package render

import (
	"regexp"
	"strings"
	"unicode"
)

// Разбор текста поста в блоки. Правила подобраны под образцы:
//   - пустая строка или строка из одной точки («.», так в VK делают отступы) — граница блока;
//   - строка «– …», «- …», «— …», «• …» — пункт списка с висячим отступом;
//   - строка в скобках сразу после пункта — продолжение пункта;
//   - строка заглавными («БЕСПЛАТНО») или короткая с «!» — заголовок, золотом;
//   - **текст** и суммы вроде «10%», «3000р» — золотой акцент.

type spanKind int

const (
	spWord spanKind = iota
	spSpace
	spEmoji
)

type span struct {
	kind   spanKind
	text   string
	emoji  string
	accent bool
	w      float64
}

type paraKind int

const (
	pText paraKind = iota
	pHeading
	pDash
	pDashCont
)

type para struct {
	kind  paraKind
	num   string // «1.» у нумерованного пункта; пусто — пункт с тире
	spans []span
}

type block struct {
	icon  IconKind
	paras []para
}

// numRe — нумерованный пункт: «1. текст» или «2) текст».
var numRe = regexp.MustCompile(`^(\d{1,2})[.)]\s+(\S.*)$`)

var accentRe = regexp.MustCompile(`^\d[\d.,]*(%|р|₽|руб)\.?[.,!)]*$`)

func parseText(text string, es *EmojiSet, stripLeadEmoji bool) []block {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var blocks []block
	var cur *block
	prevKind := pText
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line == "." || line == "·" {
			cur = nil
			continue
		}
		if cur == nil {
			blocks = append(blocks, block{icon: IconAuto})
			cur = &blocks[len(blocks)-1]
		}
		kind, num := pText, ""
		if rest, ok := cutDash(line); ok {
			kind, line = pDash, rest
		} else if m := numRe.FindStringSubmatch(line); m != nil {
			kind, num, line = pDash, m[1]+".", m[2]
		} else if (prevKind == pDash || prevKind == pDashCont) && strings.HasPrefix(line, "(") {
			kind = pDashCont
		} else if isHeading(line, len(cur.paras) == 0) {
			kind = pHeading
		}
		spans := tokenize(line, es)
		// Первый эмодзи блока задаёт иконку; в стилях с иконками его убираем из текста.
		if len(cur.paras) == 0 && len(spans) > 0 && spans[0].kind == spEmoji {
			cur.icon = iconForEmoji(spans[0].emoji)
			if stripLeadEmoji {
				spans = trimLeftSpaces(spans[1:])
			}
		}
		cur.paras = append(cur.paras, para{kind: kind, num: num, spans: spans})
		prevKind = kind
	}
	for i := range blocks {
		if blocks[i].icon == IconAuto {
			blocks[i].icon = iconForText(blockText(blocks[i]))
		}
	}
	return blocks
}

func cutDash(line string) (string, bool) {
	for _, p := range []string{"– ", "- ", "— ", "• "} {
		if strings.HasPrefix(line, p) {
			return strings.TrimSpace(line[len(p):]), true
		}
	}
	return "", false
}

func isHeading(line string, firstInBlock bool) bool {
	upper, letters := 0, 0
	for _, r := range line {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	if letters >= 4 && float64(upper)/float64(letters) > 0.7 {
		return true
	}
	// «Приятный бонус!» — заголовок, только если открывает блок.
	return firstInBlock && letters > 0 && len([]rune(line)) <= 40 && strings.HasSuffix(line, "!")
}

func tokenize(line string, es *EmojiSet) []span {
	rs := []rune(line)
	var out []span
	var word []rune
	accent := false
	flush := func() {
		if len(word) > 0 {
			w := string(word)
			out = append(out, span{kind: spWord, text: w, accent: accent || accentRe.MatchString(w)})
			word = word[:0]
		}
	}
	for i := 0; i < len(rs); {
		if rs[i] == '*' && i+1 < len(rs) && rs[i+1] == '*' {
			flush()
			accent = !accent
			i += 2
			continue
		}
		if name, n := es.Match(rs, i); n > 0 {
			flush()
			out = append(out, span{kind: spEmoji, emoji: name})
			i += n
			continue
		}
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			flush()
			if len(out) > 0 && out[len(out)-1].kind != spSpace {
				out = append(out, span{kind: spSpace, text: " "})
			}
		case r == 0xFE0F || r == 0x200D:
			// одиночные модификаторы без эмодзи — выбрасываем
		default:
			word = append(word, r)
		}
		i++
	}
	flush()
	return out
}

func trimLeftSpaces(s []span) []span {
	for len(s) > 0 && s[0].kind == spSpace {
		s = s[1:]
	}
	return s
}

func blockText(b block) string {
	var sb strings.Builder
	for _, p := range b.paras {
		for _, s := range p.spans {
			sb.WriteString(s.text)
		}
		sb.WriteByte(' ')
	}
	return strings.ToLower(sb.String())
}

// BlockSummary — блок поста для списка «Иконки блоков» в интерфейсе.
type BlockSummary struct {
	Title string   // первые слова блока
	Icon  IconKind // иконка, выбранная автоматически
}

// Blocks разбирает текст так же, как рендер, и возвращает блоки по порядку.
func Blocks(text string, es *EmojiSet) []BlockSummary {
	var out []BlockSummary
	for _, b := range parseText(text, es, true) {
		var sb strings.Builder
		for _, p := range b.paras {
			for _, s := range p.spans {
				if s.kind != spEmoji {
					sb.WriteString(s.text)
				}
			}
			sb.WriteByte(' ')
		}
		title := []rune(strings.Join(strings.Fields(sb.String()), " "))
		if len(title) > 34 {
			title = append(title[:33], '…')
		}
		out = append(out, BlockSummary{Title: string(title), Icon: b.icon})
	}
	return out
}

// IconName — подпись иконки для интерфейса.
func IconName(k IconKind) string {
	for _, c := range IconChoices {
		if c.Kind == k {
			return c.Name
		}
	}
	return "Авто"
}
