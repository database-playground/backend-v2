package submission_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/event"
	"github.com/database-playground/backend-v2/ent/question"
	"github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/ent/user"
	eventsService "github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/setup"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
	submissionService "github.com/database-playground/backend-v2/internal/submission"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// Helper function to create a real SQLRunner for testing
func newTestSQLRunner(t *testing.T) *sqlrunner.SqlRunner {
	t.Helper()
	return testhelper.NewSQLRunnerClient(t)
}

// setupTestData creates test data for submission tests
func setupTestData(t *testing.T, client *ent.Client) (userID, questionID, databaseID int) {
	t.Helper()

	ctx := context.Background()

	// Setup the database with required groups and scope sets
	setupResult, err := setup.Setup(ctx, client)
	require.NoError(t, err)

	// Create a user for testing
	testUser, err := client.User.Create().
		SetName("Test User").
		SetEmail("test@example.com").
		SetGroup(setupResult.NewUserGroup).
		Save(ctx)
	require.NoError(t, err)

	// Create a test database with sample data
	testDatabase, err := client.Database.Create().
		SetSlug("test-db").
		SetDescription("Test database").
		SetSchema("CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO users (id, name) VALUES (1, 'John'), (2, 'Jane');").
		SetRelationFigure("test-figure").
		Save(ctx)
	require.NoError(t, err)

	// Create a test question
	testQuestion, err := client.Question.Create().
		SetCategory("basic-sql").
		SetDifficulty("easy").
		SetTitle("Select all users").
		SetDescription("Write a SQL query to select all users from the users table").
		SetReferenceAnswer("SELECT * FROM users;").
		SetDatabase(testDatabase).
		Save(ctx)
	require.NoError(t, err)

	return testUser.ID, testQuestion.ID, testDatabase.ID
}

func TestSubmitAnswer_Success_MatchingAnswer(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	eventService := eventsService.NewEventService(client)
	sqlRunner := newTestSQLRunner(t)

	service := submissionService.NewSubmissionService(client, eventService, sqlRunner)

	userID, questionID, _ := setupTestData(t, client)

	input := submissionService.SubmitAnswerInput{
		SubmitterID: userID,
		QuestionID:  questionID,
		Answer:      "SELECT * FROM users;",
	}

	result, err := service.SubmitAnswer(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, submission.StatusSuccess, result.Status)
	require.NotNil(t, result.QueryResult)
	require.Nil(t, result.Error)

	// Verify the query result contains the expected data
	require.Equal(t, []string{"id", "name"}, result.QueryResult.Columns)
	require.Equal(t, [][]string{{"1", "John"}, {"2", "Jane"}}, result.QueryResult.Rows)
	require.True(t, result.QueryResult.MatchAnswer)

	// Verify event was triggered
	ctx := context.Background()
	events, err := client.Event.Query().
		Where(event.UserIDEQ(userID)).
		Where(event.TypeEQ(string(eventsService.EventTypeSubmitAnswer))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, events, 1)
}

func TestSubmitAnswer_Failed_NonMatchingAnswer(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	eventService := eventsService.NewEventService(client)
	sqlRunner := newTestSQLRunner(t)

	service := submissionService.NewSubmissionService(client, eventService, sqlRunner)

	userID, questionID, _ := setupTestData(t, client)

	input := submissionService.SubmitAnswerInput{
		SubmitterID: userID,
		QuestionID:  questionID,
		Answer:      "SELECT id FROM users;", // Different from reference answer "SELECT * FROM users;"
	}

	result, err := service.SubmitAnswer(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, submission.StatusFailed, result.Status)
	require.NotNil(t, result.QueryResult)
	require.Nil(t, result.Error)

	// Verify the query result contains user's query result
	require.Equal(t, []string{"id"}, result.QueryResult.Columns)
	require.Equal(t, [][]string{{"1"}, {"2"}}, result.QueryResult.Rows)
	require.False(t, result.QueryResult.MatchAnswer)
}

func TestSubmitAnswer_Failed_UserQueryError(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	eventService := eventsService.NewEventService(client)
	sqlRunner := newTestSQLRunner(t)

	service := submissionService.NewSubmissionService(client, eventService, sqlRunner)

	userID, questionID, _ := setupTestData(t, client)

	input := submissionService.SubmitAnswerInput{
		SubmitterID: userID,
		QuestionID:  questionID,
		Answer:      "SELECT * FROM nonexistent_table;", // This will cause a real SQL error
	}

	result, err := service.SubmitAnswer(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, submission.StatusFailed, result.Status)
	require.Nil(t, result.QueryResult)
	require.NotNil(t, result.Error)
	require.Contains(t, *result.Error, "QUERY_ERROR")
}

func TestSubmitAnswer_Failed_ReferenceQueryError(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	eventService := eventsService.NewEventService(client)
	sqlRunner := newTestSQLRunner(t)

	service := submissionService.NewSubmissionService(client, eventService, sqlRunner)

	// Setup basic data but create a question with invalid reference answer
	ctx := context.Background()
	setupResult, err := setup.Setup(ctx, client)
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetName("Test User").
		SetEmail("test@example.com").
		SetGroup(setupResult.NewUserGroup).
		Save(ctx)
	require.NoError(t, err)

	testDatabase, err := client.Database.Create().
		SetSlug("test-db").
		SetDescription("Test database").
		SetSchema("CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO users (id, name) VALUES (1, 'John'), (2, 'Jane');").
		SetRelationFigure("test-figure").
		Save(ctx)
	require.NoError(t, err)

	// Create a question with invalid reference answer
	testQuestion, err := client.Question.Create().
		SetCategory("basic-sql").
		SetDifficulty("easy").
		SetTitle("Select all users").
		SetDescription("Write a SQL query to select all users from the users table").
		SetReferenceAnswer("SELECT * FROM invalid_table;"). // Invalid reference answer
		SetDatabase(testDatabase).
		Save(ctx)
	require.NoError(t, err)

	input := submissionService.SubmitAnswerInput{
		SubmitterID: testUser.ID,
		QuestionID:  testQuestion.ID,
		Answer:      "SELECT * FROM users;",
	}

	result, err := service.SubmitAnswer(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, submission.StatusFailed, result.Status)
	require.Nil(t, result.QueryResult)
	require.NotNil(t, result.Error)
	require.Contains(t, *result.Error, "run reference answer:")
}

func TestSubmitAnswer_QuestionNotFound(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	eventService := eventsService.NewEventService(client)
	sqlRunner := newTestSQLRunner(t)

	service := submissionService.NewSubmissionService(client, eventService, sqlRunner)

	userID, _, _ := setupTestData(t, client)

	input := submissionService.SubmitAnswerInput{
		SubmitterID: userID,
		QuestionID:  99999, // Non-existent question
		Answer:      "SELECT * FROM users;",
	}

	result, err := service.SubmitAnswer(context.Background(), input)

	require.Error(t, err)
	require.ErrorIs(t, err, submissionService.ErrQuestionNotFound)
	require.Nil(t, result)
}

func TestCompareAnswer(t *testing.T) {
	testCases := []struct {
		name            string
		answer          sqlrunner.DataResponse
		referenceAnswer sqlrunner.DataResponse
		expected        bool
	}{
		{
			name: "Identical responses",
			answer: sqlrunner.DataResponse{
				Columns: []string{"id", "name"},
				Rows:    [][]string{{"1", "John"}, {"2", "Jane"}},
			},
			referenceAnswer: sqlrunner.DataResponse{
				Columns: []string{"id", "name"},
				Rows:    [][]string{{"1", "John"}, {"2", "Jane"}},
			},
			expected: true,
		},
		{
			name: "Different columns",
			answer: sqlrunner.DataResponse{
				Columns: []string{"id"},
				Rows:    [][]string{{"1"}, {"2"}},
			},
			referenceAnswer: sqlrunner.DataResponse{
				Columns: []string{"id", "name"},
				Rows:    [][]string{{"1", "John"}, {"2", "Jane"}},
			},
			expected: false,
		},
		{
			name: "Different rows",
			answer: sqlrunner.DataResponse{
				Columns: []string{"id", "name"},
				Rows:    [][]string{{"1", "John"}},
			},
			referenceAnswer: sqlrunner.DataResponse{
				Columns: []string{"id", "name"},
				Rows:    [][]string{{"1", "John"}, {"2", "Jane"}},
			},
			expected: false,
		},
		{
			name: "Different order of rows",
			answer: sqlrunner.DataResponse{
				Columns: []string{"id", "name"},
				Rows:    [][]string{{"2", "Jane"}, {"1", "John"}},
			},
			referenceAnswer: sqlrunner.DataResponse{
				Columns: []string{"id", "name"},
				Rows:    [][]string{{"1", "John"}, {"2", "Jane"}},
			},
			expected: false,
		},
		{
			name: "Empty responses",
			answer: sqlrunner.DataResponse{
				Columns: []string{},
				Rows:    [][]string{},
			},
			referenceAnswer: sqlrunner.DataResponse{
				Columns: []string{},
				Rows:    [][]string{},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := submissionService.CompareAnswer(tc.answer, tc.referenceAnswer)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestNewSubmissionService(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	eventService := eventsService.NewEventService(client)
	sqlRunner := newTestSQLRunner(t)

	service := submissionService.NewSubmissionService(client, eventService, sqlRunner)

	require.NotNil(t, service)
	// Test that the service is properly initialized with dependencies
}

func TestSubmitAnswer_Integration_MultipleSubmissions(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	eventService := eventsService.NewEventService(client)
	sqlRunner := newTestSQLRunner(t)

	service := submissionService.NewSubmissionService(client, eventService, sqlRunner)

	// Create custom test data with count query
	ctx := context.Background()
	setupResult, err := setup.Setup(ctx, client)
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetName("Test User").
		SetEmail("test@example.com").
		SetGroup(setupResult.NewUserGroup).
		Save(ctx)
	require.NoError(t, err)

	// Create a test database with count query setup
	testDatabase, err := client.Database.Create().
		SetSlug("test-db").
		SetDescription("Test database").
		SetSchema("CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO users (id, name) VALUES (1, 'John'), (2, 'Jane');").
		SetRelationFigure("test-figure").
		Save(ctx)
	require.NoError(t, err)

	// Create a test question with count query
	testQuestion, err := client.Question.Create().
		SetCategory("basic-sql").
		SetDifficulty("easy").
		SetTitle("Count all users").
		SetDescription("Write a SQL query to count all users from the users table").
		SetReferenceAnswer("SELECT COUNT(*) as count FROM users;").
		SetDatabase(testDatabase).
		Save(ctx)
	require.NoError(t, err)

	// Submit multiple answers from the same user for the same question
	for i := 0; i < 3; i++ {
		input := submissionService.SubmitAnswerInput{
			SubmitterID: testUser.ID,
			QuestionID:  testQuestion.ID,
			Answer:      "SELECT COUNT(*) as count FROM users;",
		}

		result, err := service.SubmitAnswer(context.Background(), input)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, submission.StatusSuccess, result.Status)
	}

	// Verify all submissions were created
	submissions, err := client.Submission.Query().
		Where(submission.HasUserWith(user.IDEQ(testUser.ID))).
		Where(submission.HasQuestionWith(question.IDEQ(testQuestion.ID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, submissions, 3)

	// Verify all events were triggered
	eventRecords, err := client.Event.Query().
		Where(event.UserIDEQ(testUser.ID)).
		Where(event.TypeEQ(string(eventsService.EventTypeSubmitAnswer))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, eventRecords, 3)
}

func TestSubmitAnswer_EventAndSubmissionRecordGeneration(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	eventService := eventsService.NewEventService(client)
	sqlRunner := newTestSQLRunner(t)

	service := submissionService.NewSubmissionService(client, eventService, sqlRunner)

	userID, questionID, _ := setupTestData(t, client)
	ctx := context.Background()

	// Test successful submission - should create both submission record and event
	t.Run("Successful submission creates records", func(t *testing.T) {
		input := submissionService.SubmitAnswerInput{
			SubmitterID: userID,
			QuestionID:  questionID,
			Answer:      "SELECT * FROM users;",
		}

		result, err := service.SubmitAnswer(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify submission record was created with correct fields
		submissions, err := client.Submission.Query().
			Where(submission.HasUserWith(user.IDEQ(userID))).
			Where(submission.HasQuestionWith(question.IDEQ(questionID))).
			Where(submission.SubmittedCodeEQ("SELECT * FROM users;")).
			All(ctx)
		require.NoError(t, err)
		require.Len(t, submissions, 1)

		submissionRecord := submissions[0]
		require.Equal(t, submission.StatusSuccess, submissionRecord.Status)
		require.Equal(t, "SELECT * FROM users;", submissionRecord.SubmittedCode)
		require.NotNil(t, submissionRecord.QueryResult)
		require.Nil(t, submissionRecord.Error)
		require.NotZero(t, submissionRecord.SubmittedAt)

		// Verify event was created with correct data
		eventRecords, err := client.Event.Query().
			Where(event.UserIDEQ(userID)).
			Where(event.TypeEQ(string(eventsService.EventTypeSubmitAnswer))).
			All(ctx)
		require.NoError(t, err)
		require.Len(t, eventRecords, 1)

		eventRecord := eventRecords[0]
		require.Equal(t, userID, eventRecord.UserID)
		require.Equal(t, string(eventsService.EventTypeSubmitAnswer), eventRecord.Type)
		require.NotNil(t, eventRecord.Payload)
		require.NotZero(t, eventRecord.TriggeredAt)
	})

	// Test failed submission - should also create both submission record and event
	t.Run("Failed submission creates records", func(t *testing.T) {
		input := submissionService.SubmitAnswerInput{
			SubmitterID: userID,
			QuestionID:  questionID,
			Answer:      "SELECT * FROM nonexistent_table;", // Will cause SQL error
		}

		result, err := service.SubmitAnswer(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify submission record was created with error information
		failedSubmissions, err := client.Submission.Query().
			Where(submission.HasUserWith(user.IDEQ(userID))).
			Where(submission.HasQuestionWith(question.IDEQ(questionID))).
			Where(submission.SubmittedCodeEQ("SELECT * FROM nonexistent_table;")).
			All(ctx)
		require.NoError(t, err)
		require.Len(t, failedSubmissions, 1)

		failedSubmission := failedSubmissions[0]
		require.Equal(t, submission.StatusFailed, failedSubmission.Status)
		require.Equal(t, "SELECT * FROM nonexistent_table;", failedSubmission.SubmittedCode)
		require.Nil(t, failedSubmission.QueryResult) // Should be nil for failed queries
		require.NotNil(t, failedSubmission.Error)
		require.Contains(t, *failedSubmission.Error, "QUERY_ERROR")
		require.NotZero(t, failedSubmission.SubmittedAt)

		// Verify event count increased (should now have 2 events total)
		allEventRecords, err := client.Event.Query().
			Where(event.UserIDEQ(userID)).
			Where(event.TypeEQ(string(eventsService.EventTypeSubmitAnswer))).
			All(ctx)
		require.NoError(t, err)
		require.Len(t, allEventRecords, 2) // One from successful, one from failed
	})

	// Test multiple submissions create multiple records
	t.Run("Multiple submissions create multiple records", func(t *testing.T) {
		initialSubmissionCount, err := client.Submission.Query().
			Where(submission.HasUserWith(user.IDEQ(userID))).
			Count(ctx)
		require.NoError(t, err)

		initialEventCount, err := client.Event.Query().
			Where(event.UserIDEQ(userID)).
			Where(event.TypeEQ(string(eventsService.EventTypeSubmitAnswer))).
			Count(ctx)
		require.NoError(t, err)

		// Submit 3 more answers
		for i := 0; i < 3; i++ {
			input := submissionService.SubmitAnswerInput{
				SubmitterID: userID,
				QuestionID:  questionID,
				Answer:      "SELECT id FROM users;", // Different answer each time
			}

			result, err := service.SubmitAnswer(ctx, input)
			require.NoError(t, err)
			require.NotNil(t, result)
		}

		// Verify submission count increased by 3
		finalSubmissionCount, err := client.Submission.Query().
			Where(submission.HasUserWith(user.IDEQ(userID))).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, initialSubmissionCount+3, finalSubmissionCount)

		// Verify event count increased by 3
		finalEventCount, err := client.Event.Query().
			Where(event.UserIDEQ(userID)).
			Where(event.TypeEQ(string(eventsService.EventTypeSubmitAnswer))).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, initialEventCount+3, finalEventCount)
	})
}
