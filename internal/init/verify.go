package initcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/check"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/doctor"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/wip"
)

// Verification is what the board looks like after a real run: whether `check` passes, what `wip --audit` still flags,
// what `doctor` calls an error, and which of the files init touched are still to be committed.
type Verification struct {
	Check       CheckResult `json:"check"`
	Audit       AuditResult `json:"audit"`
	Doctor      []string    `json:"doctor_errors"`
	Commit      []string    `json:"commit"`
	NextActions []string    `json:"next_actions"`
}

// CheckResult is `kanbanzai check`.
type CheckResult struct {
	OK     bool   `json:"ok"`
	Exit   int    `json:"exit"`
	Output string `json:"output"`
}

// AuditResult is `kanbanzai wip --audit`: its exit code and its hard findings, as it printed them.
type AuditResult struct {
	Exit     int      `json:"exit"`
	Findings []string `json:"findings"`
}

// uncommittedFlag is the one finding every fresh init leaves behind: the new files are not committed yet.
const uncommittedFlag = "uncommitted file(s)"

// Verify reloads the project and checks the result. It reads and reports; it fixes nothing and commits nothing.
func Verify(start string, plan *Plan) *Verification {
	v := &Verification{Doctor: []string{}, Commit: []string{}, NextActions: []string{}, Audit: AuditResult{Findings: []string{}}}
	root, top, common, err := config.Locate(start)
	var project *config.Project
	if err == nil {
		project, err = config.New(root, top, common, nil)
	}
	if err != nil {
		v.Check = CheckResult{OK: false, Exit: 1, Output: err.Error()}
		v.NextActions = append(v.NextActions, "the config does not load: "+err.Error())
		return v
	}

	var checkOut bytes.Buffer
	code := check.Run(project, &checkOut)
	v.Check = CheckResult{OK: code == 0, Exit: code, Output: strings.TrimSpace(checkOut.String())}

	var auditOut, auditErr bytes.Buffer
	v.Audit.Exit = wip.Report(project, wip.Options{Audit: true}, &auditOut, &auditErr)
	for _, line := range strings.Split(auditOut.String(), "\n") {
		if strings.HasPrefix(line, "  ! ") {
			v.Audit.Findings = append(v.Audit.Findings, strings.TrimPrefix(line, "  ! "))
		}
	}

	var doctorOut bytes.Buffer
	doctor.Run(start, doctor.Options{JSON: true, Out: &doctorOut})
	var report struct {
		Findings []struct{ ID, Status, Message string } `json:"findings"`
	}
	if json.Unmarshal(doctorOut.Bytes(), &report) == nil {
		for _, finding := range report.Findings {
			if finding.Status == "error" {
				v.Doctor = append(v.Doctor, finding.ID+": "+finding.Message)
			}
		}
	}

	v.Commit = uncommitted(plan)
	if len(v.Commit) > 0 {
		v.NextActions = append(v.NextActions, "commit the files listed above (init never commits): git add "+
			strings.Join(v.Commit, " ")+" && git commit -m \"Set up Kanbanzai\"")
	}
	if !v.Check.OK {
		v.NextActions = append(v.NextActions, "fix what kanbanzai check reports, then run it again")
	}
	other := 0
	for _, finding := range v.Audit.Findings {
		if !strings.Contains(finding, uncommittedFlag) {
			other++
			v.NextActions = append(v.NextActions, "wip --audit: "+finding)
		}
	}
	if other == 0 && len(v.Commit) > 0 {
		v.NextActions = append(v.NextActions, "after that commit, kanbanzai wip --audit passes")
	}
	for _, message := range v.Doctor {
		v.NextActions = append(v.NextActions, "doctor: "+message)
	}
	if len(v.NextActions) == 0 {
		v.NextActions = append(v.NextActions, "nothing: the board is sound and committed")
	}
	return v
}

// uncommitted lists the files init wrote or changed that git does not yet have committed.
func uncommitted(plan *Plan) []string {
	touched := map[string]bool{}
	for _, action := range plan.Actions {
		if action.Op != OpKeep && action.Op != OpMkdir {
			touched[action.Path] = true
		}
	}
	out, err := gitutil.GitRaw(plan.Root, "status", "--porcelain", "--untracked-files=all", "-z")
	if err != nil {
		return []string{}
	}
	found := []string{}
	for _, entry := range strings.Split(out, "\x00") {
		if len(entry) > 3 && touched[entry[3:]] {
			found = append(found, entry[3:])
		}
	}
	sort.Strings(found)
	return found
}

func renderVerify(out io.Writer, v *Verification) {
	status := "passes"
	if !v.Check.OK {
		status = "FAILS"
	}
	fmt.Fprintf(out, "\nkanbanzai check %s: %s\n", status, firstLine(v.Check.Output))
	if len(v.Audit.Findings) == 0 {
		fmt.Fprintln(out, "wip --audit: passes")
	} else {
		fmt.Fprintf(out, "wip --audit still flags %d thing(s):\n", len(v.Audit.Findings))
		for _, finding := range v.Audit.Findings {
			fmt.Fprintf(out, "  ! %s\n", finding)
		}
	}
	if len(v.Commit) > 0 {
		fmt.Fprintln(out, "files to commit:")
		for _, file := range v.Commit {
			fmt.Fprintf(out, "  %s\n", file)
		}
	}
	fmt.Fprintln(out, "next:")
	for _, action := range v.NextActions {
		fmt.Fprintf(out, "  - %s\n", action)
	}
}

func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		return text[:index]
	}
	return text
}
