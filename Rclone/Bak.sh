#!/bin/bash

# 指定配置文件位置，外部传参优先级最高，默认脚本同目录bak.conf
bakFileConfig="$1"

set -eo pipefail
# 开启严格模式：变量未定义、管道失败直接退出，提升健壮性
shopt -s extglob

bashversion=${BASH_VERSION%%.*}
if (( bashversion < 4 )); then
    echo "Bash版本过低，不支持关联数组，请用旧脚本。"
    exit 1
fi

# 打印当前时间
echo "===== 备份任务开始 $(date +"%F %T") ====="

# 自动获取脚本所在目录
script_path="$(readlink -f "$0")"
workdir="$(dirname "${script_path}")"
# 统一路径末尾带/
[[ "${workdir}" != */ ]] && workdir="${workdir}/"

# 未传入配置文件则使用默认路径
[[ -z "${bakFileConfig}" ]] && bakFileConfig="${workdir}bak.conf"

# 必填配置 key=说明
declare -A requirekeys=(
    ["rcloneLabel"]="保存到哪个远程盘，rclone配置中的标签名称"
    ["machine"]="本机器名称，备份时用于区别的目录名"
)
# 可选配置 key=说明+默认值
declare -A validkeys=(
    ["rcloneFile"]="rclone程序所在位置|/usr/bin/rclone"
    ["rcloneConfig"]="rclone配置文件路径|/root/.config/rclone/rclone.conf"
    ["remotePath"]="备份到盘里哪个目录|/"
    ["tmpPath"]="临时打包目录，有些系统家目录空间小|${HOME}"
    ["deleteDay"]="远端旧备份保留天数|90"
)

# 配置文件不存在则生成范例
if [[ ! -f "${bakFileConfig}" ]]; then
    echo "错误：配置文件 ${bakFileConfig} 不存在"
    read -p "是否生成范例配置文件？(Y/N): " user_input
    case "${user_input}" in
        [yY])
            echo "正在生成范例配置文件..."
            printf -v tmpConfig '# 自动生成备份配置范例，修改后再执行脚本
[config]
## ========== 必填配置 ==========\n'
            # 循环输出必填项
            for k in "${!requirekeys[@]}"; do
                printf -v tmpConfig '%s# %s\n%s=""\n' "${tmpConfig}" "${requirekeys[$k]}" "${k}"
            done
            printf -v tmpConfig '%s\n## ========== 可选配置（留空使用默认值） ==========\n' "${tmpConfig}"
            # 循环输出可选项
            for k in "${!validkeys[@]}"; do
                IFS='|' read -r desc defval <<< "${validkeys[$k]}"
                printf -v tmpConfig '%s# %s（默认：%s）\n%s=""\n' "${tmpConfig}" "${desc}" "${defval}" "${k}"
            done
            printf -v tmpConfig '%s\n[file]
#待备份文件清单列表，一行一条；路径含空格用双引号包裹，#开头注释
#支持tar排除参数，示例："/var/www/html/" --exclude="/var/www/html/*.log"
#每行会拼接为 tar -rPvf 包文件 内容
\n\n' "${tmpConfig}"
            echo -n "$tmpConfig" > "${bakFileConfig}"
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

# 待备份文件数组
fileListArr=()
fileListDropArr=()
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
            if [[ " ${!requirekeys[@]} " =~ " ${configkey} " || " ${!validkeys[@]} " =~ " ${configkey} " ]]; then
                eval "${configkey}='${configvalue}'"
            fi
        fi
    fi

    # [file] 备份路径清单
    if [[ ${isfilelist} -eq 1 && -n "${line}" ]]; then
        tmpArr=()
        while IFS= read -r item; do
            tmpArr+=("$item")
        done < <(xargs -n1 <<< "$line")
        tmpF=""
        tmpLis=0
        arr_len="${#tmpArr[@]}"

        if [[ ${arr_len} -eq 1 ]]; then
            tmpF="${tmpArr[0]}"
        elif [[ ${arr_len} -eq 2 ]]; then
            # 判断第一个参数是排除标识
            if [[ "${tmpArr[0]}" == "--exclude="* ]]; then
                tmpF="${tmpArr[1]}"
            # 判断第二个参数是排除标识
            elif [[ "${tmpArr[1]}" == "--exclude="* ]]; then
                tmpF="${tmpArr[0]}"
            else
                echo "错误：请一行仅填写一个文件或目录，不可同时填写两个路径"
                tmpLis=1
            fi
        elif [[ ${arr_len} -gt 2 ]]; then
            tmpLis=1
        fi
        # 路径异常
        [[ -z "${tmpF}" ]] && tmpLis=1
        if [[ -n "${tmpF}" ]]; then
            if [[ "$tmpF" == *"*"* ]]; then
                # 包含通配符，用glob匹配判断有无文件
                #shopt -s nullglob
                matches=($tmpF)
                #shopt -u nullglob
                # 匹配数量为0 → 无文件，标记异常
                [[ ${#matches[@]} -eq 0 ]] && tmpLis=1
            else
                # 普通路径，直接判断是否存在
                [[ ! -e "${tmpF}" ]] && tmpLis=1
            fi
        fi

        if [[ ${tmpLis} -eq 0 ]]; then
            fileListArr+=("${line}")
        else
            fileListDropArr+=("${line}")
        fi
    fi
done < "${bakFileConfig}"

# 校验必填项全部配置
for key in "${!requirekeys[@]}"; do
    val="${!key}"
    if [[ -z "${val}" ]]; then
        echo "错误：[config] 必填项 ${key} 未配置，请修改 ${bakFileConfig}"
        exit 1
    fi
done

# ---------------------- 默认值填充 & 合法性校验 ----------------------
# rclone程序路径
IFS='|' read -r _ def_rcloneFile <<< "${validkeys[rcloneFile]}"
[[ -z "${rcloneFile}" ]] && rcloneFile="${def_rcloneFile}"
if [[ ! -x "${rcloneFile}" ]]; then
    echo "错误：rclone程序不存在或无执行权限 ${rcloneFile}"
    exit 1
fi
# 检测rclone可用
if ! "${rcloneFile}" version &> /dev/null; then
    echo "错误：rclone程序运行异常"
    exit 1
fi

# rclone配置文件
IFS='|' read -r _ def_rcloneConfig <<< "${validkeys[rcloneConfig]}"
[[ -z "${rcloneConfig}" ]] && rcloneConfig="${def_rcloneConfig}"
if [[ ! -f "${rcloneConfig}" ]]; then
    echo "错误：rclone配置文件不存在 ${rcloneConfig}"
    exit 1
fi
# 校验远端标签存在
if ! grep -q "\[${rcloneLabel}\]" "${rcloneConfig}"; then
    echo "错误：rclone配置中无 [${rcloneLabel}] 远程标签"
    exit 1
fi

# 远端目录标准化
IFS='|' read -r _ def_remotePath <<< "${validkeys[remotePath]}"
[[ -z "${remotePath}" ]] && remotePath="${def_remotePath}"
[[ "${remotePath}" != */ ]] && remotePath="${remotePath}/"

# 临时打包目录
IFS='|' read -r _ def_tmpPath <<< "${validkeys[tmpPath]}"
if [[ -z "${tmpPath}" ]]; then
    tmpPath="${def_tmpPath}"
    [[ ! -d "${tmpPath}" ]] && tmpPath="/tmp"
fi
# 去除末尾/
tmpPath="${tmpPath%/}"
if [[ ! -d "${tmpPath}" ]]; then
    echo "错误：临时目录不存在 ${tmpPath}"
    exit 1
fi

# 清理天数校验，必须数字
IFS='|' read -r _ def_deleteDay <<< "${validkeys[deleteDay]}"
let def_deleteDay=${def_deleteDay}*1
if [[ -z "${deleteDay}" || ! "${deleteDay}" =~ ^[0-9]+$ ]]; then
    deleteDay="${def_deleteDay}"
fi

# 校验是否有需要备份的文件
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
tar -rPvf "${tmpbakfile}" "$script_path" "${bakFileConfig}" "${rcloneConfig}"
# 导出crontab临时文件并打包
crontmp="${tmpPath}/_crontab_tmp.bak"
crontab -l > "${crontmp}"
tar -rPvf "${tmpbakfile}" "${crontmp}"
rm -f "${crontmp}"

# 循环打包用户自定义备份文件/目录
echo "开始打包自定义备份清单，共${#fileListArr[@]}项"
count1=0
for item in "${fileListArr[@]}"; do
    # 脚本开始有 set -e ，出现非0值脚本退出
    let count1++ || true
    echo "打包项${count1}：${item}"
    tar -rPvf "${tmpbakfile}" ${item}
done
echo ""

if [[ ${#fileListDropArr[@]} -gt 0 ]]; then
    echo "-----"
    for item in "${fileListDropArr[@]}"; do
        echo "有问题行，未打包：${item}"
    done
    echo -e "-----\n"
fi

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
    remote_dir="${rcloneLabel}:${remotePath}${machine}"
    echo "网络正常，准备上传至远程 ${remote_dir}/"
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
    if [[ ${deleteDay} -gt 0 ]]; then
        echo "清理远端${deleteDay}天前的备份文件..."
        "${rcloneFile}" --config "${rcloneConfig}" --min-age "${deleteDay}d" delete "${remote_dir}/" --include "*_${machine}.tar.gz"
    else
        echo "删除天数小于1天，跳过删除"
    fi
else
    echo "❌ 多次网络检测失败，跳过云端上传，本地临时包将被清理"
fi

echo "===== 备份任务结束 $(date +"%F %T") ====="
echo ""
exit 0
