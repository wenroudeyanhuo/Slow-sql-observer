package fingerprint

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"

	"slow-sql-observer/internal/model"
)

var (
	hexLiteralRE  = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)
	numberRE      = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	inListRE      = regexp.MustCompile(`\bin\s*\((?:\s*\?(?:\s*,\s*\?)*\s*)\)`)
	spaceRE       = regexp.MustCompile(`\s+`)
	selectTableRE = regexp.MustCompile(`\bfrom\s+([a-zA-Z0-9_` + "`" + `\.]+)`)
	insertTableRE = regexp.MustCompile(`\binsert\s+into\s+([a-zA-Z0-9_` + "`" + `\.]+)`)
	updateTableRE = regexp.MustCompile(`\bupdate\s+([a-zA-Z0-9_` + "`" + `\.]+)`)
	deleteTableRE = regexp.MustCompile(`\bdelete\s+from\s+([a-zA-Z0-9_` + "`" + `\.]+)`)
)

type Normalizer struct{}

func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

func (n *Normalizer) Process(sql string) model.ProcessedFingerprint {
	normalized := normalizeSQL(sql)
	return model.ProcessedFingerprint{
		Hash:          hashSQL(normalized),
		NormalizedSQL: normalized,
		SQLType:       detectSQLType(normalized),
		MainTableName: detectMainTable(normalized),
	}
}

func normalizeSQL(sql string) string {
	cleaned := stripComments(sql)
	cleaned = replaceQuotedStrings(cleaned)
	cleaned = strings.ToLower(cleaned)
	cleaned = strings.TrimSuffix(strings.TrimSpace(cleaned), ";")
	cleaned = hexLiteralRE.ReplaceAllString(cleaned, "?")
	cleaned = numberRE.ReplaceAllString(cleaned, "?")
	cleaned = normalizeValuesGroups(cleaned)
	cleaned = inListRE.ReplaceAllString(cleaned, "in (?)")
	cleaned = normalizeLimitOffset(cleaned)
	cleaned = spaceRE.ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

func hashSQL(sql string) string {
	sum := sha1.Sum([]byte(sql))
	return hex.EncodeToString(sum[:])
}

func stripComments(input string) string {
	var builder strings.Builder
	inSingle := false
	inDouble := false
	inBacktick := false

	for i := 0; i < len(input); i++ {
		ch := input[i]
		next := byte(0)
		if i+1 < len(input) {
			next = input[i+1]
		}

		if !inSingle && !inDouble && !inBacktick {
			if ch == '-' && next == '-' {
				for i < len(input) && input[i] != '\n' {
					i++
				}
				if i < len(input) {
					builder.WriteByte('\n')
				}
				continue
			}
			if ch == '#' {
				for i < len(input) && input[i] != '\n' {
					i++
				}
				if i < len(input) {
					builder.WriteByte('\n')
				}
				continue
			}
			if ch == '/' && next == '*' {
				i += 2
				for i < len(input)-1 {
					if input[i] == '*' && input[i+1] == '/' {
						i++
						break
					}
					i++
				}
				builder.WriteByte(' ')
				continue
			}
		}

		switch ch {
		case '\'':
			if !inDouble && !inBacktick && !isEscaped(input, i) {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick && !isEscaped(input, i) {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		}

		builder.WriteByte(ch)
	}

	return builder.String()
}

func replaceQuotedStrings(input string) string {
	var builder strings.Builder
	inQuote := false
	var quote byte

	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inQuote {
			if ch == quote && !isEscaped(input, i) {
				inQuote = false
				builder.WriteByte('?')
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inQuote = true
			quote = ch
			continue
		}
		builder.WriteByte(ch)
	}

	return builder.String()
}

func isEscaped(input string, index int) bool {
	backslashes := 0
	for i := index - 1; i >= 0 && input[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func normalizeValuesGroups(sql string) string {
	idx := strings.Index(sql, "values")
	if idx == -1 {
		return sql
	}
	after := sql[idx+len("values"):]
	groups, end := parseValuesGroups(after)
	if len(groups) <= 1 || end == 0 {
		return sql
	}
	return sql[:idx+len("values")] + " " + groups[0] + after[end:]
}

func parseValuesGroups(input string) ([]string, int) {
	var groups []string
	i := 0
	for i < len(input) && input[i] == ' ' {
		i++
	}
	for i < len(input) {
		if input[i] != '(' {
			break
		}
		depth := 0
		groupStart := i
		for i < len(input) {
			if input[i] == '(' {
				depth++
			} else if input[i] == ')' {
				depth--
				if depth == 0 {
					i++
					groups = append(groups, strings.TrimSpace(input[groupStart:i]))
					break
				}
			}
			i++
		}
		for i < len(input) && input[i] == ' ' {
			i++
		}
		if i >= len(input) || input[i] != ',' {
			break
		}
		i++
		for i < len(input) && input[i] == ' ' {
			i++
		}
	}
	if len(groups) == 0 {
		return nil, 0
	}
	return groups, i
}

func normalizeLimitOffset(sql string) string {
	sql = strings.ReplaceAll(sql, "limit ? , ?", "limit ?, ?")
	sql = strings.ReplaceAll(sql, "limit ?,?", "limit ?, ?")
	sql = strings.ReplaceAll(sql, "offset ?", "offset ?")
	return sql
}

func detectSQLType(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return "UNKNOWN"
	}
	return strings.ToUpper(fields[0])
}

func detectMainTable(sql string) *string {
	var match []string
	switch detectSQLType(sql) {
	case "SELECT":
		match = selectTableRE.FindStringSubmatch(sql)
	case "INSERT":
		match = insertTableRE.FindStringSubmatch(sql)
	case "UPDATE":
		match = updateTableRE.FindStringSubmatch(sql)
	case "DELETE":
		match = deleteTableRE.FindStringSubmatch(sql)
	}
	if len(match) < 2 {
		return nil
	}
	table := strings.Trim(match[1], "`")
	return &table
}
