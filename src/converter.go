package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Frontmatter struct {
	Title   string
	Date    string
	Draft   bool
	Tags    []string
	Summary string
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <old-post.md>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	inputPath := os.Args[1]

	data, err := os.ReadFile(inputPath)
	if err != nil {
		fatal(err)
	}

	fm, body, err := parsePost(data)
	if err != nil {
		fatal(err)
	}

	newPost, err := convertPost(fm, body)
	if err != nil {
		fatal(err)
	}

	outputPath := convertedPath(inputPath)

	if err := os.WriteFile(outputPath, []byte(newPost), 0644); err != nil {
		fatal(err)
	}

	fmt.Printf("Wrote %s\n", outputPath)
}

func parsePost(data []byte) (Frontmatter, []byte, error) {
	var fm Frontmatter

	// Frontmatter must start with "---" on the first line.
	if !strings.HasPrefix(string(data), "---\n") &&
		!strings.HasPrefix(string(data), "---\r\n") {
		return fm, nil, errors.New("file does not start with YAML frontmatter")
	}

	// Find the end of the opening line.
	openEnd := strings.IndexByte(string(data), '\n')
	if openEnd == -1 {
		return fm, nil, errors.New("invalid frontmatter")
	}
	openEnd++ // body of frontmatter starts here

	// Find the closing "---" line.
	rest := string(data[openEnd:])
	closeMarker := findClosingDelimiter(rest)
	if closeMarker == -1 {
		return fm, nil, errors.New("could not find closing frontmatter delimiter")
	}

	frontmatterText := rest[:closeMarker]
	after := rest[closeMarker:]

	// Remove the closing delimiter itself.
	if strings.HasPrefix(after, "---\r\n") {
		after = after[len("---\r\n"):]
	} else if strings.HasPrefix(after, "---\n") {
		after = after[len("---\n"):]
	} else if strings.HasPrefix(after, "---\r") {
		after = after[len("---\r"):]
	} else {
		after = after[len("---"):]
	}

	if err := parseFrontmatter(frontmatterText, &fm); err != nil {
		return fm, nil, err
	}

	return fm, []byte(after), nil
}

func findClosingDelimiter(s string) int {
	// Closing delimiter must be "---" at the beginning of a line.
	if strings.HasPrefix(s, "---\n") || strings.HasPrefix(s, "---\r\n") {
		return 0
	}

	for {
		idx := strings.Index(s, "\n---")
		if idx == -1 {
			return -1
		}

		rest := s[idx+1:]
		if strings.HasPrefix(rest, "---\n") ||
			strings.HasPrefix(rest, "---\r\n") ||
			strings.HasPrefix(rest, "---\r") ||
			rest == "---" {
			return idx + 1
		}

		s = s[idx+4:]
	}
}

func parseFrontmatter(text string, fm *Frontmatter) error {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	var seen = map[string]bool{}
	var i int

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if line == "" {
			i++
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid frontmatter line: %q", lines[i])
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "title":
			fm.Title = parseScalar(value)
			seen["title"] = true

		case "date":
			fm.Date = parseScalar(value)
			seen["date"] = true

		case "draft":
			v, err := strconv.ParseBool(parseScalar(value))
			if err != nil {
				return fmt.Errorf("invalid draft value %q", value)
			}
			fm.Draft = v
			seen["draft"] = true

		case "summary":
			fm.Summary = parseScalar(value)
			seen["summary"] = true

		case "tags":
			if strings.HasPrefix(value, "[") {
				tags, err := parseInlineTags(value)
				if err != nil {
					return fmt.Errorf("invalid tags: %w", err)
				}
				fm.Tags = tags
			} else if value == "" {
				// Support:
				//
				// tags:
				//   - gcc
				//   - compiler
				for i++; i < len(lines); i++ {
					tagLine := strings.TrimSpace(lines[i])

					if tagLine == "" {
						continue
					}

					if !strings.HasPrefix(tagLine, "-") {
						i--
						break
					}

					tag := strings.TrimSpace(strings.TrimPrefix(tagLine, "-"))
					fm.Tags = append(fm.Tags, parseScalar(tag))
				}
			} else {
				return fmt.Errorf("unsupported tags format: %q", value)
			}
			seen["tags"] = true

		default:
			return fmt.Errorf("unknown frontmatter field %q", key)
		}

		i++
	}

	if !seen["title"] {
		return errors.New("frontmatter is missing title")
	}
	if !seen["date"] {
		return errors.New("frontmatter is missing date")
	}
	if !seen["draft"] {
		return errors.New("frontmatter is missing draft")
	}

	return nil
}

func parseScalar(s string) string {
	s = strings.TrimSpace(s)

	// YAML single-quoted string.
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}

	// YAML/JSON-style double-quoted string.
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if v, err := strconv.Unquote(s); err == nil {
			return v
		}
	}

	return s
}

func parseInlineTags(s string) ([]string, error) {
	s = strings.TrimSpace(s)

	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, errors.New("expected [ ... ]")
	}

	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, nil
	}

	// csv.Reader conveniently handles quoted values and commas.
	r := csv.NewReader(strings.NewReader(inner))
	fields, err := r.Read()
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, field := range fields {
		tags = append(tags, parseScalar(strings.TrimSpace(field)))
	}

	return tags, nil
}

func convertPost(fm Frontmatter, body []byte) (string, error) {
	t, err := time.Parse("2006-01-02", fm.Date)
	if err != nil {
		return "", fmt.Errorf("invalid date %q: %w", fm.Date, err)
	}

	// Use the old calendar date, but force 20:00:00 at UTC+08:00.
	date := time.Date(
		t.Year(),
		t.Month(),
		t.Day(),
		20, 0, 0, 0,
		time.FixedZone("UTC+8", 8*60*60),
	)

	var out strings.Builder

	out.WriteString("---\n")
	out.WriteString(fmt.Sprintf("title: %s\n", strconv.Quote(fm.Title)))
	out.WriteString(fmt.Sprintf("date: %s\n", date.Format("2006-01-02T15:04:05-07:00")))
	out.WriteString(fmt.Sprintf("draft: %t\n", fm.Draft))
	out.WriteString("toc: false\n")
	out.WriteString("images:\n")
	out.WriteString("tags:\n")

	for _, tag := range fm.Tags {
		out.WriteString(fmt.Sprintf("  - %s\n", strconv.Quote(tag)))
	}

	out.WriteString("---\n")

	// Insert summary immediately after the frontmatter.
	if fm.Summary != "" {
		out.WriteString("\n> ")
		out.WriteString(strings.ReplaceAll(fm.Summary, "\n", "\n> "))
		out.WriteString("\n")
	}

	// Preserve the original post body exactly as it appeared after
	// the frontmatter.
	out.Write(body)

	return out.String(), nil
}

func convertedPath(inputPath string) string {
	ext := filepath.Ext(inputPath)

	if ext == "" {
		return inputPath + ".converted.md"
	}

	base := strings.TrimSuffix(inputPath, ext)
	return base + ".converted" + ext
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
