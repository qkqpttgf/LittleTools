#!/bin/bash

bakFileConfig="/root/bakFile.conf"

#必须配置项
requirekeys=("rclonefile" "rcloneconfig" "rclonelable" "bak2path" "machine")
#可选的，有效配置项
validkeys=("tmppath")
# 读取分析配置文件
fileListArr=()
isconfig=0
isfilelist=0
while read -r line || [[ -n "${line}" ]]; do
    #去除尾部\r
    line="${line//[$'\t\r\n']/}"
    line1="${line// /}"
    #空
    [ g"${line1}" = g"" ] && continue
    #以#开头
    [ g"${line1:0:1}" = g"#" ] && continue
    #以[开头，要么是config，要么是file
    if [ "g${line1:0:1}" = "g[" ]; then
        line=${line#*\[}
        line=${line%\]*}
        #echo ${line}
        if [ "g${line}" = "gconfig" ]; then
            isconfig=1
            isfilelist=0
            continue
        fi
        if [ "g${line}" = "gfile" ]; then
            isconfig=0
            isfilelist=1
            continue
        fi
        isconfig=0
        isfilelist=0
        continue
    fi
    #line="$(echo ${line})"
    #配置区
    if [ ${isconfig} -eq 1 ]; then
        configkey="${line%%=*}"
        #谨防有人=前后加空格
        configkey="$(echo ${configkey})"
        configvalue="${line#*=}"
        configvalue="$(echo ${configvalue})"
        for key in "${requirekeys[@]}"; do
            #读入必须配置
            if [ g"${configkey}" = g"${key}" ]; then
                eval ${configkey}=${configvalue}
                break
            fi
        done
        for key in "${validkeys[@]}"; do
            #只有有效配置才读入
            if [ g"${configkey}" = g"${key}" ]; then
                eval ${configkey}=${configvalue}
                break
            fi
        done
    fi
    #文件区
    if [ ${isfilelist} -eq 1 ]; then
        fileListArr[${#fileListArr[*]}+1]="${line}"
    fi
done <"${bakFileConfig}"

#验证所需配置都有了
#echo "_${rclonefile}_"
#echo "_${rcloneconfig}_"
for key in "${requirekeys[@]}"; do
    tmp="echo \$${key}"
    value1=`eval $tmp`
    if [ ${#value1} -lt 1 ]; then
        echo "${key}无配置，退出"
        exit
    fi
done
#echo "_${fileListArr[@]}_"

#--------------------------------
#本次备份文件名称
bakfilename="$(date +\%Y\%m\%d\-%H\%M\%S)_${machine}.tar"
if [ g"${tmppath}" = g"" ]; then
    #非root用户也可用
    tmppath=$HOME
    if [ ! -d "${tmppath}" ]; then
        tmppath="/tmp"
    fi
fi
if [ g"${tmppath:0-1}" = g"/" ]; then
    tmppath=${tmppath:0:${#tmppath}-1}
fi
tmpbakfile="${tmppath}/${bakfilename}"
echo "${tmpbakfile}"
echo "----"
#先备份配置文件
tar -rPvf "${tmpbakfile}" "${bakFileConfig}"
#再备份rclone配置
tar -rPvf "${tmpbakfile}" "${rcloneconfig}"
#备份计划任务
crontab -l >${tmppath}/crontab.bak
tar -rPvf "${tmpbakfile}" ${tmppath}/crontab.bak
rm -f ${tmppath}/crontab.bak
#读列表，备份指定文件与文件夹
for file in "${fileListArr[@]}"; do
    #echo "_${file}_"
    tar -rPvf "${tmpbakfile}" ${file}
done
#压缩一下
gzip "${tmpbakfile}"
echo "打包结束"

#判断有没有网络
p=0
while [ $p -lt 3 ]; do
  network1=$(curl -s live.cn -w "%{http_code}")
  network1=${network1:0-3}
  #network1="${network1//[$'\t\r\n ']}"
  [ ${network1} -gt 0 ] && break
  let p++
  sleep 2
done
#echo ${network1}
if [ ${network1} -gt 0 ]; then
  [ ${bak2path:0-1} != "/" ] && bak2path="${bak2path}/"
  #创建目录
  ${rclonefile} --config ${rcloneconfig} mkdir ${rclonelable}:"${bak2path}${machine}"
  #上传文件
  p=0
  while [ $p -lt 3 ]; do
    ${rclonefile} --config ${rcloneconfig} moveto "${tmpbakfile}.gz" ${rclonelable}:"${bak2path}${machine}/${bakfilename}.gz"
    sleep 2
    ${rclonefile} --config ${rcloneconfig} ls ${rclonelable}:"${bak2path}${machine}/${bakfilename}.gz" && break
    let p++
  done
  if [ $p -lt 3 ]; then
    echo "应该上传成功"
  else
    echo "可能上传失败"
  fi
  #删除90天前
  ${rclonefile} --config ${rcloneconfig} --min-age 90d delete ${rclonelable}:"${bak2path}${machine}/"
else
  echo "没有网络"
fi
echo "--------"
echo ""
