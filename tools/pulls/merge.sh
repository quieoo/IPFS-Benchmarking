#!/bin/bash

# 检查输入参数是否足够
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <file_prefix> <output_file>"
    exit 1
fi

file_prefix=$1
output_file=$2

# 清空输出文件
> "$output_file"

# 查找符合前缀的文件并合并
for file in "$file_prefix"*; do
    if [ -f "$file" ]; then
        cat "$file" >> "$output_file" # 合并文件内容
        echo "" >> "$output_file"     # 添加换行符
    fi
done

echo "All files with prefix '$file_prefix' have been merged into '$output_file'."
