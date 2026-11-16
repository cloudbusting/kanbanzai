package number

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// BareNumber is value as a card number when it is only digits (07 and 123 are, TASK-07 and #7 are not).
func BareNumber(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return 0, false
	}
	return number, true
}

// Cards is every card that has a brief in any worktree, by number: the canonical ids that carry it, sorted. Only a
// brief's file name counts, in any folder (Done and Paused too); one brief checked out in two worktrees is one card.
func Cards(project *config.Project) map[int][]string {
	found := map[int][]string{}
	inside, err := filepath.Rel(project.Root, project.Work)
	external := err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator))
	for _, root := range Roots(project) {
		briefs := project.Work
		if !external {
			briefs = filepath.Join(root, inside)
		}
		if info, err := os.Stat(briefs); err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(briefs, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			match := project.CardFileRe.FindStringSubmatch(entry.Name())
			if match == nil {
				return nil
			}
			card, ok := project.CanonicalCardID(match[1])
			number, hasNumber := project.CardNumber(match[1])
			if ok && hasNumber && !contains(found[number], card) {
				found[number] = append(found[number], card)
			}
			return nil
		})
	}
	for number := range found {
		sort.Strings(found[number])
	}
	return found
}

// Resolve is the canonical id a command argument names. A full id (any case or padding) is its canonical form, with
// no lookup. A bare number is the one card whose brief carries it, in any worktree; no card, or two cards with the
// number, is an error that names it: a command never guesses which card was meant.
func Resolve(project *config.Project, value string) (string, error) {
	value = strings.TrimSpace(value)
	if card, ok := project.CanonicalCardID(value); ok {
		return card, nil
	}
	number, ok := BareNumber(value)
	if !ok {
		return "", numberErr("card must look like %s or a number: '%s'", strings.Join(project.IDExamples(), " or "), value)
	}
	switch cards := Cards(project)[number]; len(cards) {
	case 0:
		return "", numberErr("no card numbered %d", number)
	case 1:
		return cards[0], nil
	default:
		return "", numberErr("number %d is on more than one card: %s", number, strings.Join(cards, ", "))
	}
}

// reservedItem is the item a ledger reservation for number was made for, or "".
func reservedItem(reserved []map[string]any, number int) string {
	for _, record := range reserved {
		if value, ok := numberOf(record["number"]); ok && value == number {
			if item, ok := record["item"].(string); ok {
				return item
			}
		}
	}
	return ""
}

// numberLabel is what release and mark print for a bare number: the ledger's item for it, else the number.
func numberLabel(reserved []map[string]any, number int) string {
	if item := reservedItem(reserved, number); item != "" {
		return item
	}
	return fmt.Sprint(number)
}
