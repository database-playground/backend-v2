package submission

import (
	"context"
	"errors"
	"fmt"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
	"github.com/database-playground/backend-v2/models"
	"github.com/samber/lo"
)

type SubmissionService struct {
	entClient    *ent.Client
	eventService *events.EventService
	sqlrunner    *sqlrunner.SqlRunner
}

func NewSubmissionService(entClient *ent.Client, eventService *events.EventService) *SubmissionService {
	return &SubmissionService{entClient: entClient, eventService: eventService}
}

type SubmitAnswerInput struct {
	SubmitterID int
	QuestionID  int
	Answer      string
}

var ErrQuestionNotFound = errors.New("question not found")

func (ss *SubmissionService) SubmitAnswer(ctx context.Context, input SubmitAnswerInput) (models.SubmissionResult, error) {
	question, err := ss.entClient.Question.Get(ctx, input.QuestionID)
	if err != nil {
		if ent.IsNotFound(err) {
			return models.SubmissionResult{}, ErrQuestionNotFound
		}

		return models.SubmissionResult{}, fmt.Errorf("get question: %w", err)
	}

	database, err := question.QueryDatabase().Only(ctx)
	if err != nil {
		return models.SubmissionResult{}, fmt.Errorf("get database: %w", err)
	}

	submissionResult := models.SubmissionResult{
		Version: "1",
	}

	response, err := ss.sqlrunner.Query(ctx, database.Schema, input.Answer)
	if err != nil {
		submissionResult.Error = lo.ToPtr(err.Error())
	} else {
		submissionResult.Result = &response
	}

	// Write submission to database
	submission, err := ss.entClient.Submission.Create().
		SetQuestion(question).
		SetResult(submissionResult).
		SetUserID(input.SubmitterID).
		Save(ctx)
	if err != nil {
		return models.SubmissionResult{}, fmt.Errorf("create submission: %w", err)
	}

	// Write event to database
	ss.eventService.TriggerEvent(ctx, events.Event{
		Type: events.EventTypeSubmitAnswer,
		Payload: map[string]interface{}{
			"submission_id": submission.ID,
			"question_id":   input.QuestionID,
		},
		UserID: input.SubmitterID,
	})

	return submissionResult, nil
}
