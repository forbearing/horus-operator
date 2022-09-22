#!/usr/bin/env bash

EXIT_SUCCESS() { exit 0; }
EXIT_FAILURE() { exit 1; }
RETURN_SUCCESS=0
RETURN_FAILURE=1

if !command -v restic &> /dev/null; then
    echo "command \"restic\" not found"
    exit 1
fi

Restic_Init() {
    echo "Starting restic init"
    # if restic repository already exist, skip init repository.
    local count=1
    while true; do
        restic list keys &> /dev/null
        local list_lock_rc=$?
        if [[ ${list_lock_rc} -eq 0 ]]; then
            echo "restic repository already exists, skip..."
            EXIT_SUCCESS; fi
        if [[ ${count} -ge 6 ]]; then break; fi
        restic unlock &> /dev/null
        echo "restic repository maybe not exist, check again ${count}."
        sleep 3
        (( count++ ))
    done

    # if restic repository not exist, start restic init to initial a restic repository.
    local count=1
    while true; do
        restic init
        local init_rc=$?
        if [[ ${init_rc} -eq 0 ]]; then
            echo "Successfully restic init."
            EXIT_SUCCESS; fi
        if [[ ${count} -ge 6 ]]; then 
            echo "restic init failed with status: ${init_rc}."
            restic unlock &> /dev/null
            EXIT_FAILURE; fi
        restic unlock &> /dev/null
        echo "restic init failed, retry ${count}."
        sleep 5
        (( count++ ))
    done
}

Restic_Init
