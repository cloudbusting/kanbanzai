package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/move"
	"github.com/cloudbusting/kanbanzai/internal/question"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

func runQuestion(project *config.Project, command string, args []string, out, errOut io.Writer) int {
	var options, remaining []string
	for i := 0; i < len(args); i++ {
		if (args[i] == "--text" || args[i] == "--recommend" || args[i] == "--note" || args[i] == "--next") && i+1 < len(args) {
			remaining = append(remaining, args[i], args[i+1])
			i++
			continue
		}
		if command == "ask" && (args[i] == "--option" || strings.HasPrefix(args[i], "--option=")) {
			if args[i] == "--option" {
				if i+1 >= len(args) {
					return usageError(errOut, command, fmt.Errorf("--option needs a value"))
				}
				i++
				options = append(options, args[i])
			} else {
				options = append(options, strings.TrimPrefix(args[i], "--option="))
			}
		} else {
			remaining = append(remaining, args[i])
		}
	}
	values := map[string]bool{"text": true}
	if command == "ask" {
		values["recommend"] = true
		values["note"] = true
	} else {
		values["next"] = true
	}
	pos, flags, _, err := parse(remaining, nil, values)
	if err != nil || len(pos) != 1 {
		return usageError(errOut, command, err)
	}
	cardID, err := claim.CheckCard(project, pos[0])
	if err != nil {
		return usageError(errOut, command, err)
	}
	owner, err := claim.CheckOwner(claim.OwnerLabel(project))
	if err != nil {
		return usageError(errOut, command, err)
	}
	store, err := claim.NewStore(project)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	lock, err := store.Lock()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer lock.Close()
	now := timeutil.Now()
	record, exists := store.Load(cardID)
	live := exists && claim.Live(record, now)
	if live && record["owner"] != owner {
		fmt.Fprintf(errOut, "%s: %s\n", command, claim.Describe(record, owner))
		return 3
	}
	if command == "ask" && (!live || record["owner"] != owner) {
		fmt.Fprintf(errOut, "ask: %s needs your live claim\n", cardID)
		return 2
	}
	key, card := board.Find(board.Load(project, map[string]string{}, nil), cardID)
	if card == nil {
		fmt.Fprintf(errOut, "%s: no card %s\n", command, cardID)
		return 2
	}
	path := filepath.Join(project.Root, filepath.FromSlash(card.Path))
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	text := string(data)
	_, fields := brief.Header(text)
	today := now.Format("2006-01-02")
	if command == "ask" {
		if project.ByKey[key].Kind != "open" || project.ByKey[key].Doing == "" || card.Held != nil {
			fmt.Fprintln(errOut, "ask: card must be unheld and in a Doing column")
			return 2
		}
		by := owner
		if flags["note"] != "" {
			by += " (" + flags["note"] + ")"
		}
		text, err = question.Append(text, question.Entry{AskedBy: by, Text: flags["text"], Options: options, Recommendation: flags["recommend"], Then: fields["Next action"]}, today)
		if err != nil {
			fmt.Fprintf(errOut, "ask: %v\n", err)
			return 2
		}
		held := "waiting: " + project.Owner + " — answer in ## Questions"
		next := "answer the question in ## Questions"
		text = brief.SetFields(text, map[string]*string{"Held": &held, "Next action": &next, "Updated": &today})
		text = brief.AddProgress(text, "- "+today+": waiting for an answer in ## Questions")
		if err := writeQuestionBrief(path, text); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if err := store.Remove(cardID); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if err := store.History(map[string]any{"at": timeutil.ToISO(now), "card": cardID, "event": "release", "owner": owner, "previous_owner": owner}); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		fmt.Fprintf(out, "%s: question recorded; claim released\n", cardID)
		return 0
	}
	entries, err := question.Parse(text)
	if err != nil {
		fmt.Fprintf(errOut, "answer: %v\n", err)
		return 2
	}
	var open *question.Entry
	for i := range entries {
		if entries[i].Answer == "" || entries[i].Answer == "—" {
			open = &entries[i]
		}
	}
	if open == nil {
		fmt.Fprintln(errOut, "answer: no open question")
		return 2
	}
	if _, given := flags["text"]; !given {
		fmt.Fprintf(out, "%s\n", open.Text)
		for _, option := range open.Options {
			fmt.Fprintf(out, "- %s\n", option)
		}
		if open.Recommendation != "" {
			fmt.Fprintf(out, "Recommendation: %s\n", open.Recommendation)
		}
		return 0
	}
	if fields["Held"] != "waiting: "+project.Owner+" — answer in ## Questions" {
		fmt.Fprintln(errOut, "answer: question hold is missing or changed")
		return 2
	}
	text, err = question.Close(text, flags["text"], today+" by "+owner)
	if err != nil {
		fmt.Fprintf(errOut, "answer: %v\n", err)
		return 2
	}
	next := open.Then
	if override, ok := flags["next"]; ok {
		next = override
	}
	if strings.TrimSpace(next) == "" || strings.ContainsAny(next, "\r\n") {
		fmt.Fprintln(errOut, "answer: next action must be one nonempty line")
		return 2
	}
	text = brief.SetFields(text, map[string]*string{"Held": nil, "Next action": &next, "Updated": &today})
	if err := writeQuestionBrief(path, text); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	// A refused return is intentional: retain the recorded answer and leave an interrupted card.
	position := 0
	for i, col := range project.Columns {
		if col.Key == key {
			position = i
			break
		}
	}
	for i := position - 1; i >= 0; i-- {
		col := project.Columns[i]
		if col.Key == key {
			continue
		}
		if col.Doing == "" && (col.Next == project.ByKey[key].Doing || col.SettledNext == project.ByKey[key].Doing) && project.ByKey[key].Doing != "" {
			message, moveErr := move.Apply(project, cardID, col.Key, owner, move.Changes{QuestionReturn: true}, &now)
			if moveErr == nil {
				fmt.Fprintln(out, message)
			} else {
				var refusal *move.Refused
				if !errors.As(moveErr, &refusal) {
					fmt.Fprintf(errOut, "answer recorded; return failed: %v\n", moveErr)
					return 1
				}
				fmt.Fprintf(out, "return refused; %s remains interrupted: %v\n", cardID, moveErr)
			}
			break
		}
	}
	fmt.Fprintf(out, "%s: answer recorded\n", cardID)
	return 0
}

func writeQuestionBrief(path, text string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, []byte(text), info.Mode().Perm())
}
