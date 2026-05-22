package main

import (
	"encoding/json"
	"fmt"
	"time"
)

func printJSON(v any, start time.Time) {
	elapsed := time.Since(start).Milliseconds()

	wrapper := map[string]any{
		"result":      v,
		"_elapsed_ms": elapsed,
	}
	b, _ := json.Marshal(wrapper)
	fmt.Println(string(b))
}

func printError(msg string, start time.Time) {
	elapsed := time.Since(start).Milliseconds()
	wrapper := map[string]any{
		"error":       msg,
		"_elapsed_ms": elapsed,
	}
	b, _ := json.Marshal(wrapper)
	fmt.Println(string(b))
}
