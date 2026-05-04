package cli

import "testing"

func TestParseJUnitResults(t *testing.T) {
	results, err := parseJUnitResults(map[string]string{"junit.xml": `<testsuite name="pkg" tests="2" failures="1" errors="0" skipped="0" time="1.5">
<testcase classname="pkg.TestThing" name="passes"/>
<testcase classname="pkg.TestThing" name="fails" file="thing_test.go"><failure message="want ok">details</failure></testcase>
</testsuite>`})
	if err != nil {
		t.Fatal(err)
	}
	if results == nil || results.Tests != 2 || results.Failures != 1 || results.Errors != 0 || len(results.Failed) != 1 {
		t.Fatalf("unexpected results: %#v", results)
	}
	if results.Failed[0].Name != "fails" || results.Failed[0].File != "thing_test.go" {
		t.Fatalf("unexpected failure: %#v", results.Failed[0])
	}
}

func TestParseJUnitResultsInitializesEmptyFailureList(t *testing.T) {
	results, err := parseJUnitResults(map[string]string{"junit.xml": `<testsuite name="pkg" tests="1" failures="0" errors="0" skipped="0" time="0.1">
<testcase classname="pkg.TestThing" name="passes"/>
</testsuite>`})
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("results nil")
	}
	if results.Failed == nil {
		t.Fatalf("failed slice is nil: %#v", results)
	}
	if len(results.Failed) != 0 {
		t.Fatalf("failed=%#v", results.Failed)
	}
}

func TestParseMarkedFiles(t *testing.T) {
	files := parseMarkedFiles("\n__CRABBOX_RESULT_FILE__:a.xml\n<a/>\n__CRABBOX_RESULT_FILE__:b.xml\n<b/>\n")
	if files["a.xml"] != "<a/>" || files["b.xml"] != "<b/>" {
		t.Fatalf("files=%#v", files)
	}
}
