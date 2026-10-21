package middleware

import "testing"

func TestSeedanceTaskReadSkipsBilling(t *testing.T) {
	reads := []string{
		"/api/v3/contents/generations/tasks/task_1",
		"/v3/contents/generations/tasks/task_1",
		"/v1/contents/generations/tasks/task_1",
		"/contents/generations/tasks/task_1",
	}
	for _, path := range reads {
		if !seedanceTaskReadSkipsBilling(httpGET, path) {
			t.Fatalf("GET %s should skip billing", path)
		}
		if !seedanceTaskReadSkipsBilling(httpDELETE, path) {
			t.Fatalf("DELETE %s should skip billing", path)
		}
	}
	if seedanceTaskReadSkipsBilling(httpPOST, "/api/v3/contents/generations/tasks") {
		t.Fatal("create must stay on the billing path")
	}
	if seedanceTaskReadSkipsBilling(httpGET, "/api/v3/contents/generations/tasks") {
		t.Fatal("collection path is not a task read")
	}
	if seedanceTaskReadSkipsBilling(httpGET, "/v1/chat/completions") {
		t.Fatal("text routes stay on the billing path")
	}
}

const (
	httpGET    = "GET"
	httpPOST   = "POST"
	httpDELETE = "DELETE"
)
