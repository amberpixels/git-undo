#\!/usr/bin/env bats

@test "simple test" {
    echo "this is a test line"
    echo "this is another line"
    run echo "command output"
    echo "$output"
}
