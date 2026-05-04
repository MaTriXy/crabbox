package cli

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type junitTestSuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Name     string           `xml:"name,attr"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Skipped  int              `xml:"skipped,attr"`
	Time     float64          `xml:"time,attr"`
	Suites   []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string         `xml:"name,attr"`
	Classname string         `xml:"classname,attr"`
	File      string         `xml:"file,attr"`
	Time      float64        `xml:"time,attr"`
	Failures  []junitFailure `xml:"failure"`
	Errors    []junitFailure `xml:"error"`
	Skipped   []struct{}     `xml:"skipped"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

func parseJUnitResults(files map[string]string) (*TestResultSummary, error) {
	if len(files) == 0 {
		return nil, nil
	}
	summary := &TestResultSummary{
		Format: "junit",
		Files:  make([]string, 0, len(files)),
		Failed: []TestFailure{},
	}
	for name, data := range files {
		trimmed := strings.TrimSpace(data)
		if trimmed == "" {
			continue
		}
		summary.Files = append(summary.Files, name)
		if err := addJUnitFile(summary, strings.NewReader(trimmed)); err != nil {
			return nil, fmt.Errorf("parse junit %s: %w", name, err)
		}
	}
	if len(summary.Files) == 0 {
		return nil, nil
	}
	return summary, nil
}

func addJUnitFile(summary *TestResultSummary, input io.Reader) error {
	decoder := xml.NewDecoder(input)
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "testsuites":
			var suites junitTestSuites
			if err := decoder.DecodeElement(&suites, &start); err != nil {
				return err
			}
			addJUnitSuites(summary, suites)
			return nil
		case "testsuite":
			var suite junitTestSuite
			if err := decoder.DecodeElement(&suite, &start); err != nil {
				return err
			}
			addJUnitSuite(summary, suite)
			return nil
		default:
			return fmt.Errorf("unsupported root element %q", start.Name.Local)
		}
	}
}

func addJUnitSuites(summary *TestResultSummary, suites junitTestSuites) {
	if len(suites.Suites) == 0 {
		summary.Suites++
		summary.Tests += suites.Tests
		summary.Failures += suites.Failures
		summary.Errors += suites.Errors
		summary.Skipped += suites.Skipped
		summary.TimeSeconds += suites.Time
		return
	}
	for _, suite := range suites.Suites {
		addJUnitSuite(summary, suite)
	}
}

func addJUnitSuite(summary *TestResultSummary, suite junitTestSuite) {
	summary.Suites++
	if suite.Tests > 0 || suite.Failures > 0 || suite.Errors > 0 || suite.Skipped > 0 {
		summary.Tests += suite.Tests
		summary.Failures += suite.Failures
		summary.Errors += suite.Errors
		summary.Skipped += suite.Skipped
		summary.TimeSeconds += suite.Time
	} else {
		summary.Tests += len(suite.TestCases)
		for _, tc := range suite.TestCases {
			summary.TimeSeconds += tc.Time
			if len(tc.Failures) > 0 {
				summary.Failures += len(tc.Failures)
			}
			if len(tc.Errors) > 0 {
				summary.Errors += len(tc.Errors)
			}
			if len(tc.Skipped) > 0 {
				summary.Skipped += len(tc.Skipped)
			}
		}
	}
	for _, tc := range suite.TestCases {
		for _, failure := range tc.Failures {
			summary.Failed = append(summary.Failed, testFailureFromJUnit(suite, tc, failure, "failure"))
		}
		for _, failure := range tc.Errors {
			summary.Failed = append(summary.Failed, testFailureFromJUnit(suite, tc, failure, "error"))
		}
	}
}

func testFailureFromJUnit(suite junitTestSuite, tc junitTestCase, failure junitFailure, kind string) TestFailure {
	message := strings.TrimSpace(failure.Message)
	if message == "" {
		message = strings.TrimSpace(failure.Text)
	}
	return TestFailure{
		Suite:     suite.Name,
		Name:      tc.Name,
		Classname: tc.Classname,
		File:      tc.File,
		Message:   message,
		Type:      failure.Type,
		Kind:      kind,
	}
}
