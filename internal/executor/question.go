package executor

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// QuestionPendingError signals that the agent parked its task on a
// question for the owner (PapDashboard questions): the run asked via
// `tq ask` and ended its turn without finishing the work. This is never
// the task's fault — the identical retry fails identically UNTIL the
// owner answers — so workers must requeue WITHOUT burning an attempt,
// delayed by RetryAfter (the question's expiry: the safety valve that
// re-enters the task if the answer never comes).
type QuestionPendingError struct {
	Cause error
	// RetryAfter is how long to wait before the next attempt: until the
	// question expires. Always positive.
	RetryAfter time.Duration
	// ResumeCloseout marks a question asked during the CLOSE-OUT turn: the
	// work turn finished and its session is alive, so the re-claim resumes
	// at closeout instead of re-running the paid work turn.
	ResumeCloseout bool
}

func (e *QuestionPendingError) Error() string {
	return fmt.Sprintf("question pending (retry after %s): %s", e.RetryAfter.Truncate(time.Second), e.Cause)
}

func (e *QuestionPendingError) Unwrap() error { return e.Cause }

// QuestionPending wraps cause in a *QuestionPendingError; nil passes
// through unchanged. A cause that is already a *QuestionPendingError is
// returned as-is, so defensive double wrapping stays a single class.
func QuestionPending(cause error, retryAfter time.Duration) error {
	if cause == nil {
		return nil
	}

	if _, ok := errors.AsType[*QuestionPendingError](cause); ok {
		return cause
	}

	return &QuestionPendingError{Cause: cause, RetryAfter: retryAfter}
}

// questionFileEnv names the per-run channel for owner questions: the
// executor hands every agent process a temp file through this variable,
// `tq ask` writes the question marker into it (the task.question-asked
// detail as JSON), and runAgent turns a written marker into a
// *QuestionPendingError — the worker parks the task until the answer.
const questionFileEnv = "TQ_QUESTION_FILE"

// minQuestionWait bounds the park when the marker's expiry is already in
// the past (a stale or nonsense marker): an unparked re-claim would
// tight-loop on the same run, so give the expiry sweep a minute.
const minQuestionWait = time.Minute

// questionPendingFrom reads the run's question marker (written by `tq ask`
// through $TQ_QUESTION_FILE) and returns the *QuestionPendingError the
// worker parks on. An absent or empty marker means no question. A corrupt
// marker is PERMANENT: the agent breached the channel contract, and
// silently completing around an owner question would hide the park.
func questionPendingFrom(path string, now time.Time) error {
	if path == "" {
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("agent: read question marker: %w", err)
	}

	body := strings.TrimSpace(string(raw))
	if body == "" {
		return nil
	}

	var asked queue.QuestionAskedDetail
	if err := json.Unmarshal(jsontext.Value(body), &asked); err != nil {
		return Permanent(fmt.Errorf("agent: question marker is not question JSON: %w: %s", err, body))
	}

	if asked.Ref == "" || asked.Question == "" {
		return Permanent(fmt.Errorf("agent: question marker missing ref/question: %s", body))
	}

	wait := minQuestionWait
	if asked.ExpiresAt > 0 {
		if until := time.UnixMilli(asked.ExpiresAt).Sub(now); until > wait {
			wait = until
		}
	}

	return QuestionPending(fmt.Errorf("agent asked: %s", asked.Question), wait)
}

// renderAnswered appends the owner's recorded answers to the prompt so the
// resumed run knows the rulings that unblocked it. The store injected them
// into the payload (RecordAnswer's "answered" array); rendering happens at
// claim time so the payload stays the single source of truth.
func renderAnswered(prompt string, answered []queue.QuestionAnsweredDetail) string {
	if len(answered) == 0 {
		return prompt
	}

	var b strings.Builder

	b.WriteString(prompt)
	b.WriteString(
		"\n\n# Answers from the owner\n\nThese rulings arrived while this task was parked on your questions. Honor them — do not re-ask what is already answered, and do not re-do work the answer supersedes.",
	)

	for _, a := range answered {
		fmt.Fprintf(&b, "\n\nQ: %s\nA: %s", a.Question, a.Answer)
	}

	return b.String()
}
