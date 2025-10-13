package submission

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
	"github.com/database-playground/backend-v2/models"
)

type SubmissionService struct {
	entClient    *ent.Client
	eventService *events.EventService
	sqlrunner    *sqlrunner.SqlRunner
}

func NewSubmissionService(entClient *ent.Client, eventService *events.EventService, sqlrunner *sqlrunner.SqlRunner) *SubmissionService {
	return &SubmissionService{entClient: entClient, eventService: eventService, sqlrunner: sqlrunner}
}

type SubmitAnswerInput struct {
	SubmitterID int
	QuestionID  int
	Answer      string
}

var ErrQuestionNotFound = errors.New("question not found")

// SubmitAnswer submits an answer from a user to a question.
func (ss *SubmissionService) SubmitAnswer(ctx context.Context, input SubmitAnswerInput) (*ent.Submission, error) {
	question, err := ss.entClient.Question.Get(ctx, input.QuestionID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrQuestionNotFound
		}

		return nil, fmt.Errorf("get question: %w", err)
	}

	database, err := question.QueryDatabase().Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("get database: %w", err)
	}

	submissionModel := ss.entClient.Submission.Create().
		SetQuestion(question).
		SetUserID(input.SubmitterID).
		SetSubmittedCode(input.Answer)

	result, err := ss.runAnswer(ctx, database.Schema, input.Answer, question.ReferenceAnswer)
	if err != nil {
		submissionModel.SetError(err.Error())
		submissionModel.SetStatus(submission.StatusFailed)
	} else {
		submissionModel.SetQueryResult(result)

		if result.MatchAnswer {
			submissionModel.SetStatus(submission.StatusSuccess)
		} else {
			submissionModel.SetStatus(submission.StatusFailed)
		}
	}

	submission, err := submissionModel.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create submission: %w", err)
	}

	// Write event to database
	ss.eventService.TriggerEvent(ctx, events.Event{
		Type: events.EventTypeSubmitAnswer,
		Payload: map[string]any{
			"submission_id": submission.ID,
			"question_id":   input.QuestionID,
			"status":        submission.Status,
		},
		UserID: input.SubmitterID,
	})

	return submission, nil
}

// runAnswer runs both the reference answer and the users' answer, compare them,
// and return the result of this submission.
func (ss *SubmissionService) runAnswer(ctx context.Context, schema, answer, referenceAnswer string) (*models.UserSQLExecutionResult, error) {
	// run the reference answer
	referenceAnswerResponse, err := ss.sqlrunner.Query(ctx, schema, referenceAnswer)
	if err != nil {
		return nil, fmt.Errorf("run reference answer: %w", err)
	}

	// run the user's answer
	response, err := ss.sqlrunner.Query(ctx, schema, answer)
	if err != nil {
		return nil, err
	}

	return &models.UserSQLExecutionResult{
		SQLExecutionResult: models.SQLExecutionResult{
			Columns: response.Columns,
			Rows:    response.Rows,
		},
		MatchAnswer: CompareAnswer(response, referenceAnswerResponse),
	}, nil
}

// CompareAnswer compares the answer to the reference answer.
func CompareAnswer(answer sqlrunner.DataResponse, referenceAnswer sqlrunner.DataResponse) bool {
	return reflect.DeepEqual(answer, referenceAnswer)
}
