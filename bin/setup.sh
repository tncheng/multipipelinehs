#!/usr/bin/env bash

user=$1
ip=$2
key=$3

# add ssh key
expect -c "
scp -i $key ~/.ssh/id_rsa.pub $user@$ip:~/.ssh;

"
    
expect <<EOF
    set timeout 60
    spawn scp -i $key ~/.ssh/id_rsa.pub $user@$ip:~/.ssh
    expect {
        "yes/no" {send "yes\r";exp_continue }
        "password:" {send "$3\r";exp_continue }
        eof
    }
EOF