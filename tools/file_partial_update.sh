#!/bin/bash

# 检查输入参数
if [ $# -ne 2 ]; then
  echo "Usage: $0 <folder_name_part> <replacement_string>"
  exit 1
fi

# 获取命令行参数
folder_name_part=$1
replacement_string=$2

# 检查字符串长度是否为3个字符
if [ ${#replacement_string} -ne 3 ]; then
  echo "Error: Replacement string must be exactly 3 characters."
  exit 1
fi

# 搜索当前目录下包含指定字符串的文件夹
folders=$(find . -type d -name "*${folder_name_part}*")

# 如果没有找到文件夹，退出
if [ -z "$folders" ]; then
  echo "Error: No folders found matching '$folder_name_part'"
  exit 1
fi

# 遍历找到的文件夹
for folder in $folders; do
  echo "Processing folder: $folder"

  # 遍历文件夹内的所有文件
  for file in "$folder"/*; do
    if [ -f "$file" ]; then
      echo "Processing file: $file"
      
      # 读取文件的前3个字节并用指定的字符串替换
      # 使用 head 和 tail 命令进行替换操作
      new_content="${replacement_string}$(tail -c +4 "$file")"
      
      # 写回文件
      echo -n "$new_content" > "$file"
    fi
  done
done

echo "All files processed."
