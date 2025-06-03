package loglater

// Note: The previous attribute preservation tests have been removed because
// they were testing implementation details of attribute storage that changed
// with the fix for the flaky group/attribute ordering issue.
//
// The important behavior (correct attribute and group ordering during replay)
// is now thoroughly tested by TestWithGroupAndAttributes in loglater_test.go.
