#!/usr/bin/env bash

set -e
for byz in 0 1 2 4 6 9
do
    python3 updatejs.py byzNo $byz
    fab remote 32 mhotstuff
    sleep 30
done

