package executor_test

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"log"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Registering a custom executor type makes any task whose type matches run
// your Go code instead of a shell command.
func ExampleRegistry() {
	reg := executor.NewRegistry()
	reg.RegisterFunc("greet", func(_ context.Context, t task.Task) error {
		var name string
		if err := json.Unmarshal(t.Payload, &name); err != nil {
			name = "stranger"
		}

		fmt.Printf("hello %s\n", name)

		return nil
	})

	ex, err := reg.Lookup("greet")
	if err != nil {
		log.Fatal(err)
	}

	err = ex.Execute(context.Background(), task.Task{
		ID:      task.ID("t1"),
		Type:    "greet",
		Payload: jsontext.Value(`"world"`),
	})
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// hello world
}
