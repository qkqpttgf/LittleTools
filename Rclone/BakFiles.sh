#!/bin/bash
date +"%F %T"

#指定配置文件位置，默认为 脚本同目录的 bak.conf
bakFileConfig=""

if [ g"${bakFileConfig}" = g"" ]; then
    workdir="$(dirname "$(readlink -f "$0")")"
    if [ ${workdir:0-1} != "/" ]; then
        workdir="${workdir}/"
    fi
    bakFileConfig="${workdir}bak.conf"
fi
if [ ! -f "${bakFileConfig}" ]; then
    echo "文件 ${bakFileConfig} 不存在"

    read -p "是否用范例创建样本？ (Y/N): " user_input
    case "$user_input" in
        [yY])
            echo "创建中……"
            cat << EOF > "${bakFileConfig}"
[config]
##必须配置项
# 保存到哪个远程盘，rclone配置中的标签名称
rcloneLable=""
# 本机器名称，备份时用于区别的名称
machine=""

##可选的有效配置项
# rclone程序所在位置 默认 /usr/bin/rclone
rcloneFile = ""
# rclone的配置的路径 默认 /root/.config/rclone/rclone.conf
rcloneConfig=""
# 备份到盘里哪个目录 默认 /
remotePath=""
# 临时目录，有些系统家目录空间小 默认 家home目录
tmpPath=""
# 盘里多少天前的旧文件将被删除，数字 默认 90
deleteDay=90

[file]
#待备份文件清单列表，内容为一行一个文件(夹)，文件名内如有空格请前后加上"，可以用*指代一些文件，可在最前加#注释
#例 "/var/www/html/" --exclude="/var/www/html/data base/"*.db
#程序将一行一行拼凑成 tar -rPvf \${tmpfile} /var/www/html/ --exclude="/var/www/html/data base/"*.db 这样一条命令来打包

EOF
            if [ ! -f "${bakFileConfig}" ]; then
                echo "创建失败"
                exit 1
            fi
            echo "创建成功，请去编辑 ${bakFileConfig} ，添加要备份的文件列表"
            exit 0
        ;;
        *)
            exit 1
        ;;
    esac
fi

requirekeys=("rcloneLable" "machine")
validkeys=("rcloneFile" "rcloneConfig" "remotePath" "tmpPath" "deleteDay")

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
#echo "_${rcloneFile}_"
#echo "_${rcloneConfig}_"
for key in "${requirekeys[@]}"; do
    tmp="echo \$${key}"
    value1=`eval $tmp`
    if [ ${#value1} -lt 1 ]; then
        echo "${key}无配置，退出"
        exit 1
    fi
done
#rclone程序
if [ g"${rcloneFile}" = g"" ]; then
    rcloneFile="/usr/bin/rclone"
fi
if [ ! -f "${rcloneFile}" ]; then
    echo "rclone程序文件 ${rcloneFile} 不存在"
    exit 1
fi
ok=$("${rcloneFile}" version > /dev/null && echo ok)
if [ g"${ok}" != g"ok" ]; then
    echo "rclone程序运行不正常"
    exit 1
fi
#rclone的配置文件
if [ g"${rcloneConfig}" = g"" ]; then
    rcloneFile="/root/.config/rclone/rclone.conf"
fi
if [ ! -f "${rcloneConfig}" ]; then
    echo "rclone配置文件 ${rcloneConfig} 不存在"
    exit 1
fi
ok=$(cat "${rcloneConfig}" | grep "\[${rcloneLable}\]" > /dev/null && echo ok)
if [ g"${ok}" != g"ok" ]; then
    echo "rclone配置文件 ${rcloneConfig} 中没有 ${rcloneLable} 。"
    exit 1
fi
#远端目录
if [ g"${remotePath}" = g"" ]; then
    remotePath="/"
fi
if [ ${remotePath:0-1} != "/" ]; then
    remotePath="${remotePath}/"
fi
#临时目录
if [ g"${tmpPath}" = g"" ]; then
    #非root用户也可用
    tmpPath=$HOME
    if [ ! -d "${tmpPath}" ]; then
        tmpPath="/tmp"
    fi
fi
if [ g"${tmpPath:0-1}" = g"/" ]; then
    tmpPath=${tmpPath:0:${#tmpPath}-1}
fi
if [ ! -d "${tmpPath}" ]; then
    echo "临时目录 ${tmpPath} 不存在"
    exit 1
fi
#旧文件删日期
if [ g"${deleteDay}" != g"" ]; then
    let deleteDay=${deleteDay}*1
fi
if [ g"${deleteDay}" = g"" ]; then
    deleteDay=90
fi
if [ ${deleteDay} -eq 0 ]; then
    deleteDay=90
fi
#echo "_${fileListArr[@]}_"
if [ ${#fileListArr[@]} -lt 1 ]; then
    echo "没有文件要备份"
    exit 0
if

#--------------------------------
#本次备份文件名称
bakfilename="$(date +"%Y%m%d-%H%M%S")_${machine}.tar"
tmpbakfile="${tmpPath}/${bakfilename}"
echo "${tmpbakfile}"
echo "----"
#备份自身
tar -rPvf "${tmpbakfile}" "$(readlink -fn "$0")"
#备份配置文件
tar -rPvf "${tmpbakfile}" "${bakFileConfig}"
#备份rclone配置
tar -rPvf "${tmpbakfile}" "${rcloneConfig}"
#备份计划任务
crontab -l >${tmpPath}/crontab.bak
tar -rPvf "${tmpbakfile}" ${tmpPath}/crontab.bak
rm -f ${tmpPath}/crontab.bak
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
  #创建目录
  "${rcloneFile}" --config ${rcloneConfig} mkdir ${rcloneLable}:"${remotePath}${machine}"
  #上传文件
  p=0
  while [ $p -lt 3 ]; do
    "${rcloneFile}" --config ${rcloneConfig} moveto "${tmpbakfile}.gz" ${rcloneLable}:"${remotePath}${machine}/${bakfilename}.gz"
    sleep 2
    "${rcloneFile}" --config ${rcloneConfig} ls ${rcloneLable}:"${remotePath}${machine}/${bakfilename}.gz" && break
    let p++
  done
  if [ $p -lt 3 ]; then
    echo "应该上传成功"
  else
    echo "可能上传失败"
  fi
  #删除90天前
  "${rcloneFile}" --config ${rcloneConfig} --min-age ${deleteDay}d delete ${rcloneLable}:"${remotePath}${machine}/"
else
  echo "没有网络"
fi
echo "--------"
date +"%F %T"
echo ""
