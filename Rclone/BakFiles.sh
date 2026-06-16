#!/bin/bash
set -eo pipefail
# 开启严格模式：变量未定义、管道失败直接退出，提升健壮性
shopt -s extglob

# 打印当前时间
echo "===== 备份任务开始 $(date +"%F %T") ====="

# 指定配置文件位置，外部传参优先级最高，默认脚本同目录bak.conf
bakFileConfig="$1"

# 自动获取脚本所在目录
script_path="$(readlink -f "$0")"
workdir="$(dirname "${script_path}")"
# 统一路径末尾带/
[[ "${workdir}" != */ ]] && workdir="${workdir}/"

# 未传入配置文件则使用默认路径
[[ -z "${bakFileConfig}" ]] && bakFileConfig="${workdir}bak.conf"

# 配置文件不存在则生成范例
if [[ ! -f "${bakFileConfig}" ]]; then
    echo "错误：配置文件 ${bakFileConfig} 不存在"
    read -p "是否生成范例配置文件？(Y/N): " user_input
    case "${user_input}" in
        [yY])
            echo "正在生成范例配置文件..."
            cat << EOF > "${bakFileConfig}"
[config]
##必须配置项
# 保存到哪个远程盘，rclone配置中的标签名称
rcloneLable=""
# 本机器名称，备份时用于区别的名称
machine=""

##可选的有效配置项（等号前后不要加空格）
# rclone程序所在位置 默认 /usr/bin/rclone
rcloneFile=""
# rclone的配置的路径 默认 /root/.config/rclone/rclone.conf
rcloneConfig=""
# 备份到盘里哪个目录 默认 /
remotePath=""
# 临时目录，有些系统家目录空间小 默认 家home目录
tmpPath=""
# 盘里多少天前的旧文件将被删除，数字 默认 90
deleteDay=90

[file]
#待备份文件清单列表，一行一个，路径含空格用双引号包裹，#开头注释
#支持tar排除参数，示例："/var/www/html/" --exclude="/var/www/html/db/*.db"
#每行会拼接为 tar -rPvf 包文件 内容
EOF
            if [[ ! -f "${bakFileConfig}" ]]; then
                echo "创建配置文件失败，请检查目录写入权限"
                exit 1
            fi
            echo "范例配置创建完成，请编辑：${bakFileConfig} 填写备份路径"
            exit 0
        ;;
        *)
            echo "未生成配置文件，退出"
            exit 1
        ;;
    esac
fi

# 必填配置、可选配置列表
requirekeys=("rcloneLable" "machine")
validkeys=("rcloneFile" "rcloneConfig" "remotePath" "tmpPath" "deleteDay")
# 待备份文件数组
fileListArr=()
# 分区标记
isconfig=0
isfilelist=0

# 逐行解析ini配置
while read -r line || [[ -n "${line}" ]]; do
    # 清除换行、回车、制表符
    line="${line//[$'\t\r\n']/}"
    line_stripped="${line//[[:space:]]/}"
    # 空行/注释跳过
    [[ -z "${line_stripped}" || "${line_stripped:0:1}" == "#" ]] && continue
    # 匹配分区 [xxx]
    if [[ "${line_stripped:0:1}" == "[" ]]; then
        section="${line_stripped#[}"
        section="${section%]}"
        case "${section}" in
            config)
                isconfig=1
                isfilelist=0
                ;;
            file)
                isconfig=0
                isfilelist=1
                ;;
            *)
                isconfig=0
                isfilelist=0
                ;;
        esac
        continue
    fi

    # [config] 分区键值解析
    if [[ ${isconfig} -eq 1 ]]; then
        # 分离key value，清除首尾空格
        configkey="${line%%=*}"
        configkey="$(echo -n "${configkey}" | xargs)"
        configvalue="${line#*=}"
        configvalue="$(echo -n "${configvalue}" | xargs)"
        # 动态赋值，安全包裹引号
        if [[ -n "${configkey}" ]]; then
            # 仅允许预设key赋值，防止注入
            if [[ " ${requirekeys[@]} " =~ " ${configkey} " || " ${validkeys[@]} " =~ " ${configkey} " ]]; then
                eval "${configkey}='${configvalue}'"
            fi
        fi
    fi

    # [file] 备份路径清单
    if [[ ${isfilelist} -eq 1 && -n "${line}" ]]; then
        fileListArr+=("${line}")
    fi
done < "${bakFileConfig}"

# 校验必填项全部配置
for key in "${requirekeys[@]}"; do
    val="${!key}"
    if [[ -z "${val}" ]]; then
        echo "错误：[config] 必填项 ${key} 未配置，请修改 ${bakFileConfig}"
        exit 1
    fi
done

# ---------------------- 默认值填充 & 合法性校验 ----------------------
# rclone 程序路径
[[ -z "${rcloneFile}" ]] && rcloneFile="/usr/bin/rclone"
if [[ ! -x "${rcloneFile}" ]]; then
    echo "错误：rclone程序不存在或无执行权限 ${rcloneFile}"
    exit 1
fi
# 检测rclone可用
if ! "${rcloneFile}" version &> /dev/null; then
    echo "错误：rclone程序运行异常"
    exit 1
fi

# rclone 配置文件（修复原脚本赋值bug）
[[ -z "${rcloneConfig}" ]] && rcloneConfig="/root/.config/rclone/rclone.conf"
if [[ ! -f "${rcloneConfig}" ]]; then
    echo "错误：rclone配置文件不存在 ${rcloneConfig}"
    exit 1
fi
# 校验远端标签存在
if ! grep -q "\[${rcloneLable}\]" "${rcloneConfig}"; then
    echo "错误：rclone配置中无 [${rcloneLable}] 远程标签"
    exit 1
fi

# 远端目录标准化
[[ -z "${remotePath}" ]] && remotePath="/"
[[ "${remotePath}" != */ ]] && remotePath="${remotePath}/"

# 临时目录处理
if [[ -z "${tmpPath}" ]]; then
    tmpPath="${HOME}"
    [[ ! -d "${tmpPath}" ]] && tmpPath="/tmp"
fi
# 去除末尾/
tmpPath="${tmpPath%/}"
if [[ ! -d "${tmpPath}" ]]; then
    echo "错误：临时目录不存在 ${tmpPath}"
    exit 1
fi

# 清理天数校验，必须数字，默认90
if [[ -z "${deleteDay}" || ! "${deleteDay}" =~ ^[0-9]+$ || ${deleteDay} -eq 0 ]]; then
    deleteDay=90
fi

# 校验是否有需要备份的文件（修复原脚本缺少fi致命错误）
if [[ ${#fileListArr[@]} -lt 1 ]]; then
    echo "提示：[file] 分区未填写任何备份路径，无需备份，退出"
    exit 0
fi

# ---------------------- 打包备份 ----------------------
# 本次备份文件名
bakfilename="$(date +"%Y%m%d-%H%M%S")_${machine}.tar"
tmpbakfile="${tmpPath}/${bakfilename}"
tmpbakgz="${tmpbakfile}.gz"

# 异常退出自动清理临时包
clean_tmp() {
    [[ -f "${tmpbakgz}" ]] && rm -f "${tmpbakgz}"
    [[ -f "${tmpbakfile}" ]] && rm -f "${tmpbakfile}"
}
trap clean_tmp EXIT SIGINT SIGTERM

echo "临时备份包路径：${tmpbakgz}"
echo "开始打包内置配置文件..."

# 打包脚本自身、配置、rclone配置、定时任务
tar -rPvf "${tmpbakfile}" -- "$script_path" "${bakFileConfig}" "${rcloneConfig}"
# 导出crontab临时文件并打包
crontmp="${tmpPath}/_crontab_tmp.bak"
crontab -l > "${crontmp}"
tar -rPvf "${tmpbakfile}" -- "${crontmp}"
rm -f "${crontmp}"

# 循环打包用户自定义备份文件/目录
echo "开始打包自定义备份清单，共${#fileListArr[@]}项"
for item in "${fileListArr[@]}"; do
    echo "打包项：${item}"
    # -- 防止路径以-开头被识别为tar参数
    tar -rPvf "${tmpbakfile}" -- ${item}
done

# gzip压缩
echo "打包完成，开始压缩..."
gzip "${tmpbakfile}"
[[ ! -f "${tmpbakgz}" ]] && { echo "压缩失败"; exit 1; }

# ---------------------- 网络检测 ----------------------
check_network() {
    # 3秒超时，访问公共检测地址
    if curl --max-time 3 -s --head live.cn &> /dev/null; then
        return 0
    else
        return 1
    fi
}

network_ok=0
for ((i=0; i<3; i++)); do
    if check_network; then
        network_ok=1
        break
    fi
    echo "网络检测失败，2秒后重试(${i+1}/3)"
    sleep 2
done

if [[ ${network_ok} -eq 1 ]]; then
    echo "网络正常，准备上传至远程 ${rcloneLable}:${remotePath}${machine}/"
    remote_dir="${rcloneLable}:${remotePath}${machine}"
    # 创建远端目录
    "${rcloneFile}" --config "${rcloneConfig}" mkdir "${remote_dir}"

    # 上传重试3次
    upload_success=0
    for ((i=0; i<3; i++)); do
        echo "上传尝试 ${i+1}/3"
        "${rcloneFile}" --config "${rcloneConfig}" moveto --retries 2 "${tmpbakgz}" "${remote_dir}/${bakfilename}.gz"
        # 校验文件是否存在远端
        if "${rcloneFile}" --config "${rcloneConfig}" ls "${remote_dir}/${bakfilename}.gz" | grep -q "${bakfilename}.gz"; then
            upload_success=1
            break
        fi
        sleep 3
    done

    if [[ ${upload_success} -eq 1 ]]; then
        echo "✅ 文件上传成功"
    else
        echo "❌ 上传全部重试失败，请检查网络/远端权限"
    fi

    # 清理远端超过deleteDay天的旧备份
    echo "清理远端${deleteDay}天前的备份文件..."
    "${rcloneFile}" --config "${rcloneConfig}" --min-age "${deleteDay}d" delete "${remote_dir}/"
else
    echo "❌ 多次网络检测失败，跳过云端上传，本地临时包将被清理"
fi

echo "===== 备份任务结束 $(date +"%F %T") ====="
echo ""
exit 0
