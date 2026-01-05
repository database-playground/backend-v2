package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// SubmissionTotal tracks the total number of submissions with status label (success, failed, or pending)
	SubmissionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dbplay_submission_total",
			Help: "Total number of submissions by status (success, failed, or pending)",
		},
		[]string{"status"},
	)

	// QuestionAttemptedTotal tracks the total number of questions attempted
	QuestionAttemptedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbplay_question_attempted_total",
			Help: "Total number of questions attempted",
		},
	)

	// ReferenceAnswerExecutionErrorTotal tracks errors when executing reference answers
	ReferenceAnswerExecutionErrorTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbplay_reference_answer_execution_error_total",
			Help: "Total number of errors when executing reference answers",
		},
	)

	// CheatTotal tracks the total number of cheat records created
	CheatTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbplay_cheat_total",
			Help: "Total number of cheat records created",
		},
	)

	// LoginTotal tracks the total number of user logins
	LoginTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dbplay_login_total",
			Help: "Total number of user logins",
		},
	)

	// EventTotal tracks the total number of events by event type
	EventTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dbplay_event_total",
			Help: "Total number of events by event type",
		},
		[]string{"event_type"},
	)
)

// RecordSubmission records a submission with the given status
func RecordSubmission(status string) {
	SubmissionTotal.WithLabelValues(status).Inc()
}

// RecordQuestionAttempted records a question attempt
func RecordQuestionAttempted() {
	QuestionAttemptedTotal.Inc()
}

// RecordReferenceAnswerExecutionError records an error when executing a reference answer
func RecordReferenceAnswerExecutionError() {
	ReferenceAnswerExecutionErrorTotal.Inc()
}

// RecordCheat records a cheat record creation
func RecordCheat() {
	CheatTotal.Inc()
}

// RecordLogin records a user login
func RecordLogin() {
	LoginTotal.Inc()
}

// RecordEvent records an event with the given event type
func RecordEvent(eventType string) {
	EventTotal.WithLabelValues(eventType).Inc()
}
