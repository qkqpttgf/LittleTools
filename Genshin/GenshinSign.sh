#!/bin/bash

#WorkWeiBot="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxx"
#DingDingBot="https://oapi.dingtalk.com/robot/send?access_token=xxxxxxxx"
#SCKEY="SCTxxxxxxxx"

#------------------------------
#cd "$(dirname "$0")"
configFolder="$HOME/.config/Genshin_Sign"
[ ! -d ${configFolder} ] && mkdir -p "${configFolder}"
cookieFile="${configFolder}/Genshin_Sign.conf"
tmpfile="/tmp/mihoyo_sign"
syh='"'
zkh='{'
ykh='}'

function getjsonvalue {
  # $1 json string
  # $2 key
  value=$(echo "$1" | awk -F "$2\":" '{print $2}')
  if [ g"${value:0:1}" = g"\"" ]; then
    value=${value#*${syh}}
    value=${value%%${syh}*}
  else
    value=${value%%,*}
  fi
  # | awk -F "," '{print $1}'`
  value=${value%${ykh}*}
  echo ${value}
}
function WorkWei() {
  #echo -e $1
  a="$1"
  a=${a//${syh}/\\${syh}}
  echo -n "企业微信机器人："
  curl -s "${WorkWeiBot}" \
    -H 'Content-Type: application/json' \
    -d '
   {
        "msgtype": "text",
        "text": {
            "content": "'"${a}"'"
        }
   }'
  echo ""
}
function DingDing() {
  ddstring="$1"
  ddstring="${ddstring//${syh}/}"
  echo -n "钉钉机器人："
  curl -s -4 "${DingDingBot}" \
    -H 'Content-Type: application/json' \
    -d '
    {
      "msgtype": "text",
      "text": {
        "content": "'"${ddstring}"'"
      }
    }'
  echo ""
}
function FTQQ() {
  echo -n "Server酱："
  url="https://sctapi.ftqq.com/${SCKEY}.send"
  #data="`date "+%F %T %A"`\n$2"
  data="$2"
  n='
'
  n1='\\n'
  data=${data//${n1}/${n}${n}}
  data='text='"$1"'&desp='"${data}"

  curl -s "${url}" \
    -d "${data}"
  echo ""
}

function signCheck {
  url="${checkSignUrl}?act_id="${act_id}"&region=${region}&uid=${game_uid}"
  checkResult=$(curl -s "${url}" -H "Cookie: ${cookie}" -w %{http_code})
  checkcode=${checkResult:0-3}
  if [ g"${checkcode}" = g"200" ]; then
    is_sign=$(getjsonvalue "${checkResult}" "is_sign")
    if [ g"${is_sign}" = g"true" ]; then
      signedDays=$(getjsonvalue "${checkResult}" "total_sign_day")
      if [ g"$1" != g"" ]; then
        echo ${signedDays}
      else
        echo "今日签过，本月已签${signedDays}天"
      fi
    else
      echo 0
    fi
  else
    echo " N${checkResult}"
  fi
}

function sign {
  data1='{"act_id":"'${act_id}'","region":"'${region}'","uid":"'${game_uid}'"}'
  #echo ${data1}
  #    salt1="9nQiU3AV0rJSIBWgdynfoGMGKaklfbM7"
  salt1="YVEIkzDFNHLeKXLxzqCA9TzxCpWwbIbk"
  time1=$(date +%s)
  random1="818465" #随机6个字母与数字
  md51=$(echo -n "salt=${salt1}&t=${time1}&r=${random1}" | md5sum)
  md51=${md51%% *}
  signResult=$(curl -s "${signUrl}" -d "${data1}" -H "User-Agent: Android; miHoYoBBS/2.36.1" -H "Cookie: ${cookie}" -H "Content-Type: application/json" -H "x-rpc-device_id: F84E53D45BFE4424ABEA9D6F0205FF4A" -H "x-rpc-app_version: 2.36.1" -H "x-rpc-client_type: 5" -H "DS: ${time1},${random1},${md51}" -w %{http_code})
  signcode=${signResult:0-3}
  if [ g"${signcode}" = g"200" ]; then
    signRetcode=$(getjsonvalue "${signResult}" "retcode")
    if [ g"${signRetcode}" = g"0" ]; then
      GT=$(getjsonvalue "${signResult}" "gt")
      if [ g"${GT}" != g"" ]; then
        echo " Error，有验证码"
        #echo " E${signResult}"
      else
        # 签到成功
        #getjsonvalue "${signResult}" "message"
        echo "OK"
      fi
    else
      if [ g"${signRetcode}" = g"-5003" ]; then
        # 已经签到过了
        getjsonvalue "${signResult}" "message"
      else
        echo " E${signResult}"
      fi
    fi
  else
    echo " N${signResult}"
  fi
}
function dailyNote {
  dailyNoteUrl="https://api-takumi-record.mihoyo.com/game_record/app/genshin/api/dailyNote?role_id=${game_uid}&server=${region}"
  #echo ${dailyNoteUrl}
  salt2="xV8v4Qu54lUKrEYFZkJhB8cuOh9Asafs"
  time1=$(date +%s)
  random1="180010"
  md51=$(echo -n "salt=${salt2}&t=${time1}&r=${random1}&b=&q=role_id=${game_uid}&server=${region}" | md5sum)
  md51=${md51%% *}
  curl -s "${dailyNoteUrl}" -H "User-Agent: miHoYoBBS/2.33.1" -H "Cookie: ${cookie}" -H "x-rpc-device_id: F84E53D45BFE4424ABEA9D6F0205FF4A" -H "x-rpc-app_version: 2.33.1" -H "x-rpc-client_type: 5" -H "DS: ${time1},${random1},${md51}" -w %{http_code}
}
function character {
  dailyNoteUrl="https://api-takumi-record.mihoyo.com/game_record/app/genshin/api/character"
  data1='{"role_id":"'${game_uid}'","server":"'${region}'"}'
  salt2="xV8v4Qu54lUKrEYFZkJhB8cuOh9Asafs"
  time1=$(date +%s)
  random1="180010"
  md51=$(echo -n "salt=${salt2}&t=${time1}&r=${random1}&b=${data1}&q=" | md5sum)
  md51=${md51%% *}
  curl -s "${dailyNoteUrl}" -d "${data1}" -H "User-Agent: miHoYoBBS/2.33.1" -H "Cookie: ${cookie}" -H "x-rpc-device_id: F84E53D45BFE4424ABEA9D6F0205FF4A" -H "x-rpc-app_version: 2.33.1" -H "x-rpc-client_type: 5" -H "DS: ${time1},${random1},${md51}" -w %{http_code}
}
function startSign {
  startTime=$(date +"%F %T %A")
  echo "${startTime} 签到开始"
  #msg="${startTime}"
  cookieNum=1
  while read line; do
    echo "Cookie${cookieNum} 开始: "
    #msg="${msg}\n{${cookieNum}},"
    #echo ${line}
    getRoleUrl=''
    configRegion=${line%% *}
    #echo "A${configRegion}A"
    configName=${configRegion%%@*}
    echo " ${configName},"
    msg="${msg}\n${configName}:"
    configRegion=${configRegion#*@}
    #echo "A${configRegion}A"
    cookie=${line#* }
    cookie=${cookie:0:0-1} # 去除最后的换行
    #echo "A${cookie}A"
    if [ g"${configRegion}" = g"" ]; then
      echo "结束"
      exit 0
    else
      i1=0
      while [ $i1 -lt 2 ]; do
        if [ $i1 -eq 0 ]; then
          echo -n "原神，"
          msg="${msg}\n原神"
          if [ g"${configRegion}" = g"cn" ]; then
            getRoleUrl="https://api-takumi.mihoyo.com/binding/api/getUserGameRolesByCookie?game_biz=hk4e_cn"
            checkSignUrl="https://api-takumi.mihoyo.com/event/bbs_sign_reward/info"
            signUrl="https://api-takumi.mihoyo.com/event/bbs_sign_reward/sign"
            act_id="e202009291139501"
          fi
          if [ g"${configRegion}" = g"global" ]; then
            getRoleUrl="https://api-os-takumi.mihoyo.com/binding/api/getUserGameRolesByLtoken?game_biz=hk4e_global"
            checkSignUrl="https://hk4e-api-os.mihoyo.com/event/sol/info"
            signUrl="https://hk4e-api-os.mihoyo.com/event/sol/sign"
            act_id="e202102251931481"
          fi
        fi
        if [ $i1 -eq 1 ]; then
          echo -n "星穹铁道，"
          msg="${msg}\n星穹铁道"
          if [ g"${configRegion}" = g"cn" ]; then
            getRoleUrl="https://api-takumi.mihoyo.com/binding/api/getUserGameRolesByCookie?game_biz=hkrpg_cn"
            checkSignUrl="https://api-takumi.mihoyo.com/event/luna/info"
            signUrl="https://api-takumi.mihoyo.com/event/luna/sign"
            act_id="e202304121516551"
          fi
          if [ g"${configRegion}" = g"global" ]; then
          #  getRoleUrl="https://api-os-takumi.mihoyo.com/binding/api/getUserGameRolesByLtoken?game_biz=hkrpg_global"
            getRoleUrl="https://sg-public-api.hoyolab.com/binding/api/getUserGameRolesByLtoken?game_biz=hkrpg_global"
          #  checkSignUrl="https://api-os-takumi.mihoyo.com/event/luna/info"
            checkSignUrl="https://sg-public-api.hoyolab.com/event/luna/info"
          #  signUrl="https://api-os-takumi.mihoyo.com/event/luna/sign"
            signUrl="https://sg-public-api.hoyolab.com/event/luna/sign"
            act_id="e202303301540311"
          fi
        fi
        if [ g"${getRoleUrl}" = g"" ]; then
          echo "配置应该为： LABLE@cn COOKIE ，或 LABLE@global COOKIE"
          exit 1
        fi
        curl -s "${getRoleUrl}" -H "Cookie: ${cookie}" -w %{http_code} >${tmpfile}
        sleep 1
        jsonstr=$(cat ${tmpfile})
        #echo ${jsonstr}
        code=${jsonstr:0-3}
        if [ g"${code}" = g"200" ]; then
          #retcode=`cat ${tmpfile} | awk -F "retcode\":" '{print $2}' | awk -F "," '{print $1}'`
          retcode=$(getjsonvalue "${jsonstr}" "retcode")
          #echo ${retcode}
          if [ g"${retcode}" = g"0" ]; then
            # 获取到信息
            numOfAccount=1
            tmp='a'
            while [ g"${tmp}" != g"" ]; do
              tmp=$(cat ${tmpfile} | awk -F "game_uid" -v t="${numOfAccount}" '{print $t}')
              ##echo "${tmp}"
              ((numOfAccount++))
            done
            let numOfAccount=numOfAccount-3
            echo "有${numOfAccount}个角色"
            msg="${msg}，有${numOfAccount}个角色"
            tmp=$(cat ${tmpfile} | awk -F "[" '{print $2}' | awk -F "]" '{print $1}')
            tmp=${tmp#*${zkh}}
            for ((i = 0; i < numOfAccount; i++)); do
              ((j = i + 1))
              echo "  第${j}个角色"
              msg="${msg}\n  [${j}] "
              account=${tmp%%${ykh}*}
              tmp=${tmp#*${zkh}}
              #echo ${account}
              region=$(getjsonvalue "${account}" "region")
              region_name=$(getjsonvalue "${account}" "region_name")
              level=$(getjsonvalue "${account}" "level")
              nickname=$(getjsonvalue "${account}" "nickname")
              game_uid=$(getjsonvalue "${account}" "game_uid")
              echo -n "   ${region_name} ${level}级的 ${nickname}(${game_uid}): "
              msg="${msg}${region_name} ${level}级的 ${nickname}(${game_uid}): "
              #dailyNote
              #character
              result=$(signCheck)
              if [ g"${result}" = g"0" ]; then
                result=$(sign)
                #echo ${result}
                if [ g"${result}" = g"OK" ]; then
                  checkedDay=$(signCheck 1)
                  echo "签到成功，本月共签${checkedDay}天"
                  msg="${msg}签到成功，本月共签${checkedDay}天"
                else
                  echo ${result}
                  msg="${msg}${result}"
                fi
              else
                echo ${result}
                msg="${msg}${result}"
              fi
              sleep 1
            done
          else
            # 出错
            message=$(getjsonvalue "${jsonstr}" "message")
            echo ${message}
            msg="${msg}\n  ${message}"
          fi
          #exit 0
        else
          echo "网络问题"
          msg="${msg}\n  网络问题"
        fi
        ((i1++))
      done
    fi
    ((cookieNum++))
    sleep 1
  done <${cookieFile}
  endTime=$(date +"%F %T %A")
  echo "${endTime} 签到结束"
  msg="${endTime}${msg}"
  #echo ${msg}
  title="米游社签到"
  [ g"${WorkWeiBot}" != g"" ] && WorkWei "${title}\n${msg}" || echo "未设置企业微信通知"
  [ g"${DingDingBot}" != g"" ] && DingDing "${title}\n${msg}" || echo "未设置钉钉通知"
  [ g"${SCKEY}" != g"" ] && FTQQ "${title}" "${msg}" || echo "未设置Server酱通知"

}

function editConfig {
  # $1 lable
  echo $1
}
function newConfig {
  echo "添加Cookie"
  lable=""
  while [ g"${lable}" = g"" ]; do
    read -p "取个名字：" lable
  done

  type=""
  while [ g"${type}" = g"" ]; do
    echo ""
    echo "1, 国内版(cn)"
    echo "2, 国际版(global)"
    echo "输入数字："
    choice=$(getChar1)
    case ${choice} in
    1) type="cn" ;;
    2) type="global" ;;
    *) echo "重新输入" ;;
    esac
  done

  echo ""
  cookie=""
  while [ g"${cookie}" = g"" ]; do
    read -p "输入cookie：" cookie
  done
  echo "${lable}@${type} ${cookie}" >>${cookieFile}
}
function delConfig {
  tr=$1
  cont=$(sed -n ${tr}p ${cookieFile})
  echo "删除 ${cont%% *}? (y/n)"
  submit1=$(getChar1)
  case ${submit1} in
  y) sed -i ${tr}'d' ${cookieFile} ;;
  *) echo "取消删除" ;;
  esac
}
function renameConfig {
  tr=$1
  cont=$(sed -n ${tr}p ${cookieFile})
  echo "${cont%%@*} 改名? (y/n)"
  submit1=$(getChar1)
  case ${submit1} in
  y)
    newname=""
    while [ g"${newname}" = g"" ]; do
      read -p "输入新名称：" newname
    done
    newcont="${newname}@${cont#*@}"
    sed -i "${tr}s/${cont}/${newcont}/" ${cookieFile}
    ;;
  *) echo "取消修改" ;;
  esac
}
function setConfig {
  while [ 1 -eq 1 ]; do
    echo "----------------------------------------------"
    echo "当前 Cookies:"
    echo -e "   Type    \tLable "
    echo -e "   ======  \t======"
    cookies=""
    num=0
    while read line; do
      configRegion=${line%% *}
      #echo "A${configRegion}A"
      configName=${configRegion%%@*}
      #echo ${configName}
      configRegion=${configRegion#*@}
      if [ g"${configRegion}" = g"cn" -o g"${configRegion}" = g"global" ]; then
        cookies[${num}]=${configName}
        ((num++))
        tmpRegion="${configRegion}      "
        tmpRegion=${tmpRegion:0:8}
        echo -e "   ${tmpRegion}\t${configName}"
      fi
    done <${cookieFile}
    echo ""
    [ ${num} -gt 0 ] && echo "e) 编辑"
    [ ${num} -gt 0 ] && echo "d) 删除"
    [ ${num} -gt 0 ] && echo "r) 重命名"
    echo "n) 新建"
    echo "q) 退出"
    echo ""
    [ ${num} -gt 0 ] && echo -n "e/"
    [ ${num} -gt 0 ] && echo -n "d/r/"
    echo -n "n/q: "
    choice=$(getChar1)
    case ${choice} in
    e)
      echo "还没写，建议删了重新建"
      #      echo ""
      #      for ((i=0; i<num; i++))
      #      do
      #        echo "${i}, ${cookies[${i}]}"
      #      done
      #      echo "输入序号："
      #      read editNum
      #      is_num=`echo ${editNum} | sed 's/[0-9]//g'`
      #      if [ g"${is_num}" = g"" ]; then
      #        editConfig "${cookies[${editNum}]}"
      #      else
      #        echo " 输入有误，按任意键重新开始"
      #        getChar1
      #      fi
      ;;
    n)
      echo ""
      newConfig
      ;;
    d)
      echo ""
      if [ ${num} -gt 0 ]; then
        delNum=-1
        while [ ${delNum} -lt 0 ]; do
          for ((i = 0; i < num; i++)); do
            echo "${i}, ${cookies[${i}]}"
          done
          echo "输入想要删除的序号(或输入c取消删除)："
          read delNum1
          [ g"${delNum1}" = g"c" ] && break
          is_num=$(echo ${delNum1} | sed 's/[0-9]//g')
          if [ g"${is_num}" = g"" ]; then
            if [ ${delNum1} -lt ${num} ]; then
              delNum=${delNum1}
            else
              echo "输入不在范围"
            fi
          else
            echo "输入不在范围"
          fi
        done
        if [ ${delNum} -gt -1 ]; then
          ((delNum++))
          delConfig "${delNum}"
        fi
      else
        echo " 输入有误，重新开始"
        #getChar1
      fi
      ;;
    r)
      echo ""
      if [ ${num} -gt 0 ]; then
        renameNum=-1
        while [ ${renameNum} -lt 0 ]; do
          for ((i = 0; i < num; i++)); do
            echo "${i}, ${cookies[${i}]}"
          done
          echo "输入想要改名的序号(或输入c取消改名)："
          read renameNum1
          [ g"${renameNum1}" = g"c" ] && break
          is_num=$(echo ${renameNum1} | sed 's/[0-9]//g')
          if [ g"${is_num}" = g"" ]; then
            if [ ${renameNum1} -lt ${num} ]; then
              renameNum=${renameNum1}
            else
              echo "输入不在范围"
            fi
          else
            echo "输入不在范围"
          fi
        done
        if [ ${renameNum} -gt -1 ]; then
          ((renameNum++))
          renameConfig "${renameNum}"
        fi
      else
        echo " 输入有误，重新开始"
        #getChar1
      fi
      ;;
    q)
      echo ""
      break
      ;;
    *)
      echo " 输入有误，重新开始"
      #getChar1
      ;;
    esac
  done
}
function getChar1 {
  SAVEDTTY=$(stty -g)
  #  stty -echo
  stty cbreak
  dd if=/dev/tty bs=1 count=1 2>/dev/null
  #  stty -raw
  #  stty echo
  stty -cbreak
  stty $SAVEDTTY
}

echo "=============================================="
echo "    米游社原神星穹铁道每日签到，支持国内版与国际版"
echo "                                      --by逸笙"
echo "用法："
echo -e "  $0 config\t配置"
echo -e "  $0 sign  \t签到"
echo "=============================================="
echo ""

if [ ! -s "${cookieFile}" ]; then
  setConfig
fi
if [ g"$1" = g"config" ]; then
  setConfig
fi
if [ g"$1" = g"sign" ]; then
  startSign
fi
#if [ g"$1" = g"" ]; then
#  startSign
#fi
