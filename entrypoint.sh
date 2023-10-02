#!/bin/sh

# Start the Go application as a background process
/usr/local/bin/app -- "$@" &

# Store the process ID of the Go application
PID=$!

# Function to forward signals to the child process
forward_signals() {
    # Forward SIGTERM and SIGINT to the child process
    trap 'kill -TERM $PID' TERM INT

    # Wait for the child process to exit
    wait $PID

    # Return the exit code of the child process
    exit $?
}

# Forward signals to the child process
forward_signals