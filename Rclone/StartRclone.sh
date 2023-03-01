
exist_systemctl=`type systemctl >/dev/null && echo $?`
if [ g"${exist_systemctl}" = g"0" ]; then
  echo "systemctl ok."
else
  echo "No systemctl, maybe other OS. exit!"
  exit 0
fi

exist_fuse=`lsmod | grep fuse`
if [ g"${exist_fuse}" = g"" ]; then
  echo "No FUSE, please check. exit!"
  exit 0
else
  echo "FUSE ok."
fi

rcloneFile="/usr/bin/rclone"
mountPath="/root"
configFile="/root/.config/rclone/rclone.conf"

had_lable=`cat "${configFile}" | grep '\['`
if [ g"${had_lable}" = g"" ]; then
  echo "No config or No lable in config. exit!"
  exit 0
else
  echo "config ok."
fi

[ -s "/etc/systemd/system/rclone@.service" ] && mv /etc/systemd/system/rclone@.service /etc/systemd/system/rclone@.service.bak
cat << EOF > /etc/systemd/system/rclone@.service
[Unit]
Description=rclone mount %I drive
After=network.target

[Service]
#Type=notify
Type=simple
#PrivateTmp=true
ExecStart=${rcloneFile} mount %i: "${mountPath}/%i" --allow-other --config "${configFile}"

[Install]
WantedBy=multi-user.target
EOF

c=`cat "${configFile}" | grep '\['`
#echo $c
for a in $c
do
    #echo $a
    b=${a:1:${#a}-2}
    echo $b
    [ ! -d "${mountPath}/${b}" ] && mkdir -p "${mountPath}/${b}"
    systemctl enable rclone@${b}
    systemctl start rclone@${b}
done

