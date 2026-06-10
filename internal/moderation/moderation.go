package moderation

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type Service struct {
	dir   string
	lists *lists
}

type lists struct {
	exceptions  []term
	obscene     []term
	obsceneRoot []term
	insults     []term
	dangerous   []term
}

type term struct {
	text            string
	squeezedText    string
	compact         string
	squeezedCompact string
}

type Violations struct {
	Obscene   []string
	Insults   []string
	Dangerous []string
}

var moderationCharMap = map[rune]rune{
	'а': 'а', 'a': 'а',
	'е': 'е', 'e': 'е', 'ё': 'е',
	'о': 'о', 'o': 'о',
	'р': 'р', 'p': 'р',
	'с': 'с', 'c': 'с',
	'х': 'х', 'x': 'х',
	'у': 'у', 'y': 'у',
	'к': 'к', 'k': 'к',
	'м': 'м', 'm': 'м',
	'т': 'т', 't': 'т',
	'н': 'н', 'h': 'н',
	'в': 'в', 'b': 'в',
	'0': 'о',
	'3': 'з',
	'4': 'ч',
	'6': 'б',
}

var obsceneRootLatinMap = map[rune]rune{
	'a': 'а',
	'b': 'б',
	'c': 'с',
	'd': 'д',
	'e': 'е',
	'g': 'г',
	'h': 'х',
	'i': 'и',
	'k': 'к',
	'l': 'л',
	'm': 'м',
	'n': 'н',
	'o': 'о',
	'p': 'п',
	'r': 'р',
	's': 'с',
	't': 'т',
	'u': 'у',
	'v': 'в',
	'x': 'х',
	'y': 'у',
	'z': 'з',
}

func NewService(dir string) *Service {
	return &Service{dir: dir}
}

func (s *Service) Moderate(fields map[string]interface{}) (Violations, error) {
	lists, err := s.loadLists()
	if err != nil {
		return Violations{}, err
	}

	violations := Violations{}
	for fieldID, fieldValue := range fields {
		for _, value := range collectTextValues(fieldValue) {
			variants := removeExceptionTerms(moderationVariants(value), lists.exceptions)
			if variants.text == "" && variants.compact == "" {
				continue
			}
			if listMatches(variants, lists.obscene) || rootMatches(variants, lists.obsceneRoot, lists.exceptions) {
				violations.Obscene = append(violations.Obscene, fieldID)
			}
			if listMatches(variants, lists.insults) {
				violations.Insults = append(violations.Insults, fieldID)
			}
			if listMatches(variants, lists.dangerous) {
				violations.Dangerous = append(violations.Dangerous, fieldID)
			}
		}
	}

	violations.Obscene = uniqueStrings(violations.Obscene)
	violations.Insults = uniqueStrings(violations.Insults)
	violations.Dangerous = uniqueStrings(violations.Dangerous)
	return violations, nil
}

func (s *Service) loadLists() (*lists, error) {
	if s.lists != nil {
		return s.lists, nil
	}
	exceptions, err := readList(filepath.Join(s.dir, "exceptions.txt"))
	if err != nil {
		return nil, err
	}
	obscene, err := readList(filepath.Join(s.dir, "obscene_words.txt"))
	if err != nil {
		return nil, err
	}
	obsceneRoot, err := readList(filepath.Join(s.dir, "obscene_roots.txt"))
	if err != nil {
		return nil, err
	}
	insults, err := readList(filepath.Join(s.dir, "insults.txt"))
	if err != nil {
		return nil, err
	}
	dangerous, err := readList(filepath.Join(s.dir, "dangerous_accusations.txt"))
	if err != nil {
		return nil, err
	}
	s.lists = &lists{
		exceptions:  exceptions,
		obscene:     obscene,
		obsceneRoot: obsceneRoot,
		insults:     insults,
		dangerous:   dangerous,
	}
	return s.lists, nil
}

func readList(filePath string) ([]term, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []term{}, nil
		}
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	terms := make([]term, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		variants := moderationVariants(value)
		if variants.text == "" && variants.compact == "" {
			continue
		}
		terms = append(terms, variants)
	}
	return terms, nil
}

func collectTextValues(value interface{}) []string {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	case map[string]interface{}:
		values := make([]string, 0, 3)
		for _, key := range []string{"value", "raw_value", "processed_value"} {
			if text, ok := typed[key].(string); ok && strings.TrimSpace(text) != "" {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

func moderationVariants(value string) term {
	text := normalizeModerationText(value)
	compact := compactModerationText(value)
	return term{
		text:            text,
		squeezedText:    squeezeRepeatedChars(text),
		compact:         compact,
		squeezedCompact: squeezeRepeatedChars(compact),
	}
}

func normalizeModerationChars(value string) string {
	raw := strings.ReplaceAll(strings.ToLower(value), "ё", "е")
	hasCyrillic := regexp.MustCompile(`[а-я]`).MatchString(raw)
	var builder strings.Builder
	for _, char := range raw {
		if !hasCyrillic && char >= 'a' && char <= 'z' {
			builder.WriteRune(char)
			continue
		}
		if mapped, ok := moderationCharMap[char]; ok {
			builder.WriteRune(mapped)
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func normalizeModerationText(value string) string {
	return normalizeSpaces(replaceNonLettersWithSpace(normalizeModerationChars(value)))
}

func compactModerationText(value string) string {
	var builder strings.Builder
	for _, char := range normalizeModerationChars(value) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func replaceNonLettersWithSpace(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
		} else {
			builder.WriteRune(' ')
		}
	}
	return builder.String()
}

func normalizeSpaces(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func squeezeRepeatedChars(value string) string {
	var builder strings.Builder
	var previous rune
	repeatCount := 0
	for _, char := range value {
		if char == previous {
			repeatCount++
			if repeatCount >= 3 {
				continue
			}
		} else {
			previous = char
			repeatCount = 1
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func removeExceptionTerms(variants term, exceptions []term) term {
	return term{
		text:            removeTextTerms(variants.text, exceptions, func(item term) string { return item.text }),
		squeezedText:    removeTextTerms(variants.squeezedText, exceptions, func(item term) string { return item.squeezedText }),
		compact:         removeCompactTerms(variants.compact, exceptions, func(item term) string { return item.compact }),
		squeezedCompact: removeCompactTerms(variants.squeezedCompact, exceptions, func(item term) string { return item.squeezedCompact }),
	}
}

func removeTextTerms(value string, exceptions []term, selector func(term) string) string {
	text := " " + value + " "
	for _, exception := range exceptions {
		exceptionText := selector(exception)
		if exceptionText == "" {
			continue
		}
		text = strings.ReplaceAll(text, " "+exceptionText+" ", " ")
	}
	return normalizeSpaces(text)
}

func removeCompactTerms(value string, exceptions []term, selector func(term) string) string {
	text := value
	for _, exception := range exceptions {
		exceptionText := selector(exception)
		if exceptionText == "" {
			continue
		}
		text = strings.ReplaceAll(text, exceptionText, "")
	}
	return text
}

func listMatches(variants term, terms []term) bool {
	for _, item := range terms {
		if item.text != "" && wholeTextContains(variants.text, item.text) {
			return true
		}
		if item.squeezedText != "" && wholeTextContains(variants.squeezedText, item.squeezedText) {
			return true
		}
		if item.compact != "" && compactTermMatches(variants.text, item.compact) {
			return true
		}
		if item.squeezedCompact != "" && compactTermMatches(variants.squeezedText, item.squeezedCompact) {
			return true
		}
	}
	return false
}

func wholeTextContains(value string, phrase string) bool {
	return strings.Contains(" "+value+" ", " "+phrase+" ")
}

func compactTermMatches(text string, compactTerm string) bool {
	if compactTerm == "" {
		return false
	}
	chars := make([]string, 0, len([]rune(compactTerm)))
	for _, char := range compactTerm {
		chars = append(chars, regexp.QuoteMeta(string(char)))
	}
	pattern := `(^|\s)` + strings.Join(chars, `\s*`) + `(\s|$)`
	return regexp.MustCompile(pattern).MatchString(text)
}

func moderationTokens(variants term) []string {
	tokens := map[string]bool{}
	for _, text := range []string{variants.text, variants.squeezedText} {
		parts := strings.Fields(text)
		for _, part := range parts {
			tokens[part] = true
		}

		var letterRun strings.Builder
		for _, part := range parts {
			if len([]rune(part)) == 1 {
				letterRun.WriteString(part)
				continue
			}
			if letterRun.Len() > 1 {
				tokens[letterRun.String()] = true
			}
			letterRun.Reset()
		}
		if letterRun.Len() > 1 {
			tokens[letterRun.String()] = true
		}
	}

	result := make([]string, 0, len(tokens))
	for token := range tokens {
		result = append(result, token)
	}
	return result
}

func transliterateObsceneRootToken(token string) string {
	var builder strings.Builder
	for _, char := range token {
		if mapped, ok := obsceneRootLatinMap[char]; ok {
			builder.WriteRune(mapped)
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func rootMatches(variants term, roots []term, exceptions []term) bool {
	tokenMap := map[string]bool{}
	for _, token := range moderationTokens(variants) {
		for _, candidate := range []string{token, squeezeRepeatedChars(token), transliterateObsceneRootToken(token)} {
			if candidate == "" || tokenIsRootException(candidate, exceptions) {
				continue
			}
			tokenMap[candidate] = true
		}
	}

	for _, root := range roots {
		for _, value := range []string{root.compact, root.squeezedCompact} {
			if value == "" {
				continue
			}
			for token := range tokenMap {
				if strings.Contains(token, value) {
					return true
				}
			}
		}
	}
	return false
}

func tokenIsRootException(token string, exceptions []term) bool {
	for _, exception := range exceptions {
		if exception.compact != "" && strings.Contains(token, exception.compact) {
			return true
		}
		if exception.squeezedCompact != "" && strings.Contains(token, exception.squeezedCompact) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
