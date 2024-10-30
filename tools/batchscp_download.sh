#!/bin/bash
# 清除 known_hosts，以避免 ssh 的主机认证问题
rm -rf ~/.ssh/known_hosts

# 读取主机列表文件
host_list=$1

# shellcheck disable=SC2162
# shellcheck disable=SC2002
cat $host_list | while read line
do
 host_ip=`echo $line|awk '{print $1}'`
 username=`echo $line|awk '{print $2}'`
 password=`echo $line|awk '{print $3}'`
 src_file=`echo $line|awk '{print $4}'`
 dest_file=`echo $line|awk '{print $5}'`

 # 调用单个文件下载的脚本
 ./singlescp_download.sh $username $host_ip $src_file $dest_file $password
done
