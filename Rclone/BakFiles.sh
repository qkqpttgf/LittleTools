#!/bin/bash

#待备份文件清单列表，内容为一行一个文件(夹)，文件名内如有空格请自觉前后加上"，可以用*指代一些文件，可在最前加#注释
#程序将会一行一行拼凑成类似 tar -rPf ${bakfile} /var/www/html/ --exclude=/var/www/html/data/*db 来打包
bakFileList="/root/.bakFileList"
# rclone程序及配置位置
rclonefile="/usr/bin/rclone"
rcloneconfig="/root/.config/rclone/rclone.conf"
# 保存到哪个远程盘，配置中标签名称
rclonelable="one1"
# 备份到盘里哪个目录
bak2path="/bak/"
#备份时用于区别的名称
machine="N1"

#--------------------------------
#本次备份文件名称
bakfile="$(date +\%Y\%m\%d\-%H\%M\%S)_${machine}.tar"
tmpbakpath="/root/${bakfile}"
#先备份文件列表
tar -rPf "${tmpbakpath}" "${bakFileList}"
#再备份rclone配置
tar -rPf "${tmpbakpath}" "${rcloneconfig}"
#读列表，备份指定文件与文件夹
while read line; do
  #以#开头的行不备份
  line1=${line// /}
  [ g"${line1}" != g"" ] && [ "g${line1:0:1}" != "g#" ] && tar -rPvf "${tmpbakpath}" ${line}
done <"${bakFileList}"
#定时任务表
crontab -l >/root/crontab.bak
tar -rPvf "${tmpbakpath}" /root/crontab.bak
rm -f /root/crontab.bak
#压缩一下
gzip "${tmpbakpath}"

#判断有没有网络
network1=$(curl -s live.cn -w "%{http_code}")
network1=${network1:0-3}
#network1="${network1//[$'\t\r\n ']}"
#echo ${network1}
if [ ${network1} -gt 0 ]; then
  [ ${bak2path:0-1} != "/" ] && bak2path="${bak2path}/"
  #创建目录
  ${rclonefile} --config ${rcloneconfig} mkdir ${rclonelable}:"${bak2path}${machine}"
  #上传文件
  fileExist=""
  while [ g"${fileExist}" = g"" ]; do
    ${rclonefile} --config ${rcloneconfig} moveto "${tmpbakpath}.gz" ${rclonelable}:"${bak2path}${machine}/${bakfile}.gz"
    sleep 1
    fileExist=$(${rclonefile} --config ${rcloneconfig} ls ${rclonelable}:"${bak2path}${machine}/${bakfile}.gz")
  done
  echo ${fileExist}
  #删除90天前
  ${rclonefile} --config ${rcloneconfig} --min-age 90d delete ${rclonelable}:"${bak2path}${machine}/"
else
  echo "没有网络"
fi
