#!/bin/bash

exist_systemctl=$(type systemctl >/dev/null && echo $?)
if [ g"${exist_systemctl}" = g"0" ]; then
  echo -e "systemctl \033[92;32mok\033[0m."
else
  echo "No systemctl, maybe other OS. exit!"
  exit 0
fi

exist_fuse=$(lsmod | grep fuse)
if [ g"${exist_fuse}" = g"" ]; then
  echo "No FUSE, please check. exit!"
  exit 0
else
  echo -e "FUSE \033[92;32mok\033[0m."
fi

rcloneFile="/usr/bin/rclone"
mountPath="/root"
configFile="/root/.config/rclone/rclone.conf"

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
After=network.target

[Service]
Type=simple
#Type=idle
#PrivateTmp=true
ExecStartPre=-/usr/bin/umount "${mountPath}/%i"
ExecStart=${rcloneFile} mount "%i:" "${mountPath}/%i" --allow-non-empty --allow-other --config "${configFile}"
ExecStop=-/usr/bin/umount -f "${mountPath}/%i"
#ExecReload=/bin/echo %i r

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

lables=$(cat "${configFile}" | grep '\[')
#echo ${lables}
for lable in ${lables}; do
  #echo ${lable}
  drive=${lable:1:${#lable}-2}
  echo -e "\033[92;93m---------${drive}--------\033[0m"
  [ ! -d "${mountPath}/${drive}" ] && mkdir -p "${mountPath}/${drive}"
  systemctl enable rclone@${drive}
  systemctl start rclone@${drive}
  systemctl status rclone@${drive} -l
done
