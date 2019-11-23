package problems

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/jsannemo/omogenjudge/frontend/paths"
	"github.com/jsannemo/omogenjudge/frontend/request"
	"github.com/jsannemo/omogenjudge/frontend/util"
	"github.com/jsannemo/omogenjudge/storage/models"
	"github.com/jsannemo/omogenjudge/storage/problems"
	"github.com/jsannemo/omogenjudge/storage/submissions"
)

type SubmitParams struct {
	Problem   *models.Problem
	Languages []*util.Language
}

func SubmitHandler(r *request.Request) (request.Response, error) {
	loginUrl := paths.Route(paths.Login)
	// TODO save current page location
	if r.Context.UserID == 0 {
		return request.Redirect(loginUrl), nil
	}

	vars := mux.Vars(r.Request)
	problems, err := problems.List(r.Request.Context(), problems.ListArgs{WithStatements: problems.StmtTitles}, problems.ListFilter{ShortName: vars[paths.ProblemNameArg]})
	if err != nil {
		return nil, err
	}
	if len(problems) == 0 {
		return request.NotFound(), nil
	}
	problem := problems[0]

	if r.Request.Method == http.MethodPost {
		submit := r.Request.FormValue("submission")
		language := r.Request.FormValue("language")
		l := util.GetLanguage(language)
		if l == nil {
			return request.NotFound(), nil
		}
		s := &models.Submission{
			AccountID: r.Context.UserID,
			ProblemID: problem.ProblemID,
			Language:  l.LanguageId,
			Files: []*models.SubmissionFile{
				&models.SubmissionFile{
					Path:     l.DefaultFile(),
					Contents: submit,
				},
			},
			Runs: []*models.SubmissionRun{&models.SubmissionRun{
				ProblemVersionID: problem.CurrentVersion.ProblemVersionID,
				Status:           models.StatusNew,
				Evaluation: models.Evaluation{
					Verdict: models.VerdictUnjudged,
				},
				Public: true,
			}},
		}
		err := submissions.CreateSubmission(r.Request.Context(), s, problem.CurrentVersion.ProblemVersionID)
		if err != nil {
			return request.Error(err), nil
		}
		subUrl := paths.Route(paths.Submission, paths.SubmissionIdArg, s.SubmissionID)
		return request.Redirect(subUrl), nil
	}

	return request.Template("problems_submit", &SubmitParams{problem, util.Languages()}), nil
}
