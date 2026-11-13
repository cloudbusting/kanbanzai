package main

// Merge is whether a card's branch is proven to be inside the main branch's HEAD. The zero value is Unknown, so a
// Candidate nobody filled in is never treated as merged.
type Merge int

const (
	MergeUnknown Merge = iota
	MergeYes
	MergeNo
)

// Route is the part of `kanbanzai step --for ID --json` the selector reads.
type Route struct {
	Action  string
	Step    string
	Blocked bool
	Reason  string
}

// Candidate is everything the classifier knows about one card in the Checked column.
type Candidate struct {
	ID           string
	Stage        string // the board's stage word
	Column       string // the column `step --for` reports
	Owner        string // the board's claim owner, empty when free
	Held         string
	Route        Route
	SkippedSteps []string
	Merge        Merge
	Err          string // a failure reading the card; the card is left out
}

// Decision is the classifier's answer for one card. A card is left out with a reason, never asked about.
type Decision struct {
	ID      string
	Include bool
	Reason  string
}

// Classify includes a card only when every fact says it is ready to be packaged. Any other case, including a
// missing fact, leaves it out with one line saying why.
func Classify(c Candidate) Decision {
	out := func(reason string) Decision { return Decision{ID: c.ID, Reason: reason} }
	switch {
	case c.Err != "":
		return out("could not read the card: " + c.Err)
	case c.Stage != "checked" || c.Column != "checked":
		return out("not in Checked")
	case c.Owner != "":
		return out("claimed by " + c.Owner)
	case c.Held != "":
		return out("held: " + c.Held)
	case c.Route.Blocked:
		return out("blocked: " + c.Route.Reason)
	}
	for _, step := range c.SkippedSteps {
		if step == "deploy-qa" {
			return out("Deploy is None, so the card is not packaged")
		}
	}
	switch {
	case c.Route.Step != "deploy-qa":
		return out("route is not deploy-qa (" + c.Route.Step + ")")
	case c.Route.Action != "step":
		return out("route action " + c.Route.Action + " is not a fresh deploy-qa step")
	case c.Merge == MergeNo:
		return out("its branch is not merged into the main branch")
	case c.Merge != MergeYes:
		return out("could not prove its branch is merged into the main branch")
	}
	return Decision{ID: c.ID, Include: true}
}
