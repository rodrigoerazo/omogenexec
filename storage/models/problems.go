package models

import (
	"html/template"

	"golang.org/x/text/language"

	"github.com/jsannemo/omogenjudge/frontend/paths"
)

// A Problem is a problem users can submit solutions to.
type Problem struct {
	// The numeric problem ID. This should not be exposed externally.
	ProblemID int32 `db:"problem_id"`
	// The short name of the problem. This is suitable to use in e.g. URLs or as externally-visible identifiers.
	ShortName      string `db:"short_name"`
	Statements     []*ProblemStatement
	Author         string          `db:"author"`
	License        License         `db:"license"`
	CurrentVersion *ProblemVersion `db:"problem_version"`
}

// localizedStatement returns the statement of a problem closest to the ones given in langs.
// By default, "en" and "sv" are fallback languages
func localizedStatement(p *Problem, langs []language.Tag) *ProblemStatement {
	var has []language.Tag
	userPrefs := append(langs, language.Make("en"), language.Make("sv"))
	for _, statement := range p.Statements {
		has = append(has, language.Make(statement.Language))
	}
	var matcher = language.NewMatcher(has)
	_, index, _ := matcher.Match(userPrefs...)
	return p.Statements[index]
}

// LocalizedTitle returns the problem title with preference given to some languages.
func (p *Problem) LocalizedTitle(preferred []language.Tag) string {
	return localizedStatement(p, preferred).Title
}

// LocalizedStatement returns the problem statement with preference given to some languages.
func (p *Problem) LocalizedStatement(preferred []language.Tag) template.HTML {
	return template.HTML(localizedStatement(p, preferred).HTML)
}

// Link returns a link to the problem page for the problem.
func (p *Problem) Link() string {
	return paths.Route(paths.Problem, paths.ProblemNameArg, p.ShortName)
}

func (p *Problem) SubmitLink() string {
	return paths.Route(paths.SubmitProblem, paths.ProblemNameArg, p.ShortName)
}

// A ProblemStatement is the text statement of a problem in a given language.
type ProblemStatement struct {
	ProblemID int32 `db:"problem_id"`
	// The tag of the language that the statement is written in.
	Language string `db:"language"`
	// The title of the statement.
	Title string `db:"title"`
	// The HTML template of the statement.
	HTML string `db:"html"`
}
