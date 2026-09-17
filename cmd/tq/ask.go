package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// defaultQuestionTTL is how long an owner question stays open: the parked
// task re-enters when it expires (the answer never came), so this bounds
// how long a question can silently hold work back.
const defaultQuestionTTL = 72 * time.Hour

// maxQuestionTTL caps --expires: a question is a ruling request, not a
// parking spot for the week after next.
const maxQuestionTTL = 7 * 24 * time.Hour

// cmdAsk is the agent→owner question channel: an agent inside a queue task
// that is unsure how to proceed runs
//
//	tq ask --task <id> [--type info|approval|confirmation|input] \
//	       [--expires 72h] [--options a,b,c] "the question"
//
// The command validates the task is actually running (the asker is
// mid-run), redacts token-shaped secrets from the question, appends the
// task.question-asked fact (the bridge forwards it to PapDashboard), and
// writes the question marker into $TQ_QUESTION_FILE — the executor turns
// the marker into a question park after the agent's turn ends, so the task
// requeues WITHOUT burning an attempt and waits for the owner's answer.
//
// Re-ask semantics (the ref is a stable hash over task + normalized
// question text): an already-ANSWERED ref is a no-op success — the answer
// is already in the resumed prompt, the agent should honor it; an
// asked-but-pending ref re-arms the marker with a fresh expiry without
// re-forwarding the question to PapDashboard.
func cmdAsk(args []string) error {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	taskID := fs.String("task", "", "the running task this question belongs to (required)")
	qType := fs.String("type", queue.QuestionTypeInfo, "question kind: info|approval|confirmation|input")
	expires := fs.String("expires", defaultQuestionTTL.String(), "how long the question stays open before the task re-enters (e.g. 72h)")
	options := fs.String("options", "", "comma-separated answer options shown to the owner")
	dbPath := dbFlag(fs)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("tq ask: %w (usage: tq ask --task <id> [--type …] [--expires 72h] [--options a,b] 'question')", err)
	}

	if *taskID == "" {
		return errors.New("tq ask: --task is required — which running task is asking?")
	}

	if fs.NArg() != 1 {
		return errors.New("usage: tq ask --task <id> 'the question' — exactly one question text argument")
	}

	question := strings.TrimSpace(fs.Arg(0))
	if question == "" {
		return errors.New("tq ask: empty question — say what you need the owner to decide")
	}

	if !queue.ValidQuestionType(*qType) {
		return fmt.Errorf("tq ask: unknown question type %q (want info|approval|confirmation|input)", *qType)
	}

	ttl, err := time.ParseDuration(*expires)
	if err != nil || ttl <= 0 {
		return fmt.Errorf("tq ask: --expires %q is not a positive duration (e.g. 72h)", *expires)
	}

	if ttl > maxQuestionTTL {
		return fmt.Errorf("tq ask: --expires %s exceeds the %s cap — re-ask instead of parking that long", ttl, maxQuestionTTL)
	}

	store := mustOpenDB(resolveDB(*dbPath))
	defer func() { _ = store.Close() }()

	t, err := store.Get(context.Background(), task.ID(*taskID))
	if err != nil {
		return fmt.Errorf("tq ask: task %s: %w", *taskID, err)
	}

	if t.Status != task.Running {
		return fmt.Errorf(
			"tq ask: task %s is %s, not running — questions can only be asked by the task's live run (check --task; the ID comes from the prompt contract)",
			*taskID, t.Status,
		)
	}

	// Secrets never leave the machine through a question: the bridge posts
	// the text to PapDashboard, so the redaction pass runs BEFORE the fact
	// exists (facts are forever).
	question = executor.RedactSecrets(question)

	var optionList []string

	for option := range strings.SplitSeq(*options, ",") {
		if option = strings.TrimSpace(option); option != "" {
			optionList = append(optionList, option)
		}
	}

	ref := questionRef(*taskID, question)
	expiry := time.Now().Add(ttl)

	detail := queue.QuestionAskedDetail{
		Ref:       ref,
		Type:      *qType,
		Question:  question,
		Options:   optionList,
		Repo:      t.Project,
		ExpiresAt: expiry.UnixMilli(),
	}

	// Re-ask dedup: scan this task's question trail. An answered ref means
	// the ruling is already in the payload — stop asking it. A pending ref
	// means the question is already on the dashboard; just re-arm the park
	// with a fresh expiry (no duplicate forward, no new fact).
	answered, err := questionRefsAnswered(t.ID.String())
	if err != nil {
		return err
	}

	if answered[ref] {
		fmt.Fprintf(os.Stderr, "tq ask: question %s was already answered — honor the ruling in the prompt instead of re-asking\n", ref)

		return nil
	}

	asked, err := questionRefsAsked(t.ID.String())
	if err != nil {
		return err
	}

	if _, pending := asked[ref]; !pending {
		if err := store.AppendFact(context.Background(), journal.Fact{
			TaskID: *taskID,
			Type:   journal.QuestionAsked,
			Detail: mustMarshalDetail(detail),
		}); err != nil {
			return fmt.Errorf("tq ask: record question: %w", err)
		}
	}

	path := os.Getenv("TQ_QUESTION_FILE")
	if path == "" {
		return errors.New(
			"TQ_QUESTION_FILE is not set — the question was recorded, but this run cannot park on it. " +
				"Finish the turn normally; the bridge still delivers the question and the answer lands in the journal",
		)
	}

	marker, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("tq ask: marshal marker: %w", err)
	}

	if err := os.WriteFile(path, marker, 0o600); err != nil {
		return fmt.Errorf("tq ask: write marker %s: %w", path, err)
	}

	fmt.Fprintf(os.Stderr, "tq ask: question %s recorded for task %s (expires %s); end your turn — the task parks until the owner answers\n",
		ref, *taskID, expiry.Format(time.RFC3339))

	return nil
}

// questionRef is the stable correlation key across journal, PapDashboard
// row and payload injection: a short hash over task + normalized question
// text, so a re-ask after a crash or a retry converges on the same ref.
func questionRef(taskID, question string) string {
	sum := sha256.Sum256([]byte(taskID + "\x00" + normalizeQuestion(question)))

	return hex.EncodeToString(sum[:8])
}

// normalizeQuestion folds case and whitespace runs so cosmetic rewording
// (extra spaces, capitalization) keeps the same ref; real rewording forks
// the ref by design (it is a different question).
func normalizeQuestion(question string) string {
	return strings.Join(strings.Fields(strings.ToLower(question)), " ")
}

// questionRefsAsked returns ref → true for every question this task asked.
func questionRefsAsked(taskID string) (map[string]bool, error) {
	return scanQuestionRefs(taskID, journal.QuestionAsked)
}

// questionRefsAnswered returns ref → true for every question of this task
// that has a recorded ruling.
func questionRefsAnswered(taskID string) (map[string]bool, error) {
	return scanQuestionRefs(taskID, journal.QuestionAnswered)
}

func scanQuestionRefs(taskID string, ftype journal.FactType) (map[string]bool, error) {
	store := mustOpenDB(resolveDB(""))
	defer func() { _ = store.Close() }()

	trail, err := store.FactsForTask(context.Background(), taskID, 0)
	if err != nil {
		return nil, fmt.Errorf("tq ask: read fact trail: %w", err)
	}

	out := map[string]bool{}

	for _, f := range trail {
		if f.Type != ftype {
			continue
		}

		switch ftype {
		case journal.QuestionAsked:
			var parsed queue.QuestionAskedDetail
			if err := json.Unmarshal(f.Detail, &parsed); err == nil && parsed.Ref != "" {
				out[parsed.Ref] = true
			}
		case journal.QuestionAnswered:
			var parsed queue.QuestionAnsweredDetail
			if err := json.Unmarshal(f.Detail, &parsed); err == nil && parsed.Ref != "" {
				out[parsed.Ref] = true
			}
		}
	}

	return out, nil
}

func mustMarshalDetail(v any) jsontext.Value {
	raw, err := json.Marshal(v)
	if err != nil {
		return jsontext.Value("{}")
	}

	return raw
}
