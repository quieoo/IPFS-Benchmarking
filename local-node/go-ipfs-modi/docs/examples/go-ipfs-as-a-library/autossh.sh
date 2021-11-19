#!/usr/bin/expect -f
#!/bin/sh
set password 2
spawn ssh root@127.0.0.1
set timeout 10;
expect {
        yes/no {send yes\n;exp_continue};
	        password: {send $password\n};
	}
set timeout 10;
send exit\n;
expect eof;
