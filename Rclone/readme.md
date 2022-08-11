# 自动启动挂载Rclone盘

## 需要
> 系统自带 systemctl 命令，如 CentOS 7 或 Debian 8 或 Ubuntu 16 以上。  
> 已安装好rclone，并已添加好盘。  
> 已经安装好 FUSE 。  

脚本会根据rclone的config中的盘名标签，在`/root`下创建各个盘名目录，然后挂载各盘，设置开机启动。
