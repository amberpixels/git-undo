package logging

// export_test.go: only available for test builds

var SetGitRefReader = func(l *Logger, gitRef GitRefReader) {
	l.gitRef = gitRef
}
