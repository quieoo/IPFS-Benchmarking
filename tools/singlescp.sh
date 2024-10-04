#!/usr/bin/expect
# shellcheck disable=SC1054
# shellcheck disable=SC1073

# 检查输入参数是否足够
if {$argc < 5} {
  send_user "usage: $argv0 src_file username ip dest_file password\n"
  exit
}

set timeout -1
set src_file [lindex $argv 0]
set username [lindex $argv 1]
set host_ip [lindex $argv 2]
set dest_file [lindex $argv 3]
set password [lindex $argv 4]

# 启动 scp 命令
spawn scp $src_file $username@$host_ip:/$dest_file

# 处理 expect 的不同输出情况
expect {
  # 第一次连接时的主机验证提示，使用部分匹配 "Are you sure"
  "Are you sure you want to continue connecting" {
    send "yes\n"
    expect "password:" {send "$password\n"}
  }
  # 如果已经信任，直接提示密码
  "password:" {
    send "$password\n"
  }
  # 出现错误或超时的情况
  timeout {
    send_user "Connection timed out.\n"
    exit
  }
  eof {
    send_user "Error: Unable to connect to host.\n"
    exit
  }
}

# 等待文件传输完成
expect "100%"
expect eof
