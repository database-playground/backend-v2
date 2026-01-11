package cli

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/database-playground/backend-v2/ent"
	entsubmission "github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/internal/submission"
)

// LatestSubmission represents a submission to be rerun
type LatestSubmission struct {
	UserID     int
	QuestionID int
	Code       string
	Submission *ent.Submission
}

// RerunAllSubmissions reruns all latest submissions for each user and question combination.
func (c *Context) RerunAllSubmissions(ctx context.Context, dryRun bool) error {
	// Get all latest submissions for each user and question
	latestSubmissions, err := c.getLatestSubmissions(ctx)
	if err != nil {
		return fmt.Errorf("get latest submissions: %w", err)
	}

	if len(latestSubmissions) == 0 {
		fmt.Println("No submissions found to rerun.")
		return nil
	}

	if dryRun {
		// In dry run mode, just display what would be executed
		return c.displayDryRun(ctx, latestSubmissions)
	}

	if c.submissionService == nil {
		return fmt.Errorf("submission service is not set")
	}

	// Create and run the TUI
	model := newRerunModel(c.submissionService, latestSubmissions, false)
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}

	return nil
}

// displayDryRun displays what would be executed in dry run mode
func (c *Context) displayDryRun(ctx context.Context, submissions []LatestSubmission) error {
	fmt.Println("\n🔍 Dry Run Mode - Preview of submissions to be rerun:")
	fmt.Println()
	fmt.Printf("Total submissions to rerun: %d\n\n", len(submissions))

	// Group by user for better readability
	userMap := make(map[int][]LatestSubmission)
	for _, sub := range submissions {
		userMap[sub.UserID] = append(userMap[sub.UserID], sub)
	}

	for userID, userSubs := range userMap {
		user, err := c.entClient.User.Get(ctx, userID)
		if err != nil {
			fmt.Printf("User %d:\n", userID)
		} else {
			fmt.Printf("User: %s (ID: %d)\n", user.Email, userID)
		}
		for _, sub := range userSubs {
			question, err := c.entClient.Question.Get(ctx, sub.QuestionID)
			if err != nil {
				fmt.Printf("  - Question ID: %d\n", sub.QuestionID)
			} else {
				fmt.Printf("  - Question: %s (ID: %d)\n", question.Title, sub.QuestionID)
			}
			fmt.Printf("    Code: %s\n", truncateString(sub.Code, 80))
		}
		fmt.Println()
	}

	fmt.Println("To actually execute these submissions, run without --dry-run flag.")
	return nil
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// getLatestSubmissions gets the latest submission for each user and question combination.
func (c *Context) getLatestSubmissions(ctx context.Context) ([]LatestSubmission, error) {
	// Get all submissions ordered by submitted_at descending
	allSubmissions, err := c.entClient.Submission.Query().
		WithQuestion().
		WithUser().
		Order(ent.Desc(entsubmission.FieldSubmittedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// Use a map to track the latest submission for each (user_id, question_id) pair
	type key struct {
		UserID     int
		QuestionID int
	}
	seen := make(map[key]bool)
	var latestSubmissions []LatestSubmission

	for _, sub := range allSubmissions {
		// Get user and question IDs from edges
		user, err := sub.Edges.UserOrErr()
		if err != nil {
			continue // Skip if user not loaded
		}
		question, err := sub.Edges.QuestionOrErr()
		if err != nil {
			continue // Skip if question not loaded
		}

		userID := user.ID
		questionID := question.ID

		k := key{UserID: userID, QuestionID: questionID}
		if !seen[k] {
			seen[k] = true
			latestSubmissions = append(latestSubmissions, LatestSubmission{
				UserID:     userID,
				QuestionID: questionID,
				Code:       sub.SubmittedCode,
				Submission: sub,
			})
		}
	}

	return latestSubmissions, nil
}

// rerunModel is the Bubble Tea model for the rerun progress UI
type rerunModel struct {
	submissionService *submission.SubmissionService
	submissions       []LatestSubmission
	currentIndex      int
	completed         int
	failed            int
	progress          progress.Model
	spinner           spinner.Model
	status            string
	err               error
	done              bool
	dryRun            bool
	mu                sync.Mutex
}

func newRerunModel(submissionService *submission.SubmissionService, submissions []LatestSubmission, dryRun bool) *rerunModel {
	prog := progress.New(progress.WithScaledGradient("#FF7CCB", "#FDFF8C"))
	prog.Width = 50

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &rerunModel{
		submissionService: submissionService,
		submissions:       submissions,
		progress:          prog,
		spinner:           s,
		status:            "Initializing...",
		dryRun:            dryRun,
	}
}

func (m *rerunModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startRerun,
	)
}

func (m *rerunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		if m.done {
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case rerunProgressMsg:
		m.mu.Lock()
		m.currentIndex = msg.Index
		m.completed = msg.Completed
		m.failed = msg.Failed
		m.status = msg.Status
		m.err = msg.Err
		m.done = msg.Done
		m.mu.Unlock()

		if m.done {
			return m, tea.Quit
		}

		// Continue processing
		return m, m.processNext()

	case rerunStartMsg:
		return m, m.processNext()

	default:
		return m, nil
	}
}

func (m *rerunModel) View() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.done {
		var result string
		result += "\n"
		if m.dryRun {
			result += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")).
				Render("🔍 Dry Run Complete!") + "\n\n"
			result += fmt.Sprintf("Total submissions that would be rerun: %d\n", len(m.submissions))
			result += lipgloss.NewStyle().Foreground(lipgloss.Color("3")).
				Render(fmt.Sprintf("Previewed: %d", m.completed)) + "\n"
			result += "\nThis was a dry run. No submissions were actually executed.\n"
			result += "Run without --dry-run to actually execute these submissions.\n"
		} else {
			result += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")).
				Render("✅ Rerun Complete!") + "\n\n"
			result += fmt.Sprintf("Total: %d\n", len(m.submissions))
			result += lipgloss.NewStyle().Foreground(lipgloss.Color("2")).
				Render(fmt.Sprintf("Completed: %d", m.completed)) + "\n"
			if m.failed > 0 {
				result += lipgloss.NewStyle().Foreground(lipgloss.Color("1")).
					Render(fmt.Sprintf("Failed: %d", m.failed)) + "\n"
			}
		}
		result += "\nPress 'q' to quit.\n"
		return result
	}

	var s string
	s += "\n"
	title := "🔄 Rerunning All Submissions"
	if m.dryRun {
		title = "🔍 Dry Run - Preview Mode"
	}
	s += lipgloss.NewStyle().Bold(true).Render(title) + "\n\n"

	// Progress bar
	percent := float64(m.completed+m.failed) / float64(len(m.submissions))
	s += fmt.Sprintf("Progress: %s %.1f%%\n", m.progress.ViewAs(percent), percent*100)
	s += "\n"

	// Status
	s += m.spinner.View() + " " + m.status + "\n"
	s += "\n"

	// Stats
	s += fmt.Sprintf("Total: %d | ", len(m.submissions))
	s += lipgloss.NewStyle().Foreground(lipgloss.Color("2")).
		Render(fmt.Sprintf("Completed: %d", m.completed))
	if m.failed > 0 {
		s += " | " + lipgloss.NewStyle().Foreground(lipgloss.Color("1")).
			Render(fmt.Sprintf("Failed: %d", m.failed))
	}
	s += "\n\n"

	// Current item
	if m.currentIndex < len(m.submissions) {
		sub := m.submissions[m.currentIndex]
		s += fmt.Sprintf("Processing: User %d, Question %d\n", sub.UserID, sub.QuestionID)
	}

	s += "\nPress 'q' to quit.\n"

	return s
}

// Messages
type rerunStartMsg struct{}

type rerunProgressMsg struct {
	Index     int
	Completed int
	Failed    int
	Status    string
	Err       error
	Done      bool
}

// Commands
func (m *rerunModel) startRerun() tea.Msg {
	return rerunStartMsg{}
}

func (m *rerunModel) processNext() tea.Cmd {
	return func() tea.Msg {
		m.mu.Lock()
		index := m.currentIndex
		completed := m.completed
		failed := m.failed
		m.mu.Unlock()

		if index >= len(m.submissions) {
			return rerunProgressMsg{
				Index:     index,
				Completed: completed,
				Failed:    failed,
				Status:    "All done!",
				Done:      true,
			}
		}

		sub := m.submissions[index]
		statusMsg := fmt.Sprintf("Processing submission %d/%d (User %d, Question %d)...",
			index+1, len(m.submissions), sub.UserID, sub.QuestionID)

		// In dry run mode, just simulate without actually executing
		if m.dryRun {
			// Simulate a small delay to show progress
			time.Sleep(50 * time.Millisecond)
			completed++
			return rerunProgressMsg{
				Index:     index + 1,
				Completed: completed,
				Failed:    failed,
				Status:    statusMsg + " (dry run)",
				Done:      false,
			}
		}

		// Rerun the submission
		ctx := context.Background()
		_, err := m.submissionService.SubmitAnswer(ctx, submission.SubmitAnswerInput{
			SubmitterID: sub.UserID,
			QuestionID:  sub.QuestionID,
			Answer:      sub.Code,
		})
		if err != nil {
			failed++
			return rerunProgressMsg{
				Index:     index + 1,
				Completed: completed,
				Failed:    failed,
				Status:    fmt.Sprintf("Failed: %v", err),
				Err:       err,
				Done:      false,
			}
		}

		completed++
		return rerunProgressMsg{
			Index:     index + 1,
			Completed: completed,
			Failed:    failed,
			Status:    statusMsg + " ✓",
			Done:      false,
		}
	}
}
