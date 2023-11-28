# 易微联WIFI开关命令行开关

官方定时只能设置8个，开4次关4次，完全不能满足需求。  
（PS：不想打包go-sqlite3，所以需要调用系统sqlite3命令，请在使用前测试sqlite3 -version能不能正常显示）  
（PS：需要易微联开发者，去官方申请，等一两天）  

1，单文件，编译后运行。  
2，第一次会引导绑定设备，在网页上进行。  
3，通过命令`ewectl turnon xxxxxxxxx`来打开对应设备，`ewectl turnoff xxxxxxxxxx`来关闭。  
4，在crontab中设置好开关时间。  
