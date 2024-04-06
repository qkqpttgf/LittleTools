# 易微联WIFI开关命令行开关

官方定时只能设置8个，开4次关4次，完全不能满足需求。  
（PS：不想打包go-sqlite3，所以需要调用系统sqlite3命令，请在使用前在命令行输入`sqlite3 -version`测试能不能正常显示）  
（PS：需要易微联开发者，去官方申请，等一两天）  

1，单文件，编译后运行。  
2，第一次会引导绑定设备，将在网页上进行。可以通过`-c|-config /var/abc.db`指定要使用的配置文件位置。  
3，绑定好后，通过命令`ewectl turnon 100xxxxxxx`或`ewectl turnon 数据库序列号`来打开对应设备，也可通过`ewectl turnon 1:0`打开数据库第1条设备的0号口。  
4，通过命令`ewectl turnoff xxxxxxxxxx`来关闭。  
5，可以在crontab中设置好命令来定时开与关。  
6，可以使用`ewectl web`开启一个简易的网页服务，在网页上操作。  
