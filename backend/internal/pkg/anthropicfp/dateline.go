package anthropicfp

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	datelineHyphenRegex = regexp.MustCompile("Today(['’ʼʹ])s date is (\\d{4})-(\\d{2})-(\\d{2})\\.")
	datelineSlashRegex  = regexp.MustCompile("Today(['’ʼʹ])s date is (\\d{4})/(\\d{2})/(\\d{2})\\.")
	systemReminderRegex = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)
)

type DatelineHit struct {
	ApostropheVariant string
	DateSeparator     string
}

type datelineMatch struct {
	start, end       int
	apostrophe       rune
	separator        string
	year, month, day string
}

func collectMatches(text string, re *regexp.Regexp, separator string) []datelineMatch {
	indices := re.FindAllStringSubmatchIndex(text, -1)
	matches := make([]datelineMatch, 0, len(indices))
	for _, index := range indices {
		var apostrophe rune
		for _, candidate := range text[index[2]:index[3]] {
			apostrophe = candidate
			break
		}
		matches = append(matches, datelineMatch{
			start: index[0], end: index[1], apostrophe: apostrophe, separator: separator,
			year: text[index[4]:index[5]], month: text[index[6]:index[7]], day: text[index[8]:index[9]],
		})
	}
	return matches
}

func apostropheVariant(value rune) string {
	switch value {
	case '’':
		return "u2019"
	case 'ʼ':
		return "u02bc"
	case 'ʹ':
		return "u02b9"
	default:
		return "ascii"
	}
}

func normalizeText(text string) (string, []DatelineHit) {
	if !strings.Contains(text, "date is ") {
		return text, nil
	}
	matches := collectMatches(text, datelineHyphenRegex, "-")
	matches = append(matches, collectMatches(text, datelineSlashRegex, "/")...)
	if len(matches) == 0 {
		return text, nil
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].start < matches[j].start })

	var output strings.Builder
	output.Grow(len(text))
	previous := 0
	hits := make([]DatelineHit, 0, len(matches))
	for _, match := range matches {
		canonical := fmt.Sprintf("Today's date is %s-%s-%s.", match.year, match.month, match.day)
		if text[match.start:match.end] == canonical {
			continue
		}
		output.WriteString(text[previous:match.start])
		output.WriteString(canonical)
		previous = match.end
		hits = append(hits, DatelineHit{ApostropheVariant: apostropheVariant(match.apostrophe), DateSeparator: match.separator})
	}
	if len(hits) == 0 {
		return text, nil
	}
	output.WriteString(text[previous:])
	return output.String(), hits
}

func normalizeReminderText(text string) (string, []DatelineHit) {
	locations := systemReminderRegex.FindAllStringIndex(text, -1)
	if len(locations) == 0 {
		return text, nil
	}
	var output strings.Builder
	output.Grow(len(text))
	previous := 0
	var hits []DatelineHit
	for _, location := range locations {
		output.WriteString(text[previous:location[0]])
		block := text[location[0]:location[1]]
		normalized, blockHits := normalizeText(block)
		output.WriteString(normalized)
		hits = append(hits, blockHits...)
		previous = location[1]
	}
	if len(hits) == 0 {
		return text, nil
	}
	output.WriteString(text[previous:])
	return output.String(), hits
}

// NormalizeDateline canonicalizes hidden dateline variants in Anthropic
// system content. Message content is eligible only inside system-reminder
// blocks, so user prose and tool payloads remain untouched.
func NormalizeDateline(body []byte) ([]byte, []DatelineHit, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, nil, false
	}
	out := body
	var hits []DatelineHit
	changed := false

	if system := gjson.GetBytes(out, "system"); system.Exists() {
		switch {
		case system.Type == gjson.String:
			normalized, found := normalizeText(system.String())
			if len(found) > 0 {
				if next, err := sjson.SetBytes(out, "system", normalized); err == nil {
					out, hits, changed = next, append(hits, found...), true
				}
			}
		case system.IsArray():
			index := -1
			system.ForEach(func(_, block gjson.Result) bool {
				index++
				text := block.Get("text")
				if block.Get("type").String() != "text" || text.Type != gjson.String {
					return true
				}
				normalized, found := normalizeText(text.String())
				if len(found) > 0 {
					if next, err := sjson.SetBytes(out, fmt.Sprintf("system.%d.text", index), normalized); err == nil {
						out, hits, changed = next, append(hits, found...), true
					}
				}
				return true
			})
		}
	}

	if messages := gjson.GetBytes(out, "messages"); messages.IsArray() {
		messageIndex := -1
		messages.ForEach(func(_, message gjson.Result) bool {
			messageIndex++
			content := message.Get("content")
			switch {
			case content.Type == gjson.String:
				normalized, found := normalizeReminderText(content.String())
				if len(found) > 0 {
					if next, err := sjson.SetBytes(out, fmt.Sprintf("messages.%d.content", messageIndex), normalized); err == nil {
						out, hits, changed = next, append(hits, found...), true
					}
				}
			case content.IsArray():
				contentIndex := -1
				content.ForEach(func(_, block gjson.Result) bool {
					contentIndex++
					text := block.Get("text")
					if block.Get("type").String() != "text" || text.Type != gjson.String {
						return true
					}
					normalized, found := normalizeReminderText(text.String())
					if len(found) > 0 {
						path := fmt.Sprintf("messages.%d.content.%d.text", messageIndex, contentIndex)
						if next, err := sjson.SetBytes(out, path, normalized); err == nil {
							out, hits, changed = next, append(hits, found...), true
						}
					}
					return true
				})
			}
			return true
		})
	}
	if !changed {
		return body, nil, false
	}
	return out, hits, true
}
