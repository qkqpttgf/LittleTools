#!/bin/bash

# 挂载位置，可改
mountPath="/root"

# rclone程序位置与配置位置
rcloneFile="/usr/bin/rclone"
configFile="/root/.config/rclone/rclone.conf"

exist_systemctl=$(
  type systemctl >/dev/null
  echo $?
)
if [ g"${exist_systemctl}" != g"0" ]; then
  echo "No systemctl, maybe other OS. exit!"
  exit 0
else
  echo -e "systemctl \033[92;32mok\033[0m."
fi

exist_fuse=$(
  fusermount -V >/dev/null
  echo $?
)
if [ g"${exist_fuse}" != g"0" ]; then
  echo "No FUSE, please check. exit!"
  exit 0
else
  echo -e "FUSE \033[92;32mok\033[0m."
fi

had_lable=$(cat "${configFile}" | grep '\[')
if [ g"${had_lable}" = g"" ]; then
  echo "No config or No lable in config. exit!"
  exit 0
else
  echo -e "config \033[92;32mok\033[0m."
fi

[ -s "/etc/systemd/system/rclone@.service" ] && mv /etc/systemd/system/rclone@.service /etc/systemd/system/rclone@.service.bak
cat <<EOF >/etc/systemd/system/rclone@.service
[Unit]
Description=Rclone Mount Drive %I
Wants=network-online.target
After=network-online.target

[Service]
#Type=idle
Type=simple
ExecStartPre=-/usr/bin/mkdir -p "${mountPath}/%i"
ExecStartPre=-/usr/bin/umount "${mountPath}/%i"
ExecStart="${rcloneFile}" mount "%i:" "${mountPath}/%i" --allow-non-empty --allow-other --vfs-cache-mode writes --config "${configFile}"
ExecStop=-/usr/bin/umount -f "${mountPath}/%i"
#ExecStopPost=-/usr/bin/rm -rf "${mountPath}/%i"
Restart=on-failure
RestartSec=30
StartLimitInterval=0
StartLimitBurst=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

lables=$(cat "${configFile}" | grep '\[')
#echo ${lables}
for lable in ${lables}; do
  #echo ${lable}
  if [ ${#lable} -gt 2 ]; then
    drive=${lable:1:${#lable}-2}
    echo -e "\n\033[92;93m---------${drive}--------\033[0m"
    systemctl enable rclone@${drive}
    systemctl start rclone@${drive}
    #echo -e "If stoping at \"\033[92;93mlines xxx (END)\033[0m\", please \033[92;93mPRESS q\033[0m.\n"
    sleep 3
    systemctl status rclone@${drive} -l --no-pager
  fi
done
