#!/bin/bash

readonly main_module='github.com/dobyte/due/v[0-9]\+'
readonly due_prefix='github.com/dobyte/due'
readonly due_tags_url='https://api.github.com/repos/dobyte/due/git/refs/tags'
readonly len=${#prefix}


# 更新框架主模块
while read line
do
  if echo "$line" | grep -q "$due_prefix/v[0-9]\+"; then
    arr=($line)
    go get "${arr[0]}@latest"
    break
  fi
done < go.mod

# 获取主模块对应的版本号
while read line
do
  if echo "$line" | grep -q "$due_prefix/v[0-9]\+"; then
    arr=($line)

    response=$(curl -s "${due_tags_url}/${arr[1]}")

    echo "$response"
    break
  fi
done < go.mod


# main_version=$(go list -m -f '{{.Version}}' $prefix)

# # 更新所有依赖
# go get -u all

# # 清理未使用的依赖
# go mod tidy




# while read line
# do
#   if [[ ${line:0:len} = ${prefix} ]];then
#     arr=($line)
#     go get "${arr[0]}@latest"
#   fi
# done < go.mod

# go mod tidy