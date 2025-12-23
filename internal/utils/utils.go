package utils

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
