#!/usr/bin/env bash
set -e
# 800 1000


for byz in 1 2 3
do
    for rate in 500 700 1000 1200 1500 1700 2000 2500 3000
    do
        fab updatec bsize 800
        fab updatec rate $rate
        fab updatec byzNo $byz
        fab updatec virtual_clients 1
        fab remote 22 mhotstuff info
        sleep 10
    done
done
