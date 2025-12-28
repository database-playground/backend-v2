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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("dbplay.submission")

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
	ctx, span := tracer.Start(ctx, "SubmitAnswer",
		trace.WithAttributes(
			attribute.Int("user.id", input.SubmitterID),
			attribute.Int("question.id", input.QuestionID),
		))
	defer span.End()

	span.AddEvent("question.fetching")
	question, err := ss.entClient.Question.Get(ctx, input.QuestionID)
	if err != nil {
		if ent.IsNotFound(err) {
			span.SetStatus(otelcodes.Error, "Question not found")
			span.RecordError(err)
			return nil, ErrQuestionNotFound
		}

		span.SetStatus(otelcodes.Error, "Failed to get question")
		span.RecordError(err)
		return nil, fmt.Errorf("get question: %w", err)
	}

	span.AddEvent("database.fetching")
	database, err := question.QueryDatabase().Only(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to get database")
		span.RecordError(err)
		return nil, fmt.Errorf("get database: %w", err)
	}

	span.SetAttributes(attribute.String("database.schema", database.Schema))

	submissionModel := ss.entClient.Submission.Create().
		SetQuestion(question).
		SetUserID(input.SubmitterID).
		SetSubmittedCode(input.Answer)

	span.AddEvent("answer.running")
	result, err := ss.runAnswer(ctx, database.Schema, input.Answer, question.ReferenceAnswer)
	if err != nil {
		span.AddEvent("answer.execution.failed")
		submissionModel.SetError(err.Error())
		submissionModel.SetStatus(submission.StatusFailed)
	} else {
		submissionModel.SetQueryResult(result)

		if result.MatchAnswer {
			span.AddEvent("answer.match.success")
			submissionModel.SetStatus(submission.StatusSuccess)
		} else {
			span.AddEvent("answer.match.failed")
			submissionModel.SetStatus(submission.StatusFailed)
		}
	}

	span.AddEvent("submission.saving")
	submission, err := submissionModel.Save(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to create submission")
		span.RecordError(err)
		return nil, fmt.Errorf("create submission: %w", err)
	}

	span.SetAttributes(
		attribute.Int("submission.id", submission.ID),
		attribute.String("submission.status", string(submission.Status)),
	)

	// Write event to database
	span.AddEvent("event.triggering")
	ss.eventService.TriggerEvent(ctx, events.Event{
		Type: events.EventTypeSubmitAnswer,
		Payload: map[string]any{
			"submission_id": submission.ID,
			"question_id":   input.QuestionID,
			"status":        submission.Status,
		},
		UserID: input.SubmitterID,
	})

	span.SetStatus(otelcodes.Ok, "Answer submitted successfully")
	return submission, nil
}

// runAnswer runs both the reference answer and the users' answer, compare them,
// and return the result of this submission.
func (ss *SubmissionService) runAnswer(ctx context.Context, schema, answer, referenceAnswer string) (*models.UserSQLExecutionResult, error) {
	ctx, span := tracer.Start(ctx, "runAnswer",
		trace.WithAttributes(
			attribute.String("database.schema", schema),
		))
	defer span.End()

	// run the reference answer
	span.AddEvent("reference_answer.executing")
	referenceAnswerResponse, err := ss.sqlrunner.Query(ctx, schema, referenceAnswer)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to run reference answer")
		span.RecordError(err)
		return nil, fmt.Errorf("run reference answer: %w", err)
	}

	// run the user's answer
	span.AddEvent("user_answer.executing")
	response, err := ss.sqlrunner.Query(ctx, schema, answer)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to run user answer")
		span.RecordError(err)
		return nil, err
	}

	span.AddEvent("answer.comparing")
	matchAnswer := CompareAnswer(response, referenceAnswerResponse)
	span.SetAttributes(
		attribute.Bool("answer.match", matchAnswer),
		attribute.Int("answer.rows_count", len(response.Rows)),
		attribute.Int("answer.columns_count", len(response.Columns)),
	)

	span.SetStatus(otelcodes.Ok, "Answer execution completed")
	return &models.UserSQLExecutionResult{
		SQLExecutionResult: models.SQLExecutionResult{
			Columns: response.Columns,
			Rows:    response.Rows,
		},
		MatchAnswer: matchAnswer,
	}, nil
}

// CompareAnswer compares the answer to the reference answer.
func CompareAnswer(answer sqlrunner.DataResponse, referenceAnswer sqlrunner.DataResponse) bool {
	return reflect.DeepEqual(answer, referenceAnswer)
}
