package daemon

import "testing"

func TestInspectDiffForCheating(t *testing.T) {
	cleanDiff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,3 +1,6 @@
+func TestNewBehavior(t *testing.T) {
+	if got := 1 + 1; got != 2 {
+		t.Fatal("bad")
+	}
+}`
	if got := InspectDiffForCheating(cleanDiff); len(got) != 0 {
		t.Errorf("clean diff flagged: %+v", got)
	}

	skipDiff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,3 +1,4 @@
 func TestFoo(t *testing.T) {
+	t.Skip("flaky")
 }`
	got := InspectDiffForCheating(skipDiff)
	if len(got) != 1 || got[0].Kind != "test_skip" {
		t.Errorf("skip diff got %+v, want 1 test_skip", got)
	}

	deletedAssertDiff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,5 +1,2 @@
 func TestFoo(t *testing.T) {
-	if err != nil {
-		t.Fatal(err)
-	}
 }`
	got = InspectDiffForCheating(deletedAssertDiff)
	if len(got) != 1 || got[0].Kind != "deleted_test" {
		t.Errorf("deleted assert diff got %+v, want 1 deleted_test", got)
	}

	replacedAssertDiff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,4 +1,4 @@
 func TestFoo(t *testing.T) {
-	t.Fatal(err)
+	t.Fatalf("boom: %v", err)
 }`
	if got := InspectDiffForCheating(replacedAssertDiff); len(got) != 0 {
		t.Errorf("replaced assert diff flagged: %+v", got)
	}

	tamperDiff := `diff --git a/.github/workflows/ci.yml b/.github/workflows/ci.yml
--- a/.github/workflows/ci.yml
+++ b/.github/workflows/ci.yml
@@ -1,2 +1,2 @@
-run: go test ./...
+run: true`
	got = InspectDiffForCheating(tamperDiff)
	if len(got) != 1 || got[0].Kind != "config_tamper" {
		t.Errorf("config tamper diff got %+v, want 1 config_tamper", got)
	}
}
