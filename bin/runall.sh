#!/usr/bin/env bash

set -e

fab remote 34 mhotstuff deploy 
for rate in 5000
do
    for vc in 1
    do
        fab updatec bsize 800
        fab updatec rate $rate
        fab updatec virtual_clients $vc
        fab remote 10 mhotstuff update
    done
done

