package models

import (
	"github.com/jsannemo/omogenjudge/frontend/paths"
	runpb "github.com/jsannemo/omogenjudge/runner/api"
)

type SubmissionList []*Submission

type Submission struct {
	// This may exposed externally.
	SubmissionId int32 `db:"submission_id"`

	// The author of the submission.
	AccountId int32 `db:"account_id"`

	// The problem the submission was for.
	ProblemId int32 `db:"problem_id"`

	Language string

	Status Status

	Verdict Verdict

	Files []*SubmissionFile
}

func (s *Submission) ToRunnerProgram() *runpb.Program {
	var files []*runpb.SourceFile
	for _, file := range s.Files {
		files = append(files, &runpb.SourceFile{
			Path:     file.Path,
			Contents: file.Contents,
		})
	}
	return &runpb.Program{
		Sources:    files,
		LanguageId: s.Language,
	}
}

// Link returns the link to this problem.
func (s *Submission) Link() string {
	return paths.Route(paths.Submission, paths.SubmissionIdArg, s.SubmissionId)
}

func (s *Submission) StatusString() string {
	if s.Status != StatusSuccessful {
		return s.Status.String()
	} else {
		return s.Verdict.String()
	}
}

type SubmissionFile struct {
	SubmissionId int32 `db:"submission_id"`

	Path string `db:"file_path"`

	Contents string `db:"file_contents"`
}