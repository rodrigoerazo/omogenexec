package main

import (
  "context"
  "flag"
  "fmt"
	"os"
  "io"
  "sync"

  "github.com/google/logger"

	"github.com/jsannemo/omogenjudge/judgemaster/queue"
	"github.com/jsannemo/omogenjudge/storage/submissions"
	"github.com/jsannemo/omogenjudge/storage/problems"
	"github.com/jsannemo/omogenjudge/storage/models"
  runpb "github.com/jsannemo/omogenjudge/runner/api"
  rclient "github.com/jsannemo/omogenjudge/runner/client"
  filepb "github.com/jsannemo/omogenjudge/filehandler/api"
  fhclient "github.com/jsannemo/omogenjudge/filehandler/client"
)

var runner runpb.RunServiceClient
var filehandler filepb.FileHandlerServiceClient
var slotPool = sync.Pool{}

func init() {
  slotPool.Put(0)
  slotPool.Put(1)
  slotPool.Put(2)
  slotPool.Put(3)
}

func compile(ctx context.Context, s *models.Submission, output chan<- *runpb.CompiledProgram, outerr **error) {
  response, err := runner.Compile(ctx,
    &runpb.CompileRequest{
      Program: s.ToRunnerProgram(),
      OutputPath: fmt.Sprintf("/var/lib/omogen/submissions/%d/program", s.SubmissionId),
    })
  if err != nil {
    *outerr = &err
  } else {
    output <- response.Program
  }
  close(output)
}


func judge(ctx context.Context, submission *models.Submission) error {
  slot := slotPool.Get()
  defer slotPool.Put(slot)
  logger.Infof("Judging submission %d", submission.SubmissionId)
  var compileErr *error = nil
  compileOutput := make(chan *runpb.CompiledProgram)
  go compile(ctx, submission, compileOutput, &compileErr)
  submission.Status = models.StatusCompiling
  if err := submissions.Update(ctx, submission, submissions.UpdateArgs{Fields: []submissions.Field{submissions.FieldStatus}}); err != nil {
    logger.Errorf("Could not set submission as compiling: %v", err)
  }
  problems, err := problems.ListProblems(ctx, problems.ListArgs{WithTests: true}, problems.ListFilter{ProblemId: submission.ProblemId})
  if err != nil {
    return err
  }
  if len(problems) == 0 {
    return fmt.Errorf("requested problem %d, got %d problems", submission.ProblemId, len(problems))
  }
  problem := problems.First()
  testHandles := problem.TestDataFiles().ToHandlerFiles()
  resp, err := filehandler.EnsureFile(ctx, &filepb.EnsureFileRequest{Handles: testHandles})
  if err != nil {
    return err
  }
  testPathMap := make(map[string]string)
  for i, handle := range testHandles {
    testPathMap[handle.Sha256Hash] = resp.Paths[i]
  }
  compiledProgram := <-compileOutput
  if compiledProgram == nil {
    submission.Status = models.StatusCompilationFailed
    if err := submissions.Update(ctx, submission, submissions.UpdateArgs{Fields: []submissions.Field{submissions.FieldStatus}}); err != nil {
      logger.Errorf("Could not set submission as failed compilation: %v", err)
    }
    return nil
  }
  submission.Status = models.StatusRunning
  if err := submissions.Update(ctx, submission, submissions.UpdateArgs{Fields: []submissions.Field{submissions.FieldStatus}}); err != nil {
    logger.Errorf("Could not set submission as running: %v", err)
  }

  tests := problem.Tests()
  var reqTests []*runpb.TestCase
  for _, test := range tests{
    reqTests = append(reqTests, &runpb.TestCase{
      Name: fmt.Sprintf("test-%d", test.TestCaseId),
      InputPath: testPathMap[test.InputFile.Hash],
      OutputPath: testPathMap[test.OutputFile.Hash],
    })
  }
	stream, err := runner.Evaluate(ctx,
    &runpb.EvaluateRequest{
      SubmissionId: submission.SubmissionId,
      Program: compiledProgram,
      Cases: reqTests,
      TimeLimitMs: 1000,
      MemLimitKb: 1024 * 1000,
    })
  if err != nil {
    return err
  }
  for {
    verdict, err := stream.Recv()
    if err == io.EOF {
      break
    }
    if err != nil {
      return err
    }
    logger.Infof("Verdict: %v", verdict)
    switch res := verdict.Result.(type) {
    case *runpb.EvaluateResponse_TestCase:
      // TODO report this
    case *runpb.EvaluateResponse_Submission:
      submission.Verdict = models.Verdict(res.Submission.Verdict.String())
      submission.Status = models.StatusSuccessful
      if err := submissions.Update(ctx, submission, submissions.UpdateArgs{Fields: []submissions.Field{submissions.FieldVerdict, submissions.FieldStatus}}); err != nil {
        return err
      }
    case nil:
      logger.Warningf("Got empty response")
    default:
      return fmt.Errorf("Got unexpected result %T", res)
    }

  }
  return nil
}

func main() {
  flag.Parse()
  defer logger.Init("evaluator", false, true, os.Stderr).Close()

  runner = rclient.NewClient()
  filehandler = fhclient.NewClient()

  judgeQueue := make(chan *models.Submission, 1000)
  if err := queue.StartQueue(judgeQueue); err != nil {
    logger.Fatalf("Failed queue startup: %v", err)
  }
  for w := 1; w <= 10; w++ {
    go func() {
      for {
        if err := judge(context.Background(), <-judgeQueue); err != nil {
          logger.Errorf("Failed judging submission: %v", err)
        }
      }
    }()
  }
  select{ }
}
