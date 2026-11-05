package football

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/tishanyq/fpl-edge/backend/internal/domain"
	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
)

var squadNumberLine = regexp.MustCompile(`^(\d{1,3})\s+(.+)$`)

func (c *Client) enrichSquadNumbers(ctx context.Context, data *domain.FootballData) error {
	if c.squadNumbersURL == "" || data.SeasonName != "2026/27" {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.squadNumbersURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "text/html")
	request.Header.Set("User-Agent", "FPL-Edge/1.0")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("fetch official squad numbers: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("official squad numbers returned HTTP %d", response.StatusCode)
	}
	numbers, err := parseSquadNumbers(io.LimitReader(response.Body, 5<<20))
	if err != nil {
		return err
	}
	clubs := make(map[int]domain.FootballClub, len(data.Clubs))
	for _, club := range data.Clubs {
		clubs[club.ProviderID] = club
	}
	for index := range data.Players {
		if data.Players[index].SquadNumber != nil {
			continue
		}
		club, ok := clubs[data.Players[index].ClubID]
		if !ok {
			continue
		}
		clubNumbers := numbersForClub(numbers, club.Name)
		if number, found := numberForPlayer(clubNumbers, data.Players[index]); found {
			value := number
			data.Players[index].SquadNumber = &value
		}
	}
	return nil
}

func parseSquadNumbers(reader io.Reader) (map[string]map[string]int, error) {
	document, err := html.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("parse official squad numbers: %w", err)
	}
	result := map[string]map[string]int{}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "h5" {
			club := normalizeName(textContent(node))
			paragraph := nextElement(node)
			if club != "" && paragraph != nil && paragraph.Data == "p" {
				players := map[string]int{}
				for _, line := range textLines(paragraph) {
					match := squadNumberLine.FindStringSubmatch(strings.TrimSpace(line))
					if len(match) != 3 {
						continue
					}
					number, convertErr := strconv.Atoi(match[1])
					if convertErr == nil && number > 0 && number <= 999 {
						players[normalizeName(match[2])] = number
					}
				}
				if len(players) > 0 {
					result[club] = players
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if len(result) == 0 {
		return nil, fmt.Errorf("official squad number page contained no club lists")
	}
	return result, nil
}

func numbersForClub(all map[string]map[string]int, clubName string) map[string]int {
	wanted := normalizeName(clubName)
	for name, players := range all {
		if name == wanted || strings.Contains(name, wanted) || strings.Contains(wanted, name) {
			return players
		}
	}
	aliases := map[string]string{
		"man city": "manchester city", "man utd": "manchester united",
		"spurs": "tottenham hotspur", "nottm forest": "nottingham forest", "nott m forest": "nottingham forest",
		"brighton": "brighton hove albion", "wolves": "wolverhampton wanderers",
	}
	if alias := aliases[wanted]; alias != "" {
		return all[normalizeName(alias)]
	}
	return nil
}

func numberForPlayer(numbers map[string]int, player domain.FootballPlayer) (int, bool) {
	fullName := normalizeName(player.FirstName + " " + player.LastName)
	if number, ok := numbers[fullName]; ok {
		return number, true
	}
	webName := normalizeName(player.WebName)
	if webName == "" {
		return 0, false
	}
	found := 0
	matches := 0
	for name, number := range numbers {
		if name == webName || strings.HasSuffix(name, " "+webName) || strings.HasSuffix(fullName, " "+name) {
			found = number
			matches++
		}
	}
	return found, matches == 1
}

func normalizeName(value string) string {
	decomposed := norm.NFD.String(strings.ToLower(value))
	var builder strings.Builder
	space := false
	for _, character := range decomposed {
		if unicode.Is(unicode.Mn, character) {
			continue
		}
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
			space = false
		} else if !space && builder.Len() > 0 {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func textContent(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func textLines(node *html.Node) []string {
	lines := []string{""}
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.ElementNode && current.Data == "br" {
			lines = append(lines, "")
			return
		}
		if current.Type == html.TextNode {
			lines[len(lines)-1] += current.Data
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return lines
}

func nextElement(node *html.Node) *html.Node {
	for sibling := node.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type == html.ElementNode {
			return sibling
		}
	}
	return nil
}
