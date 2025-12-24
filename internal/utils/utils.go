package utils

import "encoding/json"

func IndexOf[T comparable](slice []T, item T) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

func Contains[T comparable](slice []T, item T) bool {
	return IndexOf(slice, item) != -1
}

func ToSet[T comparable](slice []T) map[T]struct{} {
	set := make(map[T]struct{})
	for _, v := range slice {
		set[v] = struct{}{}
	}
	return set
}

/// Example usage:
// type Task struct {
// 	ID     string
// 	Name   string
// 	Status string
// }

// byID := IndexBy(tasks, func(t *Task) string {
// 	return t.ID
// })

// if task, ok := byID["t2"]; ok {
// 	fmt.Println(task.Name)
// }

func IndexBy[T any, K comparable](items []T, keyFn func(*T) K) map[K]*T {

	index := make(map[K]*T, len(items))

	for i := range items {
		index[keyFn(&items[i])] = &items[i]
	}

	return index
}

// CoerceStruct decodes an arbitrary value (typically `any`, map[string]any, or
// an anonymous struct) into the destination struct type T via JSON round-trip.
//
// This is a pragmatic way to "cast" when the source is dynamic (e.g. unmarshalled
// JSON fields stored as `any`).
func CoerceStruct[T any](value any) (T, error) {
	var out T
	if value == nil {
		return out, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return out, err
	}
	return out, nil
}
