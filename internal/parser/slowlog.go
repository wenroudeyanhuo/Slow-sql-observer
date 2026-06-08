package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"slow-sql-observer/internal/model"
)

var (
	timeLineRE    = regexp.MustCompile(`^# Time:\s*(.+)$`)
	userHostRE    = regexp.MustCompile(`^# User@Host:\s*([^\[]+?)(?:\[[^\]]*\])?\s+@\s+([^\[]*)(?:\[(.*?)\])?\s*$`)
	metricsLineRE = regexp.MustCompile(`^# Query_time:\s*([0-9.]+)\s+Lock_time:\s*([0-9.]+)\s+Rows_sent:\s*([0-9]+)\s+Rows_examined:\s*([0-9]+)\s*$`)
	useDBRE       = regexp.MustCompile(`^use\s+([^;]+);$`)
	setTSRE       = regexp.MustCompile(`^SET timestamp=([0-9]+);$`)
)

type Parser struct{}

func New() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(block string) (model.SlowQueryRecord, error) {
	lines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	record := model.SlowQueryRecord{RawBlock: strings.TrimSpace(block)}

	var sqlLines []string
	var parsedTime *time.Time
	var fallbackTime *time.Time
	var queryTime bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if matches := timeLineRE.FindStringSubmatch(line); len(matches) == 2 {
			ts, err := parseTime(matches[1])
			if err != nil {
				return model.SlowQueryRecord{}, fmt.Errorf("parse # Time line: %w", err)
			}
			parsedTime = &ts
			continue
		}

		if matches := userHostRE.FindStringSubmatch(line); len(matches) >= 4 {
			user := strings.TrimSpace(strings.TrimSuffix(matches[1], " "))
			host := strings.TrimSpace(matches[3])
			if user != "" {
				record.UserName = stringPtr(user)
			}
			if host != "" {
				record.ClientHost = stringPtr(host)
			}
			continue
		}

		if matches := metricsLineRE.FindStringSubmatch(line); len(matches) == 5 {
			value, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				return model.SlowQueryRecord{}, fmt.Errorf("parse query_time: %w", err)
			}
			record.QueryTimeSec = value
			queryTime = true

			if lockTime, err := strconv.ParseFloat(matches[2], 64); err == nil {
				record.LockTimeSec = floatPtr(lockTime)
			}
			if rowsSent, err := strconv.ParseInt(matches[3], 10, 64); err == nil {
				record.RowsSent = int64Ptr(rowsSent)
			}
			if rowsExamined, err := strconv.ParseInt(matches[4], 10, 64); err == nil {
				record.RowsExamined = int64Ptr(rowsExamined)
			}
			continue
		}

		if matches := useDBRE.FindStringSubmatch(strings.ToLower(line)); len(matches) == 2 {
			name := strings.TrimSpace(matches[1])
			record.DBName = stringPtr(name)
			continue
		}

		if matches := setTSRE.FindStringSubmatch(line); len(matches) == 2 {
			if epoch, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				ts := time.Unix(epoch, 0).UTC()
				fallbackTime = &ts
			}
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		sqlLines = append(sqlLines, line)
	}

	if parsedTime != nil {
		record.OccurredAt = *parsedTime
	} else if fallbackTime != nil {
		record.OccurredAt = *fallbackTime
	} else {
		return model.SlowQueryRecord{}, fmt.Errorf("missing event time")
	}

	if !queryTime {
		return model.SlowQueryRecord{}, fmt.Errorf("missing query_time")
	}

	record.RawSQL = strings.TrimSpace(strings.Join(sqlLines, "\n"))
	if record.RawSQL == "" {
		return model.SlowQueryRecord{}, fmt.Errorf("missing SQL body")
	}

	return record, nil
}

func parseTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999Z07:00",
		"060102 15:04:05",
	}
	for _, layout := range layouts {
		ts, err := time.Parse(layout, strings.TrimSpace(value))
		if err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

func stringPtr(value string) *string {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}
