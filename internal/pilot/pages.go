package pilot

import (
	"regexp"
	"strconv"
	"strings"
)

type page struct {
	ID  int
	URL string
}

var pageLine = regexp.MustCompile(`^(\d+): (.*?)( \[selected\])?$`)

func urlOfEntry(entry string) string {
	if !strings.HasSuffix(entry, ")") {
		return entry
	}
	if open := strings.LastIndex(entry, " ("); open >= 0 {
		return entry[open+2 : len(entry)-1]
	}
	return entry
}

func parsePages(listing string) []page {
	var pages []page
	inPages := false
	for line := range strings.SplitSeq(listing, "\n") {
		if strings.HasPrefix(line, "## ") {
			inPages = line == "## Pages"
			continue
		}
		if !inPages {
			continue
		}
		match := pageLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		id, _ := strconv.Atoi(match[1])
		pages = append(pages, page{ID: id, URL: urlOfEntry(match[2])})
	}
	return pages
}

func filterPages(text string, keep func(id int) bool) string {
	lines := strings.Split(text, "\n")
	out := lines[:0]
	inPages := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			inPages = line == "## Pages"
		} else if inPages {
			if match := pageLine.FindStringSubmatch(line); match != nil {
				id, _ := strconv.Atoi(match[1])
				if !keep(id) {
					continue
				}
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
