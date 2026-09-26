package companion

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// RecordAnswer records an owner's decision for a parked task's question.
// Same-transaction payload injection + NotBefore clear + fact, idempotent
// per question ref — tq semantics over the shared tasks/facts tables.
func RecordAnswer(ctx context.Context, d Dialect, db Beginner, id task.ID, ans queue.AnswerRecord) error {
	if ans.Ref == "" {
		return queue.ErrEmptyAnswerRef
	}

	if strings.TrimSpace(ans.Answer) == "" {
		return queue.ErrEmptyAnswer
	}

	now := time.Now()

	answeredAt := ans.AnsweredAt
	if answeredAt.IsZero() {
		answeredAt = now
	}

	return WithTx(ctx, db, d, func(r Runner) error {
		answered, err := factDetailRefs(ctx, r, id.String(), journal.QuestionAnswered)
		if err != nil {
			return err
		}

		if _, done := answered[ans.Ref]; done {
			return nil
		}

		var status string

		var payload string

		err = r.QueryRowContext(ctx,
			`SELECT status, payload FROM tasks WHERE id = ?`, id.String()).
			Scan(&status, &payload)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return task.ErrNotFound
			}

			return err
		}

		question := ans.Question
		if question == "" {
			if question, err = askedQuestionText(ctx, r, id, ans.Ref); err != nil {
				return err
			}
		}

		detail := queue.QuestionAnsweredDetail{
			Ref:        ans.Ref,
			Question:   question,
			Answer:     ans.Answer,
			PapID:      ans.PapID,
			AnsweredAt: answeredAt.UnixMilli(),
		}

		if status == "pending" {
			if err := unblockParkedTask(ctx, r, id, payload, question, ans, now, answeredAt); err != nil {
				return err
			}
		}

		return appendFact(ctx, r, journal.Fact{
			TaskID: id.String(),
			Type:   journal.QuestionAnswered,
			Detail: MustJSON(detail),
		})
	})
}

// askedQuestionText backfills the question text from the task's
// task.question-asked fact when the answer record does not carry it.
func askedQuestionText(ctx context.Context, r Runner, id task.ID, ref string) (string, error) {
	asked, err := factDetailRefs(ctx, r, id.String(), journal.QuestionAsked)
	if err != nil {
		return "", err
	}

	return asked[ref], nil
}

// unblockParkedTask injects the answer into a PARKED task's payload and
// clears its NotBefore so the re-claim is immediate.
func unblockParkedTask(
	ctx context.Context,
	r Runner,
	id task.ID,
	payload, question string,
	ans queue.AnswerRecord,
	now, answeredAt time.Time,
) error {
	merged, ok, err := mergeAnsweredPayload(payload, ans, question, answeredAt)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	_, err = r.ExecContext(ctx, `
		UPDATE tasks
		SET payload = ?, not_before = ?, updated_at = ?
		WHERE id = ? AND status = 'pending'`,
		string(merged), now.UnixMilli(), now.UnixMilli(), id.String())

	return err
}

// factDetailRefs scans a task's facts of one question type and returns
// ref -> text.
func factDetailRefs(ctx context.Context, r Runner, taskID string, ftype journal.FactType) (map[string]string, error) {
	rows, err := r.QueryContext(ctx,
		`SELECT detail FROM facts WHERE task_id = ? AND type = ?`,
		taskID, string(ftype))
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	out := map[string]string{}

	for rows.Next() {
		var detail string

		if err := rows.Scan(&detail); err != nil {
			return nil, err
		}

		switch ftype {
		case journal.QuestionAsked:
			var parsed queue.QuestionAskedDetail
			if err := json.Unmarshal(jsontext.Value(detail), &parsed); err != nil {
				continue
			}

			out[parsed.Ref] = parsed.Question
		case journal.QuestionAnswered:
			var parsed queue.QuestionAnsweredDetail
			if err := json.Unmarshal(jsontext.Value(detail), &parsed); err != nil {
				continue
			}

			out[parsed.Ref] = parsed.Answer
		default:
		}
	}

	return out, rows.Err()
}

// mergeAnsweredPayload injects one answered question into a JSON-object
// payload's "answered" array. Raw (non-object) payloads report ok=false.
func mergeAnsweredPayload(
	payload string,
	ans queue.AnswerRecord,
	question string,
	answeredAt time.Time,
) (jsontext.Value, bool, error) {
	trimmed := strings.TrimSpace(payload)
	if !strings.HasPrefix(trimmed, "{") {
		return jsontext.Value(payload), false, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(jsontext.Value(trimmed), &obj); err != nil {
		return jsontext.Value(payload), false, nil
	}

	answered, _ := obj["answered"].([]any)
	answered = append(answered, map[string]any{
		"ref":         ans.Ref,
		"question":    question,
		"answer":      ans.Answer,
		"pap_id":      ans.PapID,
		"answered_at": answeredAt.UnixMilli(),
	})
	obj["answered"] = answered

	merged, err := json.Marshal(obj)
	if err != nil {
		return jsontext.Value(payload), false, err
	}

	return merged, true, nil
}
